import { useEffect, type MouseEvent } from "react";
import type { Event } from "../../lib/api";
import { formatDateTime, formatTime, toLocalInput } from "../../lib/format";

interface EventInfoProps {
  event: Event;
  onClose: () => void;
  onEdit: (event: Event) => void;
}

// Read-only details card between clicking a block and editing, so the
// description and full span are visible before any change. It reads the
// event already fetched with the week — no extra request.
export default function EventInfo({ event, onClose, onEdit }: EventInfoProps) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const stop = (e: MouseEvent) => e.stopPropagation();
  const description = event.description?.trim();

  return (
    <div className="modal modal-open" onClick={onClose}>
      <div className="modal-box w-full max-w-sm" onClick={stop}>
        <h2 className="break-words text-base font-semibold">{event.title}</h2>
        <p className="mt-2 text-sm tabular-nums text-base-content/70">
          {formatEventRange(event.starts_at, event.ends_at)}
        </p>
        <div className="mt-3">
          <span className="label-text text-xs uppercase text-base-content/60">
            Description
          </span>
          {description ? (
            <p className="mt-1 whitespace-pre-wrap text-sm">{description}</p>
          ) : (
            <p className="mt-1 text-sm text-base-content/50">No description.</p>
          )}
        </div>
        <div className="mt-4 flex justify-end gap-2">
          <button
            type="button"
            className="btn btn-ghost btn-sm"
            onClick={onClose}
          >
            Close
          </button>
          <button
            type="button"
            className="btn btn-primary btn-sm"
            onClick={() => onEdit(event)}
          >
            Edit
          </button>
        </div>
      </div>
    </div>
  );
}

function formatEventRange(starts: string, ends: string): string {
  const start = new Date(starts);
  const end = new Date(ends);
  const sameDay = start.toDateString() === end.toDateString();
  return sameDay
    ? `${fullWhen(starts)} – ${formatTime(ends)}`
    : `${fullWhen(starts)} – ${fullWhen(ends)}`;
}

function fullWhen(iso: string): string {
  return formatDateTime(toLocalInput(iso));
}
