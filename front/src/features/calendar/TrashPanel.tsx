import { useFetch } from "../../lib/useFetch";
import { api, isApiError, type EventsResponse } from "../../lib/api";
import { formatMonthTitle, formatTime } from "../../lib/format";
import { eventColorDotClasses } from "./eventColor";
import EmptyState from "../../components/ui/EmptyState";
import ErrorState from "../../components/ui/ErrorState";
import Modal from "../../components/ui/Modal";

const EMPTY_MESSAGE = "Nothing in the trash.";
const LOAD_FAILED = "Couldn't load events.";
const RESTORE_FAILED = "Couldn't restore the event.";

interface TrashPanelProps {
  onClose: () => void;
  onRestored: () => void;
  onRestoreError: (message: string) => void;
}

// Soft-deleted events (GET /api/events?trash=1); the panel owns its fetch
// and restore, notifying the page only to refetch the week or show a toast.
// Conditionally mounted by the page (mount = open), so the useFetch lifecycle
// starts fresh each time it opens.
export default function TrashPanel({
  onClose,
  onRestored,
  onRestoreError,
}: TrashPanelProps) {
  const trash = useFetch<EventsResponse>("/events?trash=1", LOAD_FAILED);

  const restore = async (id: string) => {
    try {
      await api(`/events/${id}/restore`, { method: "POST" });
      trash.refetch();
      onRestored();
    } catch (err: unknown) {
      onRestoreError(isApiError(err) ? err.message : RESTORE_FAILED);
    }
  };

  return (
    <Modal onClose={onClose}>
      <h2 className="text-base font-semibold">Trash</h2>
      <div className="mt-3">
        {trash.loading ? (
          <div className="space-y-2">
            {Array.from({ length: 3 }, (_, i) => (
              <div key={i} className="skeleton h-10" />
            ))}
          </div>
        ) : trash.error !== null ? (
          <ErrorState message={trash.error} onRetry={trash.retry} />
        ) : trash.data === null || trash.data.data.length === 0 ? (
          <EmptyState message={EMPTY_MESSAGE} />
        ) : (
          <ul className="divide-y divide-base-300">
            {trash.data.data.map((event) => (
              <li
                key={event.id}
                className="flex items-center justify-between gap-2 py-2"
              >
                <div className="min-w-0">
                  <p className="flex items-center gap-2 text-sm font-medium">
                    <span
                      data-testid="event-color-dot"
                      aria-hidden="true"
                      className={`inline-block h-2 w-2 shrink-0 rounded-full ${eventColorDotClasses(event.id)}`}
                    />
                    <span className="truncate">{event.title}</span>
                  </p>
                  <p className="text-xs text-base-content/60">
                    {formatMonthTitle(new Date(event.starts_at))}{" "}
                    {formatTime(event.starts_at)}
                  </p>
                </div>
                <button
                  type="button"
                  className="btn btn-ghost btn-sm"
                  onClick={() => void restore(event.id)}
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
    </Modal>
  );
}
