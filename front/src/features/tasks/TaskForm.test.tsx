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
  it("creates a task, trimming the due and link fields", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(201, {}));
    vi.stubGlobal("fetch", fetchMock);
    const { onClose, onSaved } = renderForm(null);

    fireEvent.change(screen.getByLabelText("Title"), {
      target: { value: "Ship it" },
    });
    fireEvent.change(screen.getByLabelText("Due"), {
      target: { value: "2026-10-20" },
    });
    fireEvent.change(screen.getByLabelText("Event ID"), {
      target: { value: "  e9  " },
    });
    fireEvent.change(screen.getByLabelText("Note ID"), {
      target: { value: "  n9  " },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(onSaved).toHaveBeenCalled());
    expect(onClose).toHaveBeenCalled();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/tasks");
    expect(fetchMock.mock.calls[0]?.[1]).toMatchObject({
      method: "POST",
      body: JSON.stringify({
        title: "Ship it",
        due_date: "2026-10-20",
        event_id: "e9",
        note_id: "n9",
      }),
    });
  });

  it("strips control characters from the title before sending", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(201, {}));
    vi.stubGlobal("fetch", fetchMock);
    renderForm(null);

    fireEvent.change(screen.getByLabelText("Title"), {
      target: { value: "Den\x00t\tist " },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    expect(fetchMock.mock.calls[0]?.[1]).toMatchObject({
      method: "POST",
      body: JSON.stringify({ title: "Dentist" }),
    });
  });

  it("rejects a control-only title before touching the network", () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    renderForm(null);

    fireEvent.change(screen.getByLabelText("Title"), {
      target: { value: "\x00\x07\t\n" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(screen.getByText("Title is required.")).toBeDefined();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("prefills and PATCHes, sending empty strings to clear fields", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, {}));
    vi.stubGlobal("fetch", fetchMock);
    renderForm(TASK);

    expect(screen.getByLabelText("Title").getAttribute("value")).toBe(
      "Buy keyboard",
    );
    expect(screen.getByLabelText("Due").getAttribute("value")).toBe(
      "2026-10-20",
    );
    expect(screen.getByLabelText("Event ID").getAttribute("value")).toBe("e9");

    fireEvent.change(screen.getByLabelText("Due"), { target: { value: "" } });
    fireEvent.change(screen.getByLabelText("Event ID"), {
      target: { value: "" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/tasks/t1");
    expect(fetchMock.mock.calls[0]?.[1]).toMatchObject({
      method: "PATCH",
      body: JSON.stringify({
        title: "Buy keyboard",
        due_date: "",
        event_id: "",
        note_id: "n9",
      }),
    });
  });

  it("rejects an empty title before touching the network", () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    renderForm(null);

    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(screen.getByText("Title is required.")).toBeDefined();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("shows server validation fields after a failed save", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        jsonResponse(400, {
          error: {
            code: "validation_error",
            message: "validation failed",
            fields: { event_id: "unknown event" },
          },
        }),
      ),
    );
    renderForm(null);

    fireEvent.change(screen.getByLabelText("Title"), {
      target: { value: "x" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(screen.getByText("event_id: unknown event")).toBeDefined(),
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
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0]?.[1]).toMatchObject({ method: "DELETE" });
  });

  it("shows an error and returns to the buttons when delete fails", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        jsonResponse(500, {
          error: { code: "internal_error", message: "server exploded" },
        }),
      ),
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
