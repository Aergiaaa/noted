import { afterEach, describe, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import type { Event } from "../../lib/api";
import EventForm from "./EventForm";

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

function fetchRouter(saveStatus = 201) {
  return vi.fn().mockImplementation((url: string, init?: RequestInit) => {
    if (init?.method === "PATCH" || init?.method === "POST") {
      return Promise.resolve(jsonResponse(saveStatus, {}));
    }
    if (url.includes("/tasks")) {
      return Promise.resolve(
        jsonResponse(200, {
          data: [
            {
              id: "t1",
              title: "Buy keyboard",
              status: "open",
              due_date: null,
              event_id: null,
              note_id: null,
              completed_at: null,
            },
          ],
        }),
      );
    }
    if (url.includes("/notes")) {
      return Promise.resolve(
        jsonResponse(200, {
          data: [{ id: "n9", title: "Groceries", body_md: "", pinned: false }],
        }),
      );
    }
    return Promise.resolve(jsonResponse(200, { data: [] }));
  });
}

const EVENT: Event = {
  id: "e1",
  title: "Dentist",
  starts_at: new Date(2026, 9, 15, 9, 0).toISOString(),
  ends_at: new Date(2026, 9, 15, 10, 0).toISOString(),
  description: null,
  note_id: "n9",
};

function renderForm(event: Event | null) {
  const onClose = vi.fn();
  const onSaved = vi.fn();
  const onDeleted = vi.fn();
  const view = render(
    <EventForm
      event={event}
      defaultStart={new Date(2026, 9, 15, 9, 0)}
      onClose={onClose}
      onSaved={onSaved}
      onDeleted={onDeleted}
    />,
  );
  return { ...view, onClose, onSaved, onDeleted };
}

function pickRange() {
  fireEvent.change(screen.getByLabelText("Starts hour"), {
    target: { value: "09" },
  });
  fireEvent.blur(screen.getByLabelText("Starts hour"));
  fireEvent.change(screen.getByLabelText("Starts minute"), {
    target: { value: "00" },
  });
  fireEvent.blur(screen.getByLabelText("Starts minute"));
  fireEvent.change(screen.getByLabelText("Ends hour"), {
    target: { value: "10" },
  });
  fireEvent.blur(screen.getByLabelText("Ends hour"));
  fireEvent.change(screen.getByLabelText("Ends minute"), {
    target: { value: "00" },
  });
  fireEvent.blur(screen.getByLabelText("Ends minute"));
}

describe("EventForm", () => {
  it("offers only existing notes in the note picker", async () => {
    vi.stubGlobal("fetch", fetchRouter());
    renderForm(null);

    await waitFor(() =>
      expect(screen.getByRole("option", { name: "Groceries" })).toBeDefined(),
    );
    expect(screen.getByRole("option", { name: "None" })).toBeDefined();
  });

  it("inserts a mention token into the description at the caret", async () => {
    vi.stubGlobal("fetch", fetchRouter());
    renderForm(null);

    const area = screen.getByLabelText("Description") as HTMLTextAreaElement;
    fireEvent.change(area, { target: { value: "Meet  now" } });
    area.setSelectionRange(5, 5);
    fireEvent.click(screen.getByRole("button", { name: "Mention" }));
    await screen.findByRole("option", { name: "Buy keyboard" });
    fireEvent.change(screen.getByLabelText("Insert mention"), {
      target: { value: "task:t1" },
    });

    expect(area.value).toBe("Meet @[task:t1] now");
    expect(area.selectionStart).toBe(5 + "@[task:t1]".length);
  });

  it("omits an empty note_id on create", async () => {
    const fetchMock = fetchRouter();
    vi.stubGlobal("fetch", fetchMock);
    renderForm(null);

    await waitFor(() =>
      expect(screen.getByRole("option", { name: "Groceries" })).toBeDefined(),
    );
    fireEvent.change(screen.getByLabelText("Title"), {
      target: { value: "X" },
    });
    pickRange();
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => {
      const post = fetchMock.mock.calls.find(
        (call) => (call[1] as RequestInit)?.method === "POST",
      );
      expect(post).toBeDefined();
    });
    const post = fetchMock.mock.calls.find(
      (call) => (call[1] as RequestInit)?.method === "POST",
    );
    const body = JSON.parse((post![1] as { body: string }).body) as Record<
      string,
      unknown
    >;
    expect("note_id" in body).toBe(false);
  });

  it("sends a picked note_id on create", async () => {
    const fetchMock = fetchRouter();
    vi.stubGlobal("fetch", fetchMock);
    renderForm(null);

    await waitFor(() =>
      expect(screen.getByRole("option", { name: "Groceries" })).toBeDefined(),
    );
    fireEvent.change(screen.getByLabelText("Title"), {
      target: { value: "X" },
    });
    fireEvent.change(screen.getByLabelText("Note"), {
      target: { value: "n9" },
    });
    pickRange();
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => {
      const post = fetchMock.mock.calls.find(
        (call) => (call[1] as RequestInit)?.method === "POST",
      );
      expect(post).toBeDefined();
    });
    const post = fetchMock.mock.calls.find(
      (call) => (call[1] as RequestInit)?.method === "POST",
    );
    const body = JSON.parse((post![1] as { body: string }).body) as Record<
      string,
      unknown
    >;
    expect(body.note_id).toBe("n9");
  });

  it("PATCHes a cleared note as an empty string", async () => {
    const fetchMock = fetchRouter(200);
    vi.stubGlobal("fetch", fetchMock);
    renderForm(EVENT);

    await waitFor(() =>
      expect(screen.getByRole("option", { name: "Groceries" })).toBeDefined(),
    );
    expect((screen.getByLabelText("Note") as HTMLSelectElement).value).toBe(
      "n9",
    );

    fireEvent.change(screen.getByLabelText("Note"), {
      target: { value: "" },
    });
    fireEvent.change(screen.getByLabelText("Title"), {
      target: { value: "Dentist 2" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => {
      const patch = fetchMock.mock.calls.find(
        (call) => (call[1] as RequestInit)?.method === "PATCH",
      );
      expect(patch).toBeDefined();
    });
    const patch = fetchMock.mock.calls.find(
      (call) => (call[1] as RequestInit)?.method === "PATCH",
    );
    expect(patch?.[0]).toBe("/api/events/e1");
    const body = JSON.parse((patch![1] as { body: string }).body) as Record<
      string,
      unknown
    >;
    expect(body.note_id).toBe("");
  });

  it("keeps a stored note_id untouched when the picker is not changed", async () => {
    const fetchMock = fetchRouter(200);
    vi.stubGlobal("fetch", fetchMock);
    renderForm(EVENT);

    await waitFor(() =>
      expect(screen.getByRole("option", { name: "Groceries" })).toBeDefined(),
    );
    fireEvent.change(screen.getByLabelText("Title"), {
      target: { value: "Dentist 2" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => {
      const patch = fetchMock.mock.calls.find(
        (call) => (call[1] as RequestInit)?.method === "PATCH",
      );
      expect(patch).toBeDefined();
    });
    const patch = fetchMock.mock.calls.find(
      (call) => (call[1] as RequestInit)?.method === "PATCH",
    );
    const body = JSON.parse((patch![1] as { body: string }).body) as Record<
      string,
      unknown
    >;
    expect(body.note_id).toBe("n9");
  });
});
