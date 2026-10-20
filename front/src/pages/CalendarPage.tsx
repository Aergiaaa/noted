import { useCallback, useState } from "react";
import { api, isApiError, type Event, type EventsResponse } from "../lib/api";
import CalendarEventForm from "../features/calendar/EventForm";
import EventInfo from "../features/calendar/EventInfo";
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
import { useFetch } from "../lib/useFetch";

const LOAD_FAILED = "Couldn't load events.";
const EMPTY_MESSAGE = "No events this week.";
const RESTORE_FAILED = "Couldn't restore the event.";

// One dialog at a time: the union makes view+create stacking unrepresentable.
type Dialog =
  | { kind: "view"; event: Event }
  | { kind: "create"; start: Date }
  | { kind: "edit"; event: Event };

// Week cursor: the anchor date and the slide direction always move together.
interface Cursor {
  date: Date;
  dir: 1 | -1;
}

export default function CalendarPage() {
  const [cursor, setCursor] = useState<Cursor>(() => ({
    date: new Date(),
    dir: 1,
  }));
  const [dialog, setDialog] = useState<Dialog | null>(null);
  const [toast, setToast] = useState<ToastData | null>(null);
  const [trashOpen, setTrashOpen] = useState(false);

  const events = useFetch<EventsResponse>(
    eventsPathFor(cursor.date),
    LOAD_FAILED,
  );

  const stepWeek = (weeks: number) => {
    setCursor((c) => ({
      date: addWeeks(c.date, weeks),
      dir: weeks > 0 ? 1 : -1,
    }));
  };
  const selectDay = (day: Date) => {
    setCursor((c) => ({
      date: day,
      dir: startOfWeek(day).getTime() >= startOfWeek(c.date).getTime() ? 1 : -1,
    }));
  };
  const openCreate = () => {
    const now = new Date();
    setDialog({
      kind: "create",
      start: new Date(now.getFullYear(), now.getMonth(), now.getDate(), 9, 0),
    });
  };
  const createAt = (day: Date, hour: number) => {
    setDialog({
      kind: "create",
      start: new Date(
        day.getFullYear(),
        day.getMonth(),
        day.getDate(),
        hour,
        0,
      ),
    });
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
      events.refetch();
    });
  };
  const dismissToast = useCallback(() => setToast(null), []);

  if (events.error !== null) {
    return (
      <section>
        <h1 className="text-xl font-semibold">Calendar</h1>
        <ErrorState message={events.error} onRetry={events.retry} />
      </section>
    );
  }

  if (events.loading) {
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

  const weekStart = startOfWeek(cursor.date);
  // The loading/error early-returns above guarantee the fetch has landed,
  // so data is non-null here (keep-previous navigation always carries the
  // previous success's data forward).
  const weekEvents = (events.data as EventsResponse).data;

  return (
    <section>
      <header className="flex flex-wrap items-center justify-between gap-2">
        <h1 className="text-xl font-semibold">Calendar</h1>
        <div className="flex items-center gap-2">
          <MonthHeader
            month={cursor.date}
            onPrevWeek={() => stepWeek(-1)}
            onNextWeek={() => stepWeek(1)}
          />
          <button
            type="button"
            aria-label="Open trash"
            className="btn btn-ghost btn-sm"
            onClick={() => setTrashOpen(true)}
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
          <MiniMonth selected={cursor.date} onSelect={selectDay} />
          {weekEvents.length === 0 && (
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
              className={`flex flex-1 flex-col ${cursor.dir === 1 ? "week-slide-fwd" : "week-slide-back"}`}
            >
              <WeekTimeGrid
                days={weekDays(weekStart)}
                events={weekEvents}
                onSlotClick={createAt}
                onEventClick={(event) => setDialog({ kind: "view", event })}
              />
            </div>
          </div>
        </div>
      </div>

      {dialog?.kind === "view" && (
        <EventInfo
          event={dialog.event}
          onClose={() => setDialog(null)}
          onEdit={(event) => setDialog({ kind: "edit", event })}
        />
      )}

      {(dialog?.kind === "create" || dialog?.kind === "edit") && (
        <CalendarEventForm
          event={dialog.kind === "edit" ? dialog.event : null}
          defaultStart={dialog.kind === "create" ? dialog.start : new Date()}
          onClose={() => setDialog(null)}
          onSaved={events.refetch}
          onDeleted={(id) =>
            setToast({
              message: "Event deleted",
              kind: "success",
              action: { label: "Undo", onAction: () => undoDelete(id) },
            })
          }
        />
      )}

      {trashOpen && (
        <TrashPanel
          onClose={() => setTrashOpen(false)}
          onRestored={events.refetch}
          onRestoreError={(message) => setToast({ message, kind: "error" })}
        />
      )}

      {toast !== null && <Toast toast={toast} onDismiss={dismissToast} />}
    </section>
  );
}

// The visible week Mon 00:00 .. Sun 23:59:59.999 local, as ISO instants.
function eventsPathFor(selectedDate: Date): string {
  const [from, to] = weekBounds(startOfWeek(selectedDate));
  return `/events?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`;
}
