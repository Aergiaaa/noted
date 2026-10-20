import type { Task } from "./api";

// Overdue = still open and its due calendar day is strictly before today.
// Comparison is plain DATE string order (YYYY-MM-DD sorts lexicographically).
export function getOverdueTasks(tasks: Task[], today: string): Task[] {
  return tasks.filter(
    (task) =>
      task.status === "open" && task.due_date !== null && task.due_date < today,
  );
}
