import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import type { Task } from "../../lib/api";
import TaskRow from "./TaskRow";

afterEach(() => {
  cleanup();
});

function task(over: Partial<Task> = {}): Task {
  return {
    id: "t1",
    title: "Buy milk",
    status: "open",
    due_date: null,
    event_id: null,
    note_id: null,
    completed_at: null,
    ...over,
  };
}

function renderRow(target: Task, overdue = false) {
  const onToggleStatus = vi.fn();
  const onOpen = vi.fn();
  const view = render(
    <MemoryRouter initialEntries={["/tasks"]}>
      <table>
        <tbody>
          <TaskRow
            task={target}
            overdue={overdue}
            onToggleStatus={onToggleStatus}
            onOpen={onOpen}
          />
        </tbody>
      </table>
    </MemoryRouter>,
  );
  return { ...view, onToggleStatus, onOpen };
}

describe("TaskRow", () => {
  it("shows the open badge for a fresh task and completes it", () => {
    const target = task();
    const { onToggleStatus } = renderRow(target);

    expect(screen.getByText("open")).toBeDefined();
    fireEvent.click(screen.getByRole("button", { name: "Complete Buy milk" }));
    expect(onToggleStatus).toHaveBeenCalledWith(target);
  });

  it("shows the done badge and reopens a finished task", () => {
    const { onToggleStatus } = renderRow(
      task({ status: "done", completed_at: "2026-10-15T09:00:00Z" }),
    );

    expect(screen.getByText("done")).toBeDefined();
    fireEvent.click(screen.getByRole("button", { name: "Reopen Buy milk" }));
    expect(onToggleStatus).toHaveBeenCalledWith(
      expect.objectContaining({ status: "done" }),
    );
  });

  it("shows the overdue badge for a late open task", () => {
    renderRow(task({ due_date: "2026-10-14" }), true);
    expect(screen.getByText("overdue")).toBeDefined();
  });

  it("opens the task from the title button", () => {
    const target = task();
    const { onOpen } = renderRow(target);

    fireEvent.click(screen.getByRole("button", { name: "Buy milk" }));
    expect(onOpen).toHaveBeenCalledWith(target);
  });

  it("opens the task from the Edit button", () => {
    const target = task();
    const { onOpen } = renderRow(target);

    fireEvent.click(screen.getByRole("button", { name: "Edit" }));
    expect(onOpen).toHaveBeenCalledWith(target);
  });

  it("renders the due day on both the mobile line and the desktop cell", () => {
    renderRow(task({ due_date: "2026-10-20" }));
    expect(screen.getAllByText("2026-10-20")).toHaveLength(2);
  });

  it("links event and note chips to their modules", () => {
    renderRow(task({ event_id: "e9", note_id: "n9" }));
    const event = screen.getByRole("link", { name: "event" });
    const note = screen.getByRole("link", { name: "note" });
    expect(event.getAttribute("href")).toBe("/calendar");
    expect(note.getAttribute("href")).toBe("/notes");
  });

  it("renders no chips for an unlinked task", () => {
    renderRow(task());
    expect(screen.queryByRole("link")).toBeNull();
  });

  it("omits the due day when the task has none", () => {
    renderRow(task());
    expect(screen.queryByText(/2026-/)).toBeNull();
  });
});
