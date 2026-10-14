import { afterEach, describe, expect, it, vi } from "vitest";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import CalendarPage from "./CalendarPage";
import { formatMonthTitle } from "../lib/format";
import type { Event } from "../lib/api";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function listResponse(events: Event[]) {
  return jsonResponse(200, { data: events });
}

// Local-constructed values keep every assertion correct in any test TZ.
function dayKey(date = new Date()): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
}

function localISO(key: string, time: string): string {
  return new Date(`${key}T${time}`).toISOString();
}

function eventOn(key: string, title: string): Event {
  return {
    id: "e1",
    title,
    starts_at: localISO(key, "09:00"),
    ends_at: localISO(key, "10:00"),
    description: null,
    note_id: null,
  };
}

function fillEventForm(title: string, start: string, end: string) {
  fireEvent.change(screen.getByLabelText("Title"), {
    target: { value: title },
  });
  fireEvent.change(screen.getByLabelText("Starts"), {
    target: { value: start },
  });
  fireEvent.change(screen.getByLabelText("Ends"), { target: { value: end } });
}

describe("CalendarPage", () => {
  it("shows a skeleton while the month loads, then the grid", async () => {
    let resolveList!: (r: Response) => void;
    const fetchMock = vi.fn().mockImplementationOnce(
      () =>
        new Promise<Response>((res) => {
          resolveList = res;
        }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const { container } = render(<CalendarPage />);
    expect(container.querySelector(".skeleton")).not.toBeNull();

    await act(async () => {
      resolveList(listResponse([]));
    });

    await waitFor(() =>
      expect(screen.getAllByRole("gridcell")).toHaveLength(42),
    );
    expect(container.querySelector(".skeleton")).toBeNull();
  });

  it("offers an empty state whose CTA opens the create form", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(listResponse([])));

    render(<CalendarPage />);
    await waitFor(() =>
      expect(screen.getByText("No events this month.")).toBeDefined(),
    );

    fireEvent.click(screen.getByRole("button", { name: /Create your first/ }));

    expect(screen.getByRole("heading", { name: "New event" })).toBeDefined();
  });

  it("creates an event from an empty day cell", async () => {
    const key = dayKey();
    const created = eventOn(key, "Dentist");
    let resolvePost!: (r: Response) => void;
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(listResponse([]))
      .mockImplementationOnce(
        () =>
          new Promise<Response>((res) => {
            resolvePost = res;
          }),
      )
      .mockResolvedValueOnce(listResponse([created]));
    vi.stubGlobal("fetch", fetchMock);

    render(<CalendarPage />);
    await waitFor(() =>
      expect(screen.getByRole("gridcell", { name: key })).toBeDefined(),
    );

    fireEvent.click(screen.getByRole("gridcell", { name: key }));
    expect(screen.getByRole("heading", { name: "New event" })).toBeDefined();

    fillEventForm("Dentist", `${key}T09:00`, `${key}T10:00`);
    fireEvent.change(screen.getByLabelText("Description"), {
      target: { value: "checkup" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(screen.getByRole("button", { name: "Saving…" })).toBeDefined();
    await act(async () => {
      resolvePost(jsonResponse(201, created));
    });

    await waitFor(() =>
      expect(screen.getByRole("button", { name: /Dentist/ })).toBeDefined(),
    );
    expect(fetchMock).toHaveBeenCalledTimes(3);

    const [, postInit] = fetchMock.mock.calls[1];
    expect(postInit.method).toBe("POST");
    expect(JSON.parse(postInit.body)).toMatchObject({
      title: "Dentist",
      starts_at: created.starts_at,
      ends_at: created.ends_at,
      description: "checkup",
    });
  });

  it("requires a title before saving", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(listResponse([])));

    render(<CalendarPage />);
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "New event" })).toBeDefined(),
    );

    fireEvent.click(screen.getByRole("button", { name: "New event" }));
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(screen.getByText("Title is required.")).toBeDefined();
    expect(vi.mocked(fetch)).toHaveBeenCalledTimes(1);
  });

  it("rejects an end time that is not after the start client-side", async () => {
    const key = dayKey();
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(listResponse([])));

    render(<CalendarPage />);
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "New event" })).toBeDefined(),
    );

    fireEvent.click(screen.getByRole("button", { name: "New event" }));
    // Both values differ from the prefill so each onChange actually fires
    // (React drops change events whose value didn't move).
    fillEventForm("X", `${key}T11:00`, `${key}T08:00`);
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(screen.getByText("Ends must be after starts.")).toBeDefined();
    expect(vi.mocked(fetch)).toHaveBeenCalledTimes(1);
  });

  it("renders server-side field errors from a 400", async () => {
    const key = dayKey();
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(listResponse([]))
      .mockResolvedValueOnce(
        jsonResponse(400, {
          error: {
            code: "validation_error",
            message: "invalid",
            fields: { ends_at: "must be after starts_at" },
          },
        }),
      );
    vi.stubGlobal("fetch", fetchMock);

    render(<CalendarPage />);
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "New event" })).toBeDefined(),
    );

    fireEvent.click(screen.getByRole("button", { name: "New event" }));
    fillEventForm("X", `${key}T09:00`, `${key}T10:00`);
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Save" }));
    });

    expect(screen.getByText("ends_at: must be after starts_at")).toBeDefined();
    expect(screen.getByRole("heading", { name: "New event" })).toBeDefined();
  });

  it("keeps the form open when saving fails without an error envelope", async () => {
    const key = dayKey();
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(listResponse([]))
      .mockRejectedValueOnce(new TypeError("offline"));
    vi.stubGlobal("fetch", fetchMock);

    render(<CalendarPage />);
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "New event" })).toBeDefined(),
    );

    fireEvent.click(screen.getByRole("button", { name: "New event" }));
    fillEventForm("X", `${key}T09:00`, `${key}T10:00`);
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Save" }));
    });

    expect(screen.getByText("network request failed")).toBeDefined();
    expect(screen.getByRole("heading", { name: "New event" })).toBeDefined();
  });

  it("shows the generic save message when the success body is not JSON", async () => {
    const key = dayKey();
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(listResponse([]))
      .mockResolvedValueOnce(new Response("nope", { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    render(<CalendarPage />);
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "New event" })).toBeDefined(),
    );

    fireEvent.click(screen.getByRole("button", { name: "New event" }));
    fillEventForm("X", `${key}T09:00`, `${key}T10:00`);
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Save" }));
    });

    expect(screen.getByText("Couldn't save the event.")).toBeDefined();
  });

  it("edits an event from its chip", async () => {
    const key = dayKey();
    const original = eventOn(key, "Dentist");
    const renamed: Event = { ...original, title: "Dentist 2.0" };
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(listResponse([original]))
      .mockResolvedValueOnce(jsonResponse(200, renamed))
      .mockResolvedValueOnce(listResponse([renamed]));
    vi.stubGlobal("fetch", fetchMock);

    render(<CalendarPage />);
    await waitFor(() =>
      expect(screen.getByRole("button", { name: /Dentist/ })).toBeDefined(),
    );

    fireEvent.click(screen.getByRole("button", { name: /Dentist/ }));
    expect(screen.getByRole("heading", { name: "Edit event" })).toBeDefined();
    expect(screen.getByLabelText("Title")).toHaveProperty("value", "Dentist");

    fireEvent.change(screen.getByLabelText("Title"), {
      target: { value: "Dentist 2.0" },
    });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Save" }));
    });

    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: /Dentist 2\.0/ }),
      ).toBeDefined(),
    );

    const [, patchInit] = fetchMock.mock.calls[1];
    expect(patchInit.method).toBe("PATCH");
    expect(JSON.parse(patchInit.body)).toMatchObject({ title: "Dentist 2.0" });
  });

  it("deletes only after the inline confirmation", async () => {
    const key = dayKey();
    const original = eventOn(key, "Dentist");
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(listResponse([original]))
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(listResponse([]));
    vi.stubGlobal("fetch", fetchMock);

    render(<CalendarPage />);
    await waitFor(() =>
      expect(screen.getByRole("button", { name: /Dentist/ })).toBeDefined(),
    );

    fireEvent.click(screen.getByRole("button", { name: /Dentist/ }));
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    expect(screen.getByText("Delete this event?")).toBeDefined();
    expect(fetchMock).toHaveBeenCalledTimes(1);

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Yes, delete" }));
    });

    await waitFor(() =>
      expect(screen.queryByRole("button", { name: /Dentist/ })).toBeNull(),
    );
    const [, deleteInit] = fetchMock.mock.calls[1];
    expect(deleteInit.method).toBe("DELETE");
    expect(screen.queryByRole("heading", { name: "Edit event" })).toBeNull();
  });

  it("keeps the event when the confirmation is dismissed", async () => {
    const key = dayKey();
    const original = eventOn(key, "Dentist");
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValueOnce(listResponse([original])),
    );

    render(<CalendarPage />);
    await waitFor(() =>
      expect(screen.getByRole("button", { name: /Dentist/ })).toBeDefined(),
    );

    fireEvent.click(screen.getByRole("button", { name: /Dentist/ }));
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    fireEvent.click(screen.getByRole("button", { name: "Keep it" }));

    expect(screen.queryByText("Delete this event?")).toBeNull();
    expect(screen.getByRole("heading", { name: "Edit event" })).toBeDefined();
    expect(vi.mocked(fetch)).toHaveBeenCalledTimes(1);
  });

  it("keeps the event and drops the confirm when delete fails", async () => {
    const key = dayKey();
    const original = eventOn(key, "Dentist");
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValueOnce(listResponse([original]))
        .mockRejectedValueOnce(new TypeError("offline")),
    );

    render(<CalendarPage />);
    await waitFor(() =>
      expect(screen.getByRole("button", { name: /Dentist/ })).toBeDefined(),
    );

    fireEvent.click(screen.getByRole("button", { name: /Dentist/ }));
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Yes, delete" }));
    });

    expect(screen.getByText("network request failed")).toBeDefined();
    expect(screen.queryByText("Delete this event?")).toBeNull();
    expect(screen.getByRole("heading", { name: "Edit event" })).toBeDefined();
    expect(vi.mocked(fetch)).toHaveBeenCalledTimes(2);
  });

  it("ignores a load that settles after unmount", async () => {
    let resolveList!: (r: Response) => void;
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementationOnce(
        () =>
          new Promise<Response>((res) => {
            resolveList = res;
          }),
      ),
    );

    const { unmount } = render(<CalendarPage />);
    unmount();

    await act(async () => {
      resolveList(listResponse([]));
    });
    expect(vi.mocked(fetch)).toHaveBeenCalledTimes(1);
  });

  it("ignores a load that fails after unmount", async () => {
    let rejectList!: (err: unknown) => void;
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementationOnce(
        () =>
          new Promise<Response>((_, rej) => {
            rejectList = rej;
          }),
      ),
    );

    const { unmount } = render(<CalendarPage />);
    unmount();

    await act(async () => {
      rejectList(new TypeError("offline"));
    });
    expect(vi.mocked(fetch)).toHaveBeenCalledTimes(1);
  });

  it("shows the generic load message when the body is not JSON", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValueOnce(
          new Response("<html>nope</html>", { status: 200 }),
        ),
    );

    render(<CalendarPage />);
    await waitFor(() => expect(screen.getByRole("alert")).toBeDefined());
    expect(screen.getByText("Couldn't load events.")).toBeDefined();
  });

  it("recovers from a load failure via retry", async () => {
    const fetchMock = vi
      .fn()
      .mockRejectedValueOnce(new TypeError("offline"))
      .mockResolvedValueOnce(listResponse([]));
    vi.stubGlobal("fetch", fetchMock);

    render(<CalendarPage />);
    await waitFor(() => expect(screen.getByRole("alert")).toBeDefined());
    expect(screen.getByText("network request failed")).toBeDefined();

    fireEvent.click(screen.getByRole("button", { name: "Retry" }));

    await waitFor(() =>
      expect(screen.getAllByRole("gridcell")).toHaveLength(42),
    );
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("navigates to the next and previous month", async () => {
    // A fresh Response per call — a consumed body can't be read twice.
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation(() => Promise.resolve(listResponse([]))),
    );

    render(<CalendarPage />);
    await waitFor(() =>
      expect(screen.getByText(formatMonthTitle(new Date()))).toBeDefined(),
    );

    const next = new Date();
    next.setMonth(next.getMonth() + 1);
    fireEvent.click(screen.getByRole("button", { name: "Next month" }));
    await waitFor(() =>
      expect(screen.getByText(formatMonthTitle(next))).toBeDefined(),
    );

    fireEvent.click(screen.getByRole("button", { name: "Previous month" }));
    await waitFor(() =>
      expect(screen.getByText(formatMonthTitle(new Date()))).toBeDefined(),
    );
    expect(vi.mocked(fetch).mock.calls.length).toBeGreaterThanOrEqual(3);
  });

  it("closes the form with Escape without saving", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(listResponse([])));

    render(<CalendarPage />);
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "New event" })).toBeDefined(),
    );

    fireEvent.click(screen.getByRole("button", { name: "New event" }));
    fireEvent.keyDown(window, { key: "Tab" });
    expect(screen.getByRole("heading", { name: "New event" })).toBeDefined();
    fireEvent.keyDown(window, { key: "Escape" });

    expect(screen.queryByRole("heading", { name: "New event" })).toBeNull();
    expect(vi.mocked(fetch)).toHaveBeenCalledTimes(1);
  });
});
