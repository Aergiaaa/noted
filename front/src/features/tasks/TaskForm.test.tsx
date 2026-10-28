import { afterEach, describe, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import type { Task } from "../../lib/api";
import TaskForm from "./TaskForm";

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

// Picker sources land alongside the save call: route by URL so list GETs
// answer {data} and the save keeps its own status/body.
function fetchRouter(lists: Record<string, unknown>, saveStatus = 201) {
  return vi.fn().mockImplementation((url: string, init?: RequestInit) => {
    if (init?.method === "PATCH" || init?.method === "POST") {
      return Promise.resolve(jsonResponse(saveStatus, {}));
    }
    if (url.includes("/events?")) {
      return Promise.resolve(
        jsonResponse(200, {
          data: [
            {
              id: "e9",
              title: "Dentist",
              starts_at: "2026-10-15T09:00:00Z",
              ends_at: "2026-10-15T10:00:00Z",
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
    if (url.includes("/tasks")) {
      return Promise.resolve(jsonResponse(200, lists.tasks ?? { data: [] }));
    }
    return Promise.resolve(jsonResponse(200, { data: [] }));
  });
}

const TASK: Task = {
  id: "t1",
  title: "Buy keyboard",
  status: "open",
  due_date: "2026-10-20",
  event_id: "e9",
  note_id: "n9",
  completed_at: null,
};

function renderForm(task: Task | null) {
  const onClose = vi.fn();
  const onSaved = vi.fn();
  const onDeleted = vi.fn();
  const view = render(
    <TaskForm
      task={task}
      onClose={onClose}
      onSaved={onSaved}
      onDeleted={onDeleted}
    />,
  );
  return { ...view, onClose, onSaved, onDeleted };
}

describe("TaskForm", () => {
  it("creates a task with title and due; empty links stay omitted", async () => {
    const fetchMock = fetchRouter({});
    vi.stubGlobal("fetch", fetchMock);
    const { onClose, onSaved } = renderForm(null);

    await waitFor(() => expect(screen.getByLabelText("Event")).toBeDefined());

    fireEvent.change(screen.getByLabelText("Title"), {
      target: { value: "Ship it" },
    });
    fireEvent.change(screen.getByLabelText("Due"), {
      target: { value: "2026-10-20" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(onSaved).toHaveBeenCalled());
    expect(onClose).toHaveBeenCalled();
    const saveCall = fetchMock.mock.calls.find(
      (call) => (call[1] as RequestInit)?.method === "POST",
    );
    expect(saveCall?.[0]).toBe("/api/tasks");
    expect(saveCall?.[1]).toMatchObject({
      method: "POST",
      body: JSON.stringify({
        title: "Ship it",
        due_date: "2026-10-20",
      }),
    });
  });

  it("offers only existing titles in the event and note pickers", async () => {
    vi.stubGlobal("fetch", fetchRouter({}));
    renderForm(null);

    await waitFor(() => expect(screen.getByLabelText("Event")).toBeDefined());
    const eventOptions = screen.getAllByRole("option");
    // Both pickers present; wait for their lists too.
    await waitFor(() =>
      expect(screen.getByRole("option", { name: "Dentist" })).toBeDefined(),
    );
    expect(screen.getByRole("option", { name: "Groceries" })).toBeDefined();
    expect(screen.getAllByRole("option", { name: "None" })).toHaveLength(2);
    expect(eventOptions.length).toBeGreaterThan(0);
  });

  it("sends picked link ids on create", async () => {
    const fetchMock = fetchRouter({});
    vi.stubGlobal("fetch", fetchMock);
    renderForm(null);

    await waitFor(() =>
      expect(screen.getByRole("option", { name: "Dentist" })).toBeDefined(),
    );
    fireEvent.change(screen.getByLabelText("Title"), {
      target: { value: "Ship it" },
    });
    fireEvent.change(screen.getByLabelText("Event"), {
      target: { value: "e9" },
    });
    fireEvent.change(screen.getByLabelText("Note"), {
      target: { value: "n9" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => {
      const saveCall = fetchMock.mock.calls.find(
        (call) => (call[1] as RequestInit)?.method === "POST",
      );
      expect(saveCall).toBeDefined();
    });
    const saveCall = fetchMock.mock.calls.find(
      (call) => (call[1] as RequestInit)?.method === "POST",
    );
    const body = JSON.parse((saveCall![1] as { body: string }).body) as Record<
      string,
      unknown
    >;
    expect(body.event_id).toBe("e9");
    expect(body.note_id).toBe("n9");
  });

  it("strips control characters from the title before sending", async () => {
    const fetchMock = fetchRouter({});
    vi.stubGlobal("fetch", fetchMock);
    renderForm(null);

    fireEvent.change(screen.getByLabelText("Title"), {
      target: { value: "Den\x00t\tist " },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => {
      const saveCall = fetchMock.mock.calls.find(
        (call) => (call[1] as RequestInit)?.method === "POST",
      );
      expect(saveCall).toBeDefined();
    });
    const saveCall = fetchMock.mock.calls.find(
      (call) => (call[1] as RequestInit)?.method === "POST",
    );
    expect(saveCall?.[1]).toMatchObject({
      method: "POST",
      body: JSON.stringify({ title: "Dentist" }),
    });
  });

  it("rejects a control-only title before touching the network", () => {
    const fetchMock = fetchRouter({});
    vi.stubGlobal("fetch", fetchMock);
    renderForm(null);

    fireEvent.change(screen.getByLabelText("Title"), {
      target: { value: "\x00\x07\t\n" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(screen.getByText("Title is required.")).toBeDefined();
    const saveCalls = fetchMock.mock.calls.filter(
      (call) => (call[1] as RequestInit)?.method === "POST",
    );
    expect(saveCalls).toHaveLength(0);
  });

  it("PATCHes cleared links as empty strings and keeps untouched ones", async () => {
    const fetchMock = fetchRouter({}, 200);
    vi.stubGlobal("fetch", fetchMock);
    renderForm(TASK);

    await waitFor(() =>
      expect(screen.getByRole("option", { name: "Dentist" })).toBeDefined(),
    );
    expect((screen.getByLabelText("Event") as HTMLSelectElement).value).toBe(
      "e9",
    );

    // Clear the event, leave the note alone.
    fireEvent.change(screen.getByLabelText("Event"), {
      target: { value: "" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => {
      const patchCall = fetchMock.mock.calls.find(
        (call) => (call[1] as RequestInit)?.method === "PATCH",
      );
      expect(patchCall).toBeDefined();
    });
    const patchCall = fetchMock.mock.calls.find(
      (call) => (call[1] as RequestInit)?.method === "PATCH",
    );
    expect(patchCall?.[0]).toBe("/api/tasks/t1");
    const body = JSON.parse((patchCall![1] as { body: string }).body) as Record<
      string,
      unknown
    >;
    expect(body).toEqual({
      title: "Buy keyboard",
      due_date: "2026-10-20",
      event_id: "",
      note_id: "n9",
    });
  });

  it("rejects an empty title before touching the network", () => {
    const fetchMock = fetchRouter({});
    vi.stubGlobal("fetch", fetchMock);
    renderForm(null);

    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(screen.getByText("Title is required.")).toBeDefined();
    const saveCalls = fetchMock.mock.calls.filter(
      (call) => (call[1] as RequestInit)?.method === "POST",
    );
    expect(saveCalls).toHaveLength(0);
  });

  it("shows server validation fields after a failed save", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation((_url: string, init?: RequestInit) => {
        if (init?.method === "PATCH" || init?.method === "POST") {
          return Promise.resolve(
            jsonResponse(400, {
              error: {
                code: "validation_error",
                message: "validation failed",
                fields: { due_date: "must be YYYY-MM-DD" },
              },
            }),
          );
        }
        return Promise.resolve(jsonResponse(200, { data: [] }));
      }),
    );
    renderForm(null);

    fireEvent.change(screen.getByLabelText("Title"), {
      target: { value: "x" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(screen.getByText("due_date: must be YYYY-MM-DD")).toBeDefined(),
    );
  });

  it("deletes after an explicit confirmation", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);
    const { onClose, onDeleted } = renderForm(TASK);

    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    expect(screen.getByText("Delete this task?")).toBeDefined();

    fireEvent.click(screen.getByRole("button", { name: "Keep it" }));
    expect(screen.queryByText("Delete this task?")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    fireEvent.click(screen.getByRole("button", { name: "Yes, delete" }));

    await waitFor(() => expect(onDeleted).toHaveBeenCalledWith("t1"));
    expect(onClose).toHaveBeenCalled();
    const deleteCall = fetchMock.mock.calls.find(
      (call) => (call[1] as RequestInit)?.method === "DELETE",
    );
    expect(deleteCall?.[1]).toMatchObject({ method: "DELETE" });
  });

  it("shows an error and returns to the buttons when delete fails", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementation((_url: string, init?: RequestInit) => {
        if (init?.method === "DELETE") {
          return Promise.resolve(
            jsonResponse(500, {
              error: { code: "internal_error", message: "server exploded" },
            }),
          );
        }
        return Promise.resolve(jsonResponse(200, { data: [] }));
      }),
    );
    renderForm(TASK);

    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    fireEvent.click(screen.getByRole("button", { name: "Yes, delete" }));

    await waitFor(() =>
      expect(screen.getByText("server exploded")).toBeDefined(),
    );
    expect(screen.queryByText("Delete this task?")).toBeNull();
    expect(screen.getByRole("button", { name: "Delete" })).toBeDefined();
  });

  it("closes on cancel, backdrop click and Escape", () => {
    const { container, onClose } = renderForm(null);

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onClose).toHaveBeenCalledTimes(1);

    fireEvent.click(container.firstElementChild as HTMLElement);
    expect(onClose).toHaveBeenCalledTimes(2);

    fireEvent.keyDown(window, { key: "Tab" });
    expect(onClose).toHaveBeenCalledTimes(2);

    fireEvent.keyDown(window, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(3);
  });
});
