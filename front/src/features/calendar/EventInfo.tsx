import type { Event } from "../../lib/api";
import { formatDateTime, formatTime, toLocalInput } from "../../lib/format";
import { eventColorDotClasses } from "./eventColor";
import Modal from "../../components/ui/Modal";

interface EventInfoProps {
  event: Event;
  onClose: () => void;
  onEdit: (event: Event) => void;
}

// Read-only details card between clicking a block and editing, so the
// description and full span are visible before any change. It reads the
// event already fetched with the week — no extra request. Escape and
// backdrop close come from Modal's focus trap.
export default function EventInfo({ event, onClose, onEdit }: EventInfoProps) {
  const description = event.description?.trim();

  return (
    <Modal onClose={onClose}>
      <div className="flex items-center gap-2">
        <span
          data-testid="event-color-dot"
          aria-hidden="true"
          className={`inline-block h-2 w-2 shrink-0 rounded-full ${eventColorDotClasses(event.id)}`}
        />
        <h2 className="break-words text-base font-semibold">{event.title}</h2>
      </div>
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
    </Modal>
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
