// All calendar rendering is browser-local (DOMAIN.md: instants UTC in,
// local out). Values are pre-formatted strings, so no Intl options means
// no locale drift between machines.

const MONTHS = [
  "January",
  "February",
  "March",
  "April",
  "May",
  "June",
  "July",
  "August",
  "September",
  "October",
  "November",
  "December",
];

export function formatTime(iso: string): string {
  const date = new Date(iso);
  return `${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

export function formatMonthTitle(date: Date): string {
  return `${MONTHS[date.getMonth()]} ${date.getFullYear()}`;
}

// datetime-local input value (YYYY-MM-DDTHH:MM) for an instant.
export function toLocalInput(iso: string): string {
  const date = new Date(iso);
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

// Stable key for a local day: YYYY-MM-DD (grid aria-labels, comparisons).
export function toDayKey(date: Date): string {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

// Parse a YYYY-MM-DD day key back into a local Date (midnight).
export function fromDayKey(key: string): Date {
  const [year, month, day] = key.split("-").map(Number);
  return new Date(year, month - 1, day);
}

const WEEKDAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

// Weekday header for the time grid: "Mon 12".
export function formatDayHeader(date: Date): string {
  return `${WEEKDAYS[date.getDay()]} ${date.getDate()}`;
}

// Compact hour label for the time gutter: "09".
export function formatHourLabel(hour: number): string {
  return pad(hour);
}

function pad(n: number): string {
  return String(n).padStart(2, "0");
}
