import type { Task } from "../../lib/api";
import type { MentionType } from "../../lib/mentions";
import TaskRow from "./TaskRow";

interface TaskListProps {
  tasks: Task[];
  overdueIds: ReadonlySet<string>;
  onToggleStatus: (task: Task) => void;
  onOpen: (task: Task) => void;
  linkTitle?: (kind: MentionType, id: string) => string | undefined;
}

export default function TaskList({
  tasks,
  overdueIds,
  onToggleStatus,
  onOpen,
  linkTitle,
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
            linkTitle={linkTitle}
          />
        ))}
      </tbody>
    </table>
  );
}
