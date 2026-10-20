import type { Task } from "../../lib/api";
import TaskRow from "./TaskRow";

interface TaskListProps {
  tasks: Task[];
  overdueIds: ReadonlySet<string>;
  onToggleStatus: (task: Task) => void;
  onOpen: (task: Task) => void;
}

export default function TaskList({
  tasks,
  overdueIds,
  onToggleStatus,
  onOpen,
}: TaskListProps) {
  return (
    <table className="table table-sm">
      <tbody>
        {tasks.map((task) => (
          <TaskRow
            key={task.id}
            task={task}
            overdue={overdueIds.has(task.id)}
            onToggleStatus={onToggleStatus}
            onOpen={onOpen}
          />
        ))}
      </tbody>
    </table>
  );
}
