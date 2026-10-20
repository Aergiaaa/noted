import { useEffect, useState, type FormEvent, type MouseEvent } from "react";
import { api, errorLines, type Note } from "../../lib/api";
import { sanitizeBody, sanitizeLine } from "../../lib/sanitize";

const SAVE_FAILED = "Couldn't save the note.";
const TITLE_REQUIRED = "Title is required.";
const BODY_MAX_CHAR = 100000;

interface NoteFormProps {
  note: Note | null;
  onClose: () => void;
  onSaved: () => void;
  onDeleted?: (id: string) => void;
}

export default function NoteForm({
  note,
  onClose,
  onSaved,
  onDeleted,
}: NoteFormProps) {
  const [title, setTitle] = useState(note?.title ?? "");
  const [body, setBody] = useState(note?.body_md ?? "");
  const [pinned, setPinned] = useState(note?.pinned ?? false);
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
  const editId = note?.id ?? "";

  const onSubmit = async (formEvent: FormEvent) => {
    formEvent.preventDefault();
    if (sanitizeLine(title) === "") {
      setErrors([TITLE_REQUIRED]);
      return;
    }

    setErrors([]);
    setSaving(true);
    const bodyPayload = {
      title: sanitizeLine(title),
      body_md: sanitizeBody(body),
      pinned,
    };

    try {
      if (note === null) {
        await api("/notes", { method: "POST", body: bodyPayload });
      } else {
        await api(`/notes/${note.id}`, { method: "PATCH", body: bodyPayload });
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
      await api(`/notes/${id}`, { method: "DELETE" });
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
          {note === null ? "New note" : "Edit note"}
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
              Body
            </span>
            <textarea
              className="textarea textarea-bordered mt-1 w-full text-sm"
              aria-label="Body"
              maxLength={BODY_MAX_CHAR}
              value={body}
              onChange={(e) => setBody(e.target.value)}
            />
          </label>
          <label className="label cursor-pointer justify-start gap-2">
            <input
              type="checkbox"
              className="checkbox checkbox-sm"
              aria-label="Pinned"
              checked={pinned}
              onChange={(e) => setPinned(e.target.checked)}
            />
            <span className="label-text">Pinned</span>
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
              <p className="text-sm">Delete this note?</p>
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
              {note !== null && (
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
