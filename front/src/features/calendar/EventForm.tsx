import { useEffect, useState, type FormEvent, type MouseEvent } from "react";
import { api, isApiError, type Event } from "../../lib/api";
import { toLocalInput } from "../../lib/format";

const SAVE_FAILED = "Couldn't save the event.";
const TITLE_REQUIRED = "Title is required.";
const ENDS_AFTER_STARTS = "Ends must be after starts.";

interface EventFormProps {
  event: Event | null;
  defaultStart: Date;
  onClose: () => void;
  onSaved: () => void;
  onDeleted?: (id: string) => void;
}

export default function EventForm({
  event,
  defaultStart,
  onClose,
  onSaved,
  onDeleted,
}: EventFormProps) {
  const [title, setTitle] = useState(event?.title ?? "");
  const [starts, setStarts] = useState(
    toLocalInput(event?.starts_at ?? defaultStart.toISOString()),
  );
  const [ends, setEnds] = useState(
    toLocalInput(event?.ends_at ?? plusHour(defaultStart).toISOString()),
  );
  const [description, setDescription] = useState(event?.description ?? "");
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
  const editId = event?.id ?? "";

  const onSubmit = async (formEvent: FormEvent) => {
    formEvent.preventDefault();
    if (title.trim() === "") {
      setErrors([TITLE_REQUIRED]);
      return;
    }
    if (new Date(ends) <= new Date(starts)) {
      setErrors([ENDS_AFTER_STARTS]);
      return;
    }

    setErrors([]);
    setSaving(true);
    const trimmed = description.trim();
    const body = {
      title: title.trim(),
      starts_at: new Date(starts).toISOString(),
      ends_at: new Date(ends).toISOString(),
      description: trimmed === "" ? undefined : trimmed,
    };

    try {
      if (event === null) await api("/events", { method: "POST", body });
      else await api(`/events/${event.id}`, { method: "PATCH", body });
      onSaved();
      onClose();
    } catch (err: unknown) {
      setErrors(saveErrorLines(err));
    } finally {
      setSaving(false);
    }
  };

  const confirmDelete = async (id: string) => {
    setSaving(true);
    try {
      await api(`/events/${id}`, { method: "DELETE" });
      onDeleted?.(id);
      onSaved();
      onClose();
    } catch (err: unknown) {
      setErrors(saveErrorLines(err));
      setConfirming(false);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="modal modal-open" onClick={onClose}>
      <div className="modal-box w-full max-w-sm" onClick={stop}>
        <h2 className="text-base font-semibold">
          {event === null ? "New event" : "Edit event"}
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
              Starts
            </span>
            <input
              type="datetime-local"
              className="input input-bordered input-sm mt-1 w-full"
              aria-label="Starts"
              value={starts}
              onChange={(e) => setStarts(e.target.value)}
            />
          </label>
          <label className="block">
            <span className="label-text text-xs uppercase text-base-content/60">
              Ends
            </span>
            <input
              type="datetime-local"
              className="input input-bordered input-sm mt-1 w-full"
              aria-label="Ends"
              value={ends}
              onChange={(e) => setEnds(e.target.value)}
            />
          </label>
          <label className="block">
            <span className="label-text text-xs uppercase text-base-content/60">
              Description
            </span>
            <textarea
              className="textarea textarea-bordered mt-1 w-full text-sm"
              aria-label="Description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
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
              <p className="text-sm">Delete this event?</p>
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
              {event !== null && (
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

// Server envelope fields (API.md validation_error) become "field: message";
// anything else falls back to the generic save failure.
function saveErrorLines(err: unknown): string[] {
  if (!isApiError(err)) return [SAVE_FAILED];
  if (err.fields !== undefined) {
    return Object.entries(err.fields).map(
      ([field, message]) => `${field}: ${message}`,
    );
  }
  return [err.message];
}

function plusHour(instant: Date): Date {
  return new Date(instant.getTime() + 60 * 60 * 1000);
}
