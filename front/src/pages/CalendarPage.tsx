import { useCallback, useEffect, useState } from "react";
import { api, isApiError, type Event, type EventsResponse } from "../lib/api";
import CalendarEventForm from "../features/calendar/EventForm";
import MiniMonth from "../features/calendar/MiniMonth";
import MonthHeader from "../features/calendar/MonthHeader";
import TrashPanel from "../features/calendar/TrashPanel";
import WeekTimeGrid from "../features/calendar/WeekTimeGrid";
import {
  addWeeks,
  startOfWeek,
  weekBounds,
  weekDays,
} from "../features/calendar/week";
import ErrorState from "../components/ui/ErrorState";
import EmptyState from "../components/ui/EmptyState";
import Toast, { type ToastData } from "../components/ui/Toast";
import { toDayKey } from "../lib/format";

const LOAD_FAILED = "Couldn't load events.";
const EMPTY_MESSAGE = "No events this week.";
const RESTORE_FAILED = "Couldn't restore the event.";

export default function CalendarPage() {
  const [selectedDate, setSelectedDate] = useState(() => new Date());
  const [weekDir, setWeekDir] = useState<1 | -1>(1);
  const [events, setEvents] = useState<Event[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);
  const [createStart, setCreateStart] = useState<Date | null>(null);
  const [editing, setEditing] = useState<Event | null>(null);
  const [toast, setToast] = useState<ToastData | null>(null);
  const [trashOpen, setTrashOpen] = useState(false);
  const [trashEvents, setTrashEvents] = useState<Event[]>([]);
  const [trashLoading, setTrashLoading] = useState(false);
  const [trashError, setTrashError] = useState<string | null>(null);
  const [trashNonce, setTrashNonce] = useState(0);

  useEffect(() => {
    let cancelled = false;
    api<EventsResponse>(eventsPathFor(selectedDate))
      .then((res) => {
        if (!cancelled) {
          setEvents(res.data);
          setError(null);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(isApiError(err) ? err.message : LOAD_FAILED);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [selectedDate, nonce]);

  useEffect(() => {
    if (!trashOpen) return;
    let cancelled = false;
    api<EventsResponse>("/events?trash=1")
      .then((res) => {
        if (!cancelled) {
          setTrashEvents(res.data);
          setTrashError(null);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled)
          setTrashError(isApiError(err) ? err.message : LOAD_FAILED);
      })
      .finally(() => {
        if (!cancelled) setTrashLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [trashOpen, trashNonce]);

  // Loading flips on only for the first load and retry, not on navigation:
  // the grid must stay mounted so the week-slide animation can play while
  // the next fetch swaps events in behind it.
  const refetch = () => {
    setError(null);
    setNonce((n) => n + 1);
  };
  const retry = () => {
    setError(null);
    setLoading(true);
    setNonce((n) => n + 1);
  };
  const stepWeek = (weeks: number) => {
    setError(null);
    setWeekDir(weeks > 0 ? 1 : -1);
    setSelectedDate((d) => addWeeks(d, weeks));
  };
  const selectDay = (day: Date) => {
    setError(null);
    setWeekDir(
      startOfWeek(day).getTime() >= startOfWeek(selectedDate).getTime()
        ? 1
        : -1,
    );
    setSelectedDate(day);
  };
  const openCreate = () => {
    setEditing(null);
    const now = new Date();
    setCreateStart(
      new Date(now.getFullYear(), now.getMonth(), now.getDate(), 9, 0),
    );
  };
  const createAt = (day: Date, hour: number) => {
    setEditing(null);
    setCreateStart(
      new Date(day.getFullYear(), day.getMonth(), day.getDate(), hour, 0),
    );
  };
  const closeForm = () => {
    setEditing(null);
    setCreateStart(null);
  };
  const openTrash = () => {
    setTrashError(null);
    setTrashLoading(true);
    setTrashOpen(true);
    setTrashNonce((n) => n + 1);
  };
  const retryTrash = () => {
    setTrashError(null);
    setTrashLoading(true);
    setTrashNonce((n) => n + 1);
  };
  const restoreEvent = async (id: string, after: () => void) => {
    try {
      await api(`/events/${id}/restore`, { method: "POST" });
      after();
    } catch (err: unknown) {
      setToast({
        message: isApiError(err) ? err.message : RESTORE_FAILED,
        kind: "error",
      });
    }
  };
  const undoDelete = (id: string) => {
    void restoreEvent(id, () => {
      setToast(null);
      refetch();
    });
  };
  const restoreFromTrash = (id: string) => {
    void restoreEvent(id, () => {
      setTrashNonce((n) => n + 1);
      refetch();
    });
  };
  const dismissToast = useCallback(() => setToast(null), []);

  if (error !== null) {
    return (
      <section>
        <h1 className="text-xl font-semibold">Calendar</h1>
        <ErrorState message={error} onRetry={retry} />
      </section>
    );
  }

  if (loading) {
    return (
      <section>
        <h1 className="text-xl font-semibold">Calendar</h1>
        <div className="mt-4 flex flex-col gap-2">
          {Array.from({ length: 7 }, (_, i) => (
            <div key={i} className="skeleton h-16" />
          ))}
        </div>
      </section>
    );
  }

  const weekStart = startOfWeek(selectedDate);

  return (
    <section>
      <header className="flex flex-wrap items-center justify-between gap-2">
        <h1 className="text-xl font-semibold">Calendar</h1>
        <div className="flex items-center gap-2">
          <MonthHeader
            month={selectedDate}
            onPrevWeek={() => stepWeek(-1)}
            onNextWeek={() => stepWeek(1)}
          />
          <button
            type="button"
            aria-label="Open trash"
            className="btn btn-ghost btn-sm"
            onClick={openTrash}
          >
            Trash
          </button>
          <button
            type="button"
            className="btn btn-primary btn-sm"
            onClick={openCreate}
          >
            New event
          </button>
        </div>
      </header>

      <div className="mt-4 flex flex-col gap-4 lg:flex-row">
        <aside className="shrink-0">
          <MiniMonth selected={selectedDate} onSelect={selectDay} />
          {events.length === 0 && (
            <EmptyState
              message={EMPTY_MESSAGE}
              action={{ label: "Create your first event", onClick: openCreate }}
            />
          )}
        </aside>
        <div className="min-w-0 flex-1">
          <div className="calendar-scroll flex h-[calc(100vh-9rem)] flex-col overflow-auto rounded-lg border border-base-300 bg-base-200">
            <div
              key={toDayKey(weekStart)}
              className={`flex flex-1 flex-col ${weekDir === 1 ? "week-slide-fwd" : "week-slide-back"}`}
            >
              <WeekTimeGrid
                days={weekDays(weekStart)}
                events={events}
                onSlotClick={createAt}
                onEventClick={(event) => {
                  setCreateStart(null);
                  setEditing(event);
                }}
              />
            </div>
          </div>
        </div>
      </div>

      {(createStart !== null || editing !== null) && (
        <CalendarEventForm
          event={editing}
          defaultStart={createStart ?? new Date()}
          onClose={closeForm}
          onSaved={refetch}
          onDeleted={(id) =>
            setToast({
              message: "Event deleted",
              kind: "success",
              action: { label: "Undo", onAction: () => undoDelete(id) },
            })
          }
        />
      )}

      <TrashPanel
        open={trashOpen}
        events={trashEvents}
        loading={trashLoading}
        error={trashError}
        onRestore={restoreFromTrash}
        onRetry={retryTrash}
        onClose={() => setTrashOpen(false)}
      />

      {toast !== null && <Toast toast={toast} onDismiss={dismissToast} />}
    </section>
  );
}

// The visible week Mon 00:00 .. Sun 23:59:59.999 local, as ISO instants.
function eventsPathFor(selectedDate: Date): string {
  const [from, to] = weekBounds(startOfWeek(selectedDate));
  return `/events?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`;
}
