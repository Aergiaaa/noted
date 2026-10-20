import { useEffect, useState, type FormEvent, type MouseEvent } from "react";
import { api, errorLines, type Task } from "../../lib/api";
import { sanitizeLine } from "../../lib/sanitize";

const SAVE_FAILED = "Couldn't save the task.";
const TITLE_REQUIRED = "Title is required.";

interface TaskFormProps {
  task: Task | null;
  onClose: () => void;
  onSaved: () => void;
  onDeleted?: (id: string) => void;
}

export default function TaskForm({
  task,
  onClose,
  onSaved,
  onDeleted,
}: TaskFormProps) {
  const [title, setTitle] = useState(task?.title ?? "");
  const [due, setDue] = useState(task?.due_date ?? "");
  const [eventId, setEventId] = useState(task?.event_id ?? "");
  const [noteId, setNoteId] = useState(task?.note_id ?? "");
  const [errors, setErrors] = useState<string[]>([]);
  const [saving, setSaving] = useState(false);
  const [confirming, setConfirming] = useState(false);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const stop = (e: MouseEvent) => e.stopPropagation();
  // Confirming only ever opens in edit mode; "" is a placeholder that can
  // never reach confirmDelete.
  const editId = task?.id ?? "";

  const onSubmit = async (formEvent: FormEvent) => {
    formEvent.preventDefault();
    if (sanitizeLine(title) === "") {
      setErrors([TITLE_REQUIRED]);
      return;
    }

    setErrors([]);
    setSaving(true);
    const cleanTitle = sanitizeLine(title);
    // Create omits empty optionals; PATCH always sends them so "" clears a
    // stored value (events normalize the same way server-side).
    const optional = {
      ...(due !== "" ? { due_date: due } : {}),
      ...(eventId.trim() !== "" ? { event_id: eventId.trim() } : {}),
      ...(noteId.trim() !== "" ? { note_id: noteId.trim() } : {}),
    };
    const patch = {
      title: cleanTitle,
      due_date: due,
      event_id: eventId.trim(),
      note_id: noteId.trim(),
    };

    try {
      if (task === null) {
        await api("/tasks", {
          method: "POST",
          body: { title: cleanTitle, ...optional },
        });
      } else {
        await api(`/tasks/${task.id}`, { method: "PATCH", body: patch });
      }
      onSaved();
      onClose();
    } catch (err: unknown) {
      setErrors(errorLines(err, SAVE_FAILED));
    } finally {
      setSaving(false);
    }
  };

  const confirmDelete = async (id: string) => {
    setSaving(true);
    try {
      await api(`/tasks/${id}`, { method: "DELETE" });
      onDeleted?.(id);
      onSaved();
      onClose();
    } catch (err: unknown) {
      setErrors(errorLines(err, SAVE_FAILED));
      setConfirming(false);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="modal modal-open" onClick={onClose}>
      <div className="modal-box w-full max-w-sm" onClick={stop}>
        <h2 className="text-base font-semibold">
          {task === null ? "New task" : "Edit task"}
        </h2>
        <form className="mt-3 space-y-3" onSubmit={onSubmit} noValidate>
          <label className="block">
            <span className="label-text text-xs uppercase text-base-content/60">
              Title
            </span>
            <input
              className="input input-bordered input-sm mt-1 w-full"
              aria-label="Title"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
            />
          </label>
          <label className="block">
            <span className="label-text text-xs uppercase text-base-content/60">
              Due
            </span>
            <input
              type="date"
              className="input input-bordered input-sm mt-1 w-full"
              aria-label="Due"
              value={due}
              onChange={(e) => setDue(e.target.value)}
            />
          </label>
          <label className="block">
            <span className="label-text text-xs uppercase text-base-content/60">
              Event ID
            </span>
            <input
              className="input input-bordered input-sm mt-1 w-full"
              aria-label="Event ID"
              placeholder="optional"
              value={eventId}
              onChange={(e) => setEventId(e.target.value)}
            />
          </label>
          <label className="block">
            <span className="label-text text-xs uppercase text-base-content/60">
              Note ID
            </span>
            <input
              className="input input-bordered input-sm mt-1 w-full"
              aria-label="Note ID"
              placeholder="optional"
              value={noteId}
              onChange={(e) => setNoteId(e.target.value)}
            />
          </label>

          {errors.length > 0 && (
            <ul className="text-xs text-error" role="alert">
              {errors.map((line) => (
                <li key={line}>{line}</li>
              ))}
            </ul>
          )}

          {confirming ? (
            <div className="mt-2 space-y-2 rounded-md border border-base-300 p-3">
              <p className="text-sm">Delete this task?</p>
              <div className="flex justify-end gap-2">
                <button
                  type="button"
                  className="btn btn-ghost btn-sm"
                  onClick={() => setConfirming(false)}
                >
                  Keep it
                </button>
                <button
                  type="button"
                  className="btn btn-error btn-sm"
                  onClick={() => confirmDelete(editId)}
                >
                  Yes, delete
                </button>
              </div>
            </div>
          ) : (
            <div className="mt-4 flex justify-end gap-2">
              {task !== null && (
                <button
                  type="button"
                  className="btn btn-error btn-sm"
                  onClick={() => setConfirming(true)}
                >
                  Delete
                </button>
              )}
              <button
                type="button"
                className="btn btn-ghost btn-sm"
                onClick={onClose}
              >
                Cancel
              </button>
              <button
                type="submit"
                className="btn btn-primary btn-sm"
                disabled={saving}
              >
                {saving ? "Saving…" : "Save"}
              </button>
            </div>
          )}
        </form>
      </div>
    </div>
  );
}
