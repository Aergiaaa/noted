import { Link } from "react-router";
import type { Task } from "../../lib/api";

interface TaskRowProps {
  task: Task;
  overdue: boolean;
  onToggleStatus: (task: Task) => void;
  onOpen: (task: Task) => void;
}

export default function TaskRow({
  task,
  overdue,
  onToggleStatus,
  onOpen,
}: TaskRowProps) {
  const done = task.status === "done";
  const badgeClass = done
    ? "badge badge-success badge-sm"
    : overdue
      ? "badge badge-warning badge-sm"
      : "badge badge-ghost badge-sm";
  const badgeText = done ? "done" : overdue ? "overdue" : "open";

  return (
    <tr data-testid="task-row">
      <td className="w-px whitespace-nowrap">
        <div className="flex items-center gap-1">
          <span className={badgeClass}>{badgeText}</span>
          <button
            type="button"
            className="btn btn-ghost btn-sm"
            aria-label={`${done ? "Reopen" : "Complete"} ${task.title}`}
            onClick={() => onToggleStatus(task)}
          >
            {done ? "Reopen" : "Complete"}
          </button>
          <button
            type="button"
            className="btn btn-ghost btn-sm"
            onClick={() => onOpen(task)}
          >
            Edit
          </button>
        </div>
      </td>
      <td>
        <button
          type="button"
          className="text-left font-medium"
          onClick={() => onOpen(task)}
        >
          {task.title}
        </button>
        <div className="mt-1 flex flex-wrap items-center gap-1">
          {task.due_date !== null && (
            <span className="tabular-nums text-xs text-base-content/60 md:hidden">
              {task.due_date}
            </span>
          )}
          {task.event_id !== null && (
            <Link className="badge badge-info badge-sm" to="/calendar">
              event
            </Link>
          )}
          {task.note_id !== null && (
            <Link className="badge badge-info badge-sm" to="/notes">
              note
            </Link>
          )}
        </div>
      </td>
      <td className="hidden tabular-nums md:table-cell">
        {task.due_date ?? ""}
      </td>
    </tr>
  );
}
