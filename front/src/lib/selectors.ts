import type { Event } from "./api";

// Bucket by local calendar day: starts_at/ends_at are UTC instants
// (DOMAIN.md) but the calendar shows the user's days.
export function isSameDay(a: string, b: string): boolean {
  const left = new Date(a);
  const right = new Date(b);

  return (
    left.getFullYear() === right.getFullYear() &&
    left.getMonth() === right.getMonth() &&
    left.getDate() === right.getDate()
  );
}

export function getEventsForDay(events: Event[], day: Date): Event[] {
  const dayInstant = day.toISOString();
  return events
    .filter((event) => isSameDay(event.starts_at, dayInstant))
    .sort((a, b) => a.starts_at.localeCompare(b.starts_at));
}
