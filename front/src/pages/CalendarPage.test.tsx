import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import CalendarPage from "./CalendarPage";
import {
  formatDate,
  formatDayHeader,
  formatMonthTitle,
  fromDayKey,
} from "../lib/format";
import type { Event } from "../lib/api";

// Fixed clock: Wed 2026-10-15 12:00 local — the visible week is always
// Mon 2026-10-12 .. Sun 2026-10-18. shouldAdvanceTime keeps RTL's waitFor
// (and cally's async render) working under fake timers.
beforeEach(() => {
  vi.useFakeTimers({
    shouldAdvanceTime: true,
    now: new Date(2026, 9, 15, 12, 0, 0),
  });
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
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

const TODAY_KEY = "2026-10-15";
const WEEK_FROM = new Date(2026, 9, 12).toISOString();
const WEEK_TO = new Date(2026, 9, 18, 23, 59, 59, 999).toISOString();

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

// Drive one start/end field the way a user would: open the date panel,
// click the day in cally's shadow DOM, close it, then set hour/minute
// (committed on blur — the comboboxes only commit valid values).
async function pickDateTime(label: string, value: string) {
  const trigger = screen.getByLabelText(label);
  fireEvent.click(trigger);
  // The panel is portaled to body (DatePicker), outside the card, and
  // distinct from the MiniMonth calendar on the page.
  const host = document.querySelector(
    '[data-testid="picker-panel"] calendar-date',
  ) as HTMLElement;
  await waitFor(() =>
    expect(
      host.shadowRoot?.querySelectorAll("button").length ?? 0,
    ).toBeGreaterThan(0),
  );
  const month = host.querySelector("calendar-month") as HTMLElement;
  const day = Array.from(
    month.shadowRoot?.querySelectorAll("button") ?? [],
  ).find((b) => b.textContent === String(Number(value.slice(8, 10))));
  fireEvent.click(day as HTMLElement);
  fireEvent.click(trigger);

  const hour = screen.getByLabelText(`${label} hour`);
  fireEvent.change(hour, { target: { value: value.slice(11, 13) } });
  fireEvent.blur(hour);
  const minute = screen.getByLabelText(`${label} minute`);
  fireEvent.change(minute, { target: { value: value.slice(14, 16) } });
  fireEvent.blur(minute);
}

async function fillEventForm(title: string, start: string, end: string) {
  fireEvent.change(screen.getByLabelText("Title"), {
    target: { value: title },
  });
  await pickDateTime("Starts", start);
  await pickDateTime("Ends", end);
}

function eventBlock(name: string | RegExp) {
  return screen.getByRole("button", { name });
}

async function waitForGrid() {
  await waitFor(() =>
    expect(
      document.querySelectorAll("button[aria-label^='Create event on']"),
    ).toHaveLength(7 * 24),
  );
}

describe("CalendarPage", () => {
  it("shows a skeleton while the week loads, then the grid", async () => {
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

    await waitForGrid();
    expect(container.querySelector(".skeleton")).toBeNull();
    expect(container.querySelector(".calendar-scroll")).not.toBeNull();
    expect(fetchMock.mock.calls[0]?.[0]).toContain(
      encodeURIComponent(WEEK_FROM),
    );
    expect(fetchMock.mock.calls[0]?.[0]).toContain(encodeURIComponent(WEEK_TO));
  });

  it("offers an empty state whose CTA opens the create form", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(listResponse([])));

    render(<CalendarPage />);
    await waitFor(() =>
      expect(screen.getByText("No events this week.")).toBeDefined(),
    );

    // Under the mini calendar so an empty week needs no scrolling.
    expect(
      screen.getByText("No events this week.").closest("aside"),
    ).not.toBeNull();

    fireEvent.click(screen.getByRole("button", { name: /Create your first/ }));

    expect(screen.getByRole("heading", { name: "New event" })).toBeDefined();
  });

  it("creates an event from an empty hour slot", async () => {
    const created = eventOn(TODAY_KEY, "Dentist");
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
    await waitForGrid();

    fireEvent.click(
      screen.getByRole("button", {
        name: `Create event on ${formatDayHeader(new Date(2026, 9, 15))} at 14:00`,
      }),
    );
    expect(screen.getByRole("heading", { name: "New event" })).toBeDefined();
    expect(screen.getByLabelText("Starts").textContent).toBe(
      formatDate(fromDayKey(TODAY_KEY)),
    );

    await fillEventForm("Dentist", `${TODAY_KEY}T14:00`, `${TODAY_KEY}T15:00`);
    fireEvent.change(screen.getByLabelText("Description"), {
      target: { value: "checkup" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(screen.getByRole("button", { name: "Saving…" })).toBeDefined();
    await act(async () => {
      resolvePost(jsonResponse(201, created));
    });

    await waitFor(() => expect(eventBlock(/Dentist/)).toBeDefined());
    expect(fetchMock).toHaveBeenCalledTimes(3);

    const [, postInit] = fetchMock.mock.calls[1];
    expect(postInit.method).toBe("POST");
    expect(JSON.parse(postInit.body)).toMatchObject({
      title: "Dentist",
      starts_at: localISO(TODAY_KEY, "14:00"),
      ends_at: localISO(TODAY_KEY, "15:00"),
      description: "checkup",
    });
  });

  it("requires a title before saving", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(listResponse([])));

    render(<CalendarPage />);
    await waitForGrid();

    fireEvent.click(screen.getByRole("button", { name: "New event" }));
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(screen.getByText("Title is required.")).toBeDefined();
    expect(vi.mocked(fetch)).toHaveBeenCalledTimes(1);
  });

  it("strips control characters from title and description before saving", async () => {
    const created = eventOn(TODAY_KEY, "Dentist");
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(listResponse([]))
      .mockResolvedValueOnce(jsonResponse(201, created))
      .mockResolvedValueOnce(listResponse([created]));
    vi.stubGlobal("fetch", fetchMock);

    render(<CalendarPage />);
    await waitForGrid();

    const nul = String.fromCharCode(0);
    const bel = String.fromCharCode(7);
    fireEvent.click(screen.getByRole("button", { name: "New event" }));
    fireEvent.change(screen.getByLabelText("Title"), {
      target: { value: `Den${nul}t ist ` },
    });
    fireEvent.change(screen.getByLabelText("Description"), {
      target: { value: `check${bel}up\nnow` },
    });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Save" }));
    });

    expect(fetchMock).toHaveBeenCalledTimes(3);
    const [, postInit] = fetchMock.mock.calls[1];
    expect(postInit.method).toBe("POST");
    expect(JSON.parse(postInit.body)).toMatchObject({
      title: "Dent ist",
      description: "checkup\nnow",
    });
  });

  it("rejects an end time that is not after the start client-side", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(listResponse([])));

    render(<CalendarPage />);
    await waitForGrid();

    fireEvent.click(screen.getByRole("button", { name: "New event" }));
    // Both values differ from the prefill so each onChange actually fires
    // (React drops change events whose value didn't move).
    await fillEventForm("X", `${TODAY_KEY}T11:00`, `${TODAY_KEY}T08:00`);
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(screen.getByText("Ends must be after starts.")).toBeDefined();
    expect(vi.mocked(fetch)).toHaveBeenCalledTimes(1);
  });

  it("renders server-side field errors from a 400", async () => {
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
    await waitForGrid();

    fireEvent.click(screen.getByRole("button", { name: "New event" }));
    await fillEventForm("X", `${TODAY_KEY}T09:00`, `${TODAY_KEY}T10:00`);
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Save" }));
    });

    expect(screen.getByText("ends_at: must be after starts_at")).toBeDefined();
    expect(screen.getByRole("heading", { name: "New event" })).toBeDefined();
  });

  it("keeps the form open when saving fails without an error envelope", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(listResponse([]))
      .mockRejectedValueOnce(new TypeError("offline"));
    vi.stubGlobal("fetch", fetchMock);

    render(<CalendarPage />);
    await waitForGrid();

    fireEvent.click(screen.getByRole("button", { name: "New event" }));
    await fillEventForm("X", `${TODAY_KEY}T09:00`, `${TODAY_KEY}T10:00`);
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Save" }));
    });

    expect(screen.getByText("network request failed")).toBeDefined();
    expect(screen.getByRole("heading", { name: "New event" })).toBeDefined();
  });

  it("shows the generic save message when the success body is not JSON", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(listResponse([]))
      .mockResolvedValueOnce(new Response("nope", { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    render(<CalendarPage />);
    await waitForGrid();

    fireEvent.click(screen.getByRole("button", { name: "New event" }));
    await fillEventForm("X", `${TODAY_KEY}T09:00`, `${TODAY_KEY}T10:00`);
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Save" }));
    });

    expect(screen.getByText("Couldn't save the event.")).toBeDefined();
  });

  it("edits an event from its block", async () => {
    const original = eventOn(TODAY_KEY, "Dentist");
    const renamed: Event = { ...original, title: "Dentist 2.0" };
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(listResponse([original]))
      .mockResolvedValueOnce(jsonResponse(200, renamed))
      .mockResolvedValueOnce(listResponse([renamed]));
    vi.stubGlobal("fetch", fetchMock);

    render(<CalendarPage />);
    await waitFor(() => expect(eventBlock("Dentist 09:00")).toBeDefined());

    fireEvent.click(eventBlock("Dentist 09:00"));
    fireEvent.click(screen.getByRole("button", { name: "Edit" }));
    expect(screen.getByRole("heading", { name: "Edit event" })).toBeDefined();
    expect(screen.getByLabelText("Title")).toHaveProperty("value", "Dentist");

    fireEvent.change(screen.getByLabelText("Title"), {
      target: { value: "Dentist 2.0" },
    });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Save" }));
    });

    await waitFor(() => expect(eventBlock("Dentist 2.0 09:00")).toBeDefined());

    const [, patchInit] = fetchMock.mock.calls[1];
    expect(patchInit.method).toBe("PATCH");
    expect(JSON.parse(patchInit.body)).toMatchObject({ title: "Dentist 2.0" });
  });

  it("shows event details from the fetched week before editing", async () => {
    const detailed: Event = {
      ...eventOn(TODAY_KEY, "Dentist"),
      description: "Bring insurance card.",
    };
    const fetchMock = vi.fn().mockResolvedValueOnce(listResponse([detailed]));
    vi.stubGlobal("fetch", fetchMock);

    render(<CalendarPage />);
    await waitFor(() => expect(eventBlock("Dentist 09:00")).toBeDefined());

    fireEvent.click(eventBlock("Dentist 09:00"));
    expect(screen.getByRole("heading", { name: "Dentist" })).toBeDefined();
    expect(screen.getByText("Bring insurance card.")).toBeDefined();
    expect(fetchMock).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole("button", { name: "Edit" }));
    expect(screen.getByRole("heading", { name: "Edit event" })).toBeDefined();
    expect(screen.getByLabelText("Title")).toHaveProperty("value", "Dentist");
    expect(screen.queryByRole("heading", { name: "Dentist" })).toBeNull();
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("closes the details card without opening the form", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(listResponse([eventOn(TODAY_KEY, "Dentist")]));
    vi.stubGlobal("fetch", fetchMock);

    render(<CalendarPage />);
    await waitFor(() => expect(eventBlock("Dentist 09:00")).toBeDefined());

    fireEvent.click(eventBlock("Dentist 09:00"));
    fireEvent.click(screen.getByRole("button", { name: "Close" }));

    expect(screen.queryByRole("heading", { name: "Dentist" })).toBeNull();
    expect(screen.queryByRole("heading", { name: "Edit event" })).toBeNull();
    expect(eventBlock("Dentist 09:00")).toBeDefined();
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("deletes only after the inline confirmation", async () => {
    const original = eventOn(TODAY_KEY, "Dentist");
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(listResponse([original]))
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(listResponse([]));
    vi.stubGlobal("fetch", fetchMock);

    render(<CalendarPage />);
    await waitFor(() => expect(eventBlock("Dentist 09:00")).toBeDefined());

    fireEvent.click(eventBlock("Dentist 09:00"));
    fireEvent.click(screen.getByRole("button", { name: "Edit" }));
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    expect(screen.getByText("Delete this event?")).toBeDefined();
    expect(fetchMock).toHaveBeenCalledTimes(1);

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Yes, delete" }));
    });

    await waitFor(() =>
      expect(
        screen.queryByRole("button", { name: "Dentist 09:00" }),
      ).toBeNull(),
    );
    const [, deleteInit] = fetchMock.mock.calls[1];
    expect(deleteInit.method).toBe("DELETE");
    expect(screen.queryByRole("heading", { name: "Edit event" })).toBeNull();
  });

  it("deleting shows an Undo toast that restores the event", async () => {
    const original = eventOn(TODAY_KEY, "Dentist");
    let listCalls = 0;
    const fetchMock = vi
      .fn()
      .mockImplementation((_path: string, init?: RequestInit) => {
        if (init?.method === "DELETE") {
          return Promise.resolve(new Response(null, { status: 204 }));
        }
        if (init?.method === "POST") {
          return Promise.resolve(jsonResponse(200, original));
        }
        listCalls += 1;
        return Promise.resolve(listResponse(listCalls === 1 ? [original] : []));
      });
    vi.stubGlobal("fetch", fetchMock);

    render(<CalendarPage />);
    await waitFor(() => expect(eventBlock("Dentist 09:00")).toBeDefined());

    fireEvent.click(eventBlock("Dentist 09:00"));
    fireEvent.click(screen.getByRole("button", { name: "Edit" }));
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Yes, delete" }));
    });

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Undo" })).toBeDefined(),
    );

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Undo" }));
    });

    await waitFor(() =>
      expect(screen.queryByRole("button", { name: "Undo" })).toBeNull(),
    );
    const restoreCall = fetchMock.mock.calls.find(
      ([, init]) => (init as RequestInit | undefined)?.method === "POST",
    );
    expect(restoreCall?.[0]).toContain("/restore");
  });

  it("keeps the event and shows an error toast when undo fails", async () => {
    const original = eventOn(TODAY_KEY, "Dentist");
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(listResponse([original]))
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(listResponse([]))
      .mockRejectedValueOnce(new TypeError("offline"));
    vi.stubGlobal("fetch", fetchMock);

    render(<CalendarPage />);
    await waitFor(() => expect(eventBlock("Dentist 09:00")).toBeDefined());

    fireEvent.click(eventBlock("Dentist 09:00"));
    fireEvent.click(screen.getByRole("button", { name: "Edit" }));
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Yes, delete" }));
    });

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Undo" })).toBeDefined(),
    );
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Undo" }));
    });

    await waitFor(() =>
      expect(screen.getByText("network request failed")).toBeDefined(),
    );
    expect(screen.queryByRole("button", { name: "Undo" })).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Dismiss" }));
    expect(screen.queryByText("network request failed")).toBeNull();
  });

  it("opens the trash panel and restores an event from it", async () => {
    const trashed: Event = { ...eventOn(TODAY_KEY, "Old dentist"), id: "e9" };
    let trashCalls = 0;
    const fetchMock = vi
      .fn()
      .mockImplementation((path: string, init?: RequestInit) => {
        if (init?.method === "POST") {
          return Promise.resolve(jsonResponse(200, trashed));
        }
        if (typeof path === "string" && path.includes("trash=1")) {
          trashCalls += 1;
          return Promise.resolve(
            listResponse(trashCalls === 1 ? [trashed] : []),
          );
        }
        return Promise.resolve(listResponse([]));
      });
    vi.stubGlobal("fetch", fetchMock);

    render(<CalendarPage />);
    await waitForGrid();

    fireEvent.click(screen.getByRole("button", { name: "Open trash" }));
    await waitFor(() => expect(screen.getByText("Old dentist")).toBeDefined());

    fireEvent.click(screen.getByRole("button", { name: "Restore" }));
    await waitFor(() =>
      expect(screen.getByText("Nothing in the trash.")).toBeDefined(),
    );
    const restoreCall = fetchMock.mock.calls.find(
      ([, init]) => (init as RequestInit | undefined)?.method === "POST",
    );
    expect(restoreCall?.[0]).toBe("/api/events/e9/restore");
  });

  it("shows a load error in the trash panel with retry", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(listResponse([]))
      .mockResolvedValueOnce(
        jsonResponse(500, {
          error: { code: "internal_error", message: "boom" },
        }),
      )
      .mockResolvedValueOnce(listResponse([]));
    vi.stubGlobal("fetch", fetchMock);

    render(<CalendarPage />);
    await waitForGrid();

    fireEvent.click(screen.getByRole("button", { name: "Open trash" }));
    await waitFor(() => expect(screen.getByText("boom")).toBeDefined());

    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() =>
      expect(screen.getByText("Nothing in the trash.")).toBeDefined(),
    );
  });

  it("shows the generic load message when the trash body is not JSON", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(listResponse([]))
      .mockResolvedValueOnce(
        new Response("<html>nope</html>", { status: 200 }),
      );
    vi.stubGlobal("fetch", fetchMock);

    render(<CalendarPage />);
    await waitForGrid();

    fireEvent.click(screen.getByRole("button", { name: "Open trash" }));
    await waitFor(() =>
      expect(screen.getByText("Couldn't load events.")).toBeDefined(),
    );
  });

  it("cancels the trash load when the panel closes", async () => {
    const pending: Array<{
      resolve: (r: Response) => void;
      reject: (e: unknown) => void;
    }> = [];
    let calls = 0;
    const fetchMock = vi.fn().mockImplementation(() => {
      calls += 1;
      if (calls === 1) return Promise.resolve(listResponse([]));
      return new Promise<Response>((res, rej) => {
        pending.push({ resolve: res, reject: rej });
      });
    });
    vi.stubGlobal("fetch", fetchMock);

    render(<CalendarPage />);
    await waitForGrid();

    fireEvent.click(screen.getByRole("button", { name: "Open trash" }));
    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(screen.queryByRole("heading", { name: "Trash" })).toBeNull();
    await act(async () => {
      pending[0].resolve(listResponse([]));
    });

    fireEvent.click(screen.getByRole("button", { name: "Open trash" }));
    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(screen.queryByRole("heading", { name: "Trash" })).toBeNull();
    await act(async () => {
      pending[1].reject(new TypeError("offline"));
    });
    expect(vi.mocked(fetch)).toHaveBeenCalledTimes(3);
  });

  it("shows the restore failure when the undo response is not JSON", async () => {
    const original = eventOn(TODAY_KEY, "Dentist");
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(listResponse([original]))
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(listResponse([]))
      .mockResolvedValueOnce(new Response("nope", { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);

    render(<CalendarPage />);
    await waitFor(() => expect(eventBlock("Dentist 09:00")).toBeDefined());

    fireEvent.click(eventBlock("Dentist 09:00"));
    fireEvent.click(screen.getByRole("button", { name: "Edit" }));
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Yes, delete" }));
    });
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Undo" })).toBeDefined(),
    );
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Undo" }));
    });

    await waitFor(() =>
      expect(screen.getByText("Couldn't restore the event.")).toBeDefined(),
    );
    expect(screen.queryByRole("button", { name: "Undo" })).toBeNull();
  });

  it("keeps the event when the confirmation is dismissed", async () => {
    const original = eventOn(TODAY_KEY, "Dentist");
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValueOnce(listResponse([original])),
    );

    render(<CalendarPage />);
    await waitFor(() => expect(eventBlock("Dentist 09:00")).toBeDefined());

    fireEvent.click(eventBlock("Dentist 09:00"));
    fireEvent.click(screen.getByRole("button", { name: "Edit" }));
    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    fireEvent.click(screen.getByRole("button", { name: "Keep it" }));

    expect(screen.queryByText("Delete this event?")).toBeNull();
    expect(screen.getByRole("heading", { name: "Edit event" })).toBeDefined();
    expect(vi.mocked(fetch)).toHaveBeenCalledTimes(1);
  });

  it("keeps the event and drops the confirm when delete fails", async () => {
    const original = eventOn(TODAY_KEY, "Dentist");
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValueOnce(listResponse([original]))
        .mockRejectedValueOnce(new TypeError("offline")),
    );

    render(<CalendarPage />);
    await waitFor(() => expect(eventBlock("Dentist 09:00")).toBeDefined());

    fireEvent.click(eventBlock("Dentist 09:00"));
    fireEvent.click(screen.getByRole("button", { name: "Edit" }));
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

    await waitForGrid();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("steps whole weeks and flips the month title at the boundary", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation(() => Promise.resolve(listResponse([]))),
    );

    render(<CalendarPage />);
    await waitFor(() => expect(screen.getByText("October 2026")).toBeDefined());
    expect(vi.mocked(fetch).mock.calls[0]?.[0]).toContain(
      encodeURIComponent(WEEK_FROM),
    );

    fireEvent.click(screen.getByRole("button", { name: "Next week" }));
    await waitFor(() =>
      expect(vi.mocked(fetch).mock.calls[1]?.[0]).toContain(
        encodeURIComponent(new Date(2026, 9, 19).toISOString()),
      ),
    );
    expect(document.querySelector(".week-slide-fwd")).not.toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Previous week" }));
    await waitFor(() =>
      expect(vi.mocked(fetch).mock.calls[2]?.[0]).toContain(
        encodeURIComponent(WEEK_FROM),
      ),
    );
    expect(document.querySelector(".week-slide-back")).not.toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Previous week" }));
    fireEvent.click(screen.getByRole("button", { name: "Previous week" }));
    fireEvent.click(screen.getByRole("button", { name: "Previous week" }));
    await waitFor(() =>
      expect(vi.mocked(fetch).mock.calls[5]?.[0]).toContain(
        encodeURIComponent(new Date(2026, 8, 21).toISOString()),
      ),
    );
    await waitFor(() =>
      expect(screen.getByText("September 2026")).toBeDefined(),
    );
  });

  it("selects a day from the mini calendar and refetches that week", async () => {
    const fetchMock = vi
      .fn()
      .mockImplementation(() => Promise.resolve(listResponse([])));
    vi.stubGlobal("fetch", fetchMock);

    const { container } = render(<CalendarPage />);
    await waitForGrid();

    const host = container.querySelector("calendar-date") as HTMLElement;
    await waitFor(() =>
      expect(
        host
          .querySelector("calendar-month")
          ?.shadowRoot?.querySelectorAll("button").length ?? 0,
      ).toBeGreaterThan(0),
    );

    const month = host.querySelector("calendar-month") as HTMLElement;
    const day20 = Array.from(
      month.shadowRoot?.querySelectorAll("button") ?? [],
    ).find((b) => b.textContent === "20");
    fireEvent.click(day20 as HTMLElement);

    await waitFor(() =>
      expect(fetchMock.mock.calls[1]?.[0]).toContain(
        encodeURIComponent(new Date(2026, 9, 19).toISOString()),
      ),
    );
    expect(screen.getByText("October 2026")).toBeDefined();

    const day5 = Array.from(
      month.shadowRoot?.querySelectorAll("button") ?? [],
    ).find((b) => b.textContent === "5");
    fireEvent.click(day5 as HTMLElement);

    await waitFor(() =>
      expect(fetchMock.mock.calls[2]?.[0]).toContain(
        encodeURIComponent(new Date(2026, 9, 5).toISOString()),
      ),
    );
    expect(document.querySelector(".week-slide-back")).not.toBeNull();
    expect(screen.getByText("October 2026")).toBeDefined();
  });

  it("shows a single now-line in the current week", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(listResponse([])));

    const { container } = render(<CalendarPage />);
    await waitForGrid();

    expect(container.querySelectorAll("[data-testid='now-line']")).toHaveLength(
      1,
    );
  });

  it("closes the form with Escape without saving", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(listResponse([])));

    render(<CalendarPage />);
    await waitForGrid();

    fireEvent.click(screen.getByRole("button", { name: "New event" }));
    fireEvent.keyDown(window, { key: "Tab" });
    expect(screen.getByRole("heading", { name: "New event" })).toBeDefined();
    fireEvent.keyDown(window, { key: "Escape" });

    expect(screen.queryByRole("heading", { name: "New event" })).toBeNull();
    expect(vi.mocked(fetch)).toHaveBeenCalledTimes(1);
  });

  it("renders the fixed week's day headers", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(listResponse([])));

    render(<CalendarPage />);
    await waitForGrid();

    expect(
      screen.getByText(formatDayHeader(new Date(2026, 9, 12))),
    ).toBeDefined();
    expect(
      screen.getByText(formatMonthTitle(new Date(2026, 9, 15))),
    ).toBeDefined();
  });
});
