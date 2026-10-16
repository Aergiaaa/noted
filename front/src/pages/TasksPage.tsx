import { useEffect, useState } from "react";
import { api, errorLines, type Task, type TasksResponse } from "../lib/api";
import TaskList from "../features/tasks/TaskList";
import TaskForm from "../features/tasks/TaskForm";
import ErrorState from "../components/ui/ErrorState";
import EmptyState from "../components/ui/EmptyState";
import { getOverdueTasks } from "../lib/selectors";
import { toDayKey } from "../lib/format";

const LOAD_FAILED = "Couldn't load tasks.";
const UPDATE_FAILED = "Couldn't update the task.";

type StatusFilter = "all" | "open" | "done";

export default function TasksPage() {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [status, setStatus] = useState<StatusFilter>("all");
  const [due, setDue] = useState("");
  // Once per mount: render stays pure (react/purity) and the overdue
  // bucket keeps the day the list was opened.
  const [todayKey] = useState(() => toDayKey(new Date()));
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<Task | null>(null);

  useEffect(() => {
    let cancelled = false;
    api<TasksResponse>(tasksPath(status, due))
      .then((res) => {
        if (!cancelled) {
          setTasks(res.data);
          setError(null);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(errorLines(err, LOAD_FAILED)[0]);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [status, due, nonce]);

  const refetch = () => {
    setError(null);
    setNonce((n) => n + 1);
  };
  const retry = () => {
    setError(null);
    setLoading(true);
    setNonce((n) => n + 1);
  };
  const changeFilter = (
    next: Partial<{ status: StatusFilter; due: string }>,
  ) => {
    setError(null);
    if (next.status !== undefined) setStatus(next.status);
    if (next.due !== undefined) setDue(next.due);
  };
  const toggleStatus = async (task: Task) => {
    try {
      await api(`/tasks/${task.id}`, {
        method: "PATCH",
        body: { status: task.status === "done" ? "open" : "done" },
      });
      refetch();
    } catch (err: unknown) {
      setError(errorLines(err, UPDATE_FAILED)[0]);
    }
  };
  const closeForm = () => {
    setCreating(false);
    setEditing(null);
  };

  if (error !== null) {
    return (
      <section>
        <h1 className="text-xl font-semibold">Tasks</h1>
        <ErrorState message={error} onRetry={retry} />
      </section>
    );
  }

  if (loading) {
    return (
      <section>
        <h1 className="text-xl font-semibold">Tasks</h1>
        <div className="mt-4 space-y-3">
          {Array.from({ length: 3 }, (_, i) => (
            <div key={i} className="skeleton h-12" />
          ))}
        </div>
      </section>
    );
  }

  const overdueIds = new Set(
    getOverdueTasks(tasks, todayKey).map((task) => task.id),
  );
  const filtered = status !== "all" || due !== "";

  return (
    <section>
      <header className="flex flex-wrap items-center justify-between gap-2">
        <h1 className="text-xl font-semibold">Tasks</h1>
        <button
          type="button"
          className="btn btn-primary btn-sm"
          onClick={() => setCreating(true)}
        >
          New task
        </button>
      </header>

      <div className="mt-4 flex flex-wrap gap-2">
        <label className="flex items-center gap-2">
          <span className="text-xs uppercase text-base-content/60">Status</span>
          <select
            className="select select-bordered select-sm"
            aria-label="Status filter"
            value={status}
            onChange={(e) =>
              changeFilter({ status: e.target.value as StatusFilter })
            }
          >
            <option value="all">All</option>
            <option value="open">Open</option>
            <option value="done">Done</option>
          </select>
        </label>
        <label className="flex items-center gap-2">
          <span className="text-xs uppercase text-base-content/60">Due</span>
          <input
            type="date"
            className="input input-bordered input-sm"
            aria-label="Due date filter"
            value={due}
            onChange={(e) => changeFilter({ due: e.target.value })}
          />
        </label>
      </div>

      <div className="mt-4">
        {tasks.length === 0 ? (
          <EmptyState
            message={
              filtered ? "No tasks match these filters." : "No tasks yet."
            }
            action={{
              label: "Create your first task",
              onClick: () => setCreating(true),
            }}
          />
        ) : (
          <TaskList
            tasks={tasks}
            overdueIds={overdueIds}
            onToggleStatus={(task) => void toggleStatus(task)}
            onOpen={setEditing}
          />
        )}
      </div>

      {(creating || editing !== null) && (
        <TaskForm
          task={creating ? null : editing}
          onClose={closeForm}
          onSaved={refetch}
          onDeleted={refetch}
        />
      )}
    </section>
  );
}

function tasksPath(status: StatusFilter, due: string): string {
  const query = new URLSearchParams();
  if (status !== "all") query.set("status", status);
  if (due !== "") query.set("due", due);
  const qs = query.toString();
  return qs === "" ? "/tasks" : `/tasks?${qs}`;
}
