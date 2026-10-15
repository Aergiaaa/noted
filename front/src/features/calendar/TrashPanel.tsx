import type { MouseEvent } from "react";
import type { Event } from "../../lib/api";
import { formatMonthTitle, formatTime } from "../../lib/format";
import EmptyState from "../../components/ui/EmptyState";
import ErrorState from "../../components/ui/ErrorState";

const EMPTY_MESSAGE = "Nothing in the trash.";

interface TrashPanelProps {
  open: boolean;
  events: Event[];
  loading: boolean;
  error: string | null;
  onRestore: (id: string) => void;
  onRetry: () => void;
  onClose: () => void;
}

// Soft-deleted events (GET /api/events?trash=1); the page owns the fetch
// so loading flips in handlers, not effects.
export default function TrashPanel({
  open,
  events,
  loading,
  error,
  onRestore,
  onRetry,
  onClose,
}: TrashPanelProps) {
  const stop = (e: MouseEvent) => e.stopPropagation();

  if (!open) return null;

  return (
    <div className="modal modal-open" onClick={onClose}>
      <div className="modal-box w-full max-w-sm" onClick={stop}>
        <h2 className="text-base font-semibold">Trash</h2>
        <div className="mt-3">
          {loading ? (
            <div className="space-y-2">
              {Array.from({ length: 3 }, (_, i) => (
                <div key={i} className="skeleton h-10" />
              ))}
            </div>
          ) : error !== null ? (
            <ErrorState message={error} onRetry={onRetry} />
          ) : events.length === 0 ? (
            <EmptyState message={EMPTY_MESSAGE} />
          ) : (
            <ul className="divide-y divide-base-300">
              {events.map((event) => (
                <li
                  key={event.id}
                  className="flex items-center justify-between gap-2 py-2"
                >
                  <div className="min-w-0">
                    <p className="truncate text-sm font-medium">
                      {event.title}
                    </p>
                    <p className="text-xs text-base-content/60">
                      {formatMonthTitle(new Date(event.starts_at))}{" "}
                      {formatTime(event.starts_at)}
                    </p>
                  </div>
                  <button
                    type="button"
                    className="btn btn-ghost btn-sm"
                    onClick={() => onRestore(event.id)}
                  >
                    Restore
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
        <div className="modal-action">
          <button
            type="button"
            className="btn btn-ghost btn-sm"
            onClick={onClose}
          >
            Close
          </button>
        </div>
      </div>
    </div>
  );
}
