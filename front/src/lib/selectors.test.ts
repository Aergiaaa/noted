import { describe, expect, it } from "vitest";
import type { Task } from "./api";
import { getOverdueTasks } from "./selectors";

function task(over: Partial<Task> = {}): Task {
  return {
    id: "t1",
    title: "Task",
    status: "open",
    due_date: null,
    event_id: null,
    note_id: null,
    completed_at: null,
    ...over,
  };
}

describe("getOverdueTasks", () => {
  it("keeps open tasks due strictly before today", () => {
    const tasks = [task({ id: "t1", due_date: "2026-10-14" })];
    expect(getOverdueTasks(tasks, "2026-10-15")).toEqual(tasks);
  });

  it("drops tasks due today or later", () => {
    const tasks = [
      task({ id: "t1", due_date: "2026-10-15" }),
      task({ id: "t2", due_date: "2026-10-16" }),
    ];
    expect(getOverdueTasks(tasks, "2026-10-15")).toEqual([]);
  });

  it("drops done tasks even when their due day passed", () => {
    const tasks = [task({ status: "done", due_date: "2026-10-01" })];
    expect(getOverdueTasks(tasks, "2026-10-15")).toEqual([]);
  });

  it("drops open tasks without a due date", () => {
    expect(getOverdueTasks([task()], "2026-10-15")).toEqual([]);
  });
});
