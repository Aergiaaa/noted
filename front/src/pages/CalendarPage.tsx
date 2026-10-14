import { useEffect, useState } from "react";
import { api, isApiError, type Event, type EventsResponse } from "../lib/api";
import { formatMonthTitle } from "../lib/format";
import CalendarGrid from "../features/calendar/CalendarGrid";
import EventForm from "../features/calendar/EventForm";
import { getMonthMatrix } from "../features/calendar/month";
import EmptyState from "../components/ui/EmptyState";
import ErrorState from "../components/ui/ErrorState";

const LOAD_FAILED = "Couldn't load events.";
const EMPTY_MESSAGE = "No events this month.";

export default function CalendarPage() {
  const [month, setMonth] = useState(
    () => new Date(new Date().getFullYear(), new Date().getMonth(), 1),
  );
  const [events, setEvents] = useState<Event[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);
  const [selectedDay, setSelectedDay] = useState<Date | null>(null);
  const [createDay, setCreateDay] = useState<Date | null>(null);
  const [editing, setEditing] = useState<Event | null>(null);

  useEffect(() => {
    let cancelled = false;
    api<EventsResponse>(eventsPathFor(month))
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
  }, [month, nonce]);

  // Loading flips on in the handlers below, not in the effect, so a render
  // never starts a cascade (lint: setState-in-effect).
  const retry = () => {
    setError(null);
    setLoading(true);
    setNonce((n) => n + 1);
  };
  const prevMonth = () => {
    setError(null);
    setLoading(true);
    setMonth((m) => new Date(m.getFullYear(), m.getMonth() - 1, 1));
  };
  const nextMonth = () => {
    setError(null);
    setLoading(true);
    setMonth((m) => new Date(m.getFullYear(), m.getMonth() + 1, 1));
  };
  const openCreate = () => {
    setEditing(null);
    setCreateDay(new Date());
  };
  const pickDay = (day: Date) => {
    setEditing(null);
    setCreateDay(day);
    setSelectedDay(day);
  };
  const closeForm = () => {
    setEditing(null);
    setCreateDay(null);
  };

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
        <div className="mt-4 grid grid-cols-7">
          {Array.from({ length: 42 }, (_, i) => (
            <div key={i} className="skeleton h-24" />
          ))}
        </div>
      </section>
    );
  }

  const days = getMonthMatrix(month.getFullYear(), month.getMonth());

  return (
    <section>
      <header className="flex flex-wrap items-center justify-between gap-2">
        <h1 className="text-xl font-semibold">Calendar</h1>
        <div className="flex items-center gap-2">
          <h2 className="text-sm font-medium">{formatMonthTitle(month)}</h2>
          <button
            type="button"
            aria-label="Previous month"
            className="btn btn-ghost btn-sm"
            onClick={prevMonth}
          >
            ‹
          </button>
          <button
            type="button"
            aria-label="Next month"
            className="btn btn-ghost btn-sm"
            onClick={nextMonth}
          >
            ›
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

      <CalendarGrid
        days={days}
        month={month}
        today={new Date()}
        selectedDay={selectedDay}
        events={events}
        onSelectDay={pickDay}
        onSelectEvent={(event) => {
          setCreateDay(null);
          setEditing(event);
        }}
      />

      {events.length === 0 && (
        <EmptyState
          message={EMPTY_MESSAGE}
          action={{ label: "Create your first event", onClick: openCreate }}
        />
      )}

      {(createDay !== null || editing !== null) && (
        <EventForm
          event={editing}
          defaultDay={createDay ?? new Date()}
          onClose={closeForm}
          onSaved={retry}
        />
      )}
    </section>
  );
}

// Range = the visible 42-cell grid, not just the month, so spill cells
// from the neighbouring months fetch their events too.
function eventsPathFor(month: Date): string {
  const [first, last] = gridBounds(month);
  return `/events?from=${encodeURIComponent(first)}&to=${encodeURIComponent(last)}`;
}

function gridBounds(month: Date): [string, string] {
  const days = getMonthMatrix(month.getFullYear(), month.getMonth());
  const from = days[0];
  from.setHours(0, 0, 0, 0);
  const to = days[41];
  to.setHours(23, 59, 59, 999);
  return [from.toISOString(), to.toISOString()];
}
