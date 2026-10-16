import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import type { Task } from "../../lib/api";
import TaskList from "./TaskList";

afterEach(() => {
  cleanup();
});

const TASKS: Task[] = [
  {
    id: "t1",
    title: "First",
    status: "open",
    due_date: null,
    event_id: null,
    note_id: null,
    completed_at: null,
  },
  {
    id: "t2",
    title: "Second",
    status: "done",
    due_date: null,
    event_id: null,
    note_id: null,
    completed_at: null,
  },
];

function renderList(tasks: Task[]) {
  const onToggleStatus = vi.fn();
  const onOpen = vi.fn();
  const view = render(
    <MemoryRouter initialEntries={["/tasks"]}>
      <TaskList
        tasks={tasks}
        overdueIds={new Set<string>()}
        onToggleStatus={onToggleStatus}
        onOpen={onOpen}
      />
    </MemoryRouter>,
  );
  return { ...view, onToggleStatus, onOpen };
}

describe("TaskList", () => {
  it("renders one row per task", () => {
    renderList(TASKS);
    expect(screen.getByRole("button", { name: "First" })).toBeDefined();
    expect(screen.getByRole("button", { name: "Second" })).toBeDefined();
    expect(screen.getByText("done")).toBeDefined();
  });

  it("passes the overdue flag through to the row", () => {
    const view = render(
      <MemoryRouter initialEntries={["/tasks"]}>
        <TaskList
          tasks={[TASKS[0]]}
          overdueIds={new Set(["t1"])}
          onToggleStatus={vi.fn()}
          onOpen={vi.fn()}
        />
      </MemoryRouter>,
    );
    expect(view.container.textContent).toContain("overdue");
  });

  it("routes status toggles back to the page", () => {
    const { onToggleStatus } = renderList(TASKS);
    fireEvent.click(screen.getByRole("button", { name: "Complete First" }));
    expect(onToggleStatus).toHaveBeenCalledWith(TASKS[0]);
  });

  it("renders no rows for an empty list", () => {
    renderList([]);
    expect(screen.queryByRole("button", { name: "Complete First" })).toBeNull();
    expect(screen.queryByText("done")).toBeNull();
  });
});
