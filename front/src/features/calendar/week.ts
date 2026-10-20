// Local-time week math for the time grid (Monday-start weeks, UI.md).

const DAYS_PER_WEEK = 7;

// Monday 00:00:00.000 local of the week containing date.
export function startOfWeek(date: Date): Date {
  const back = (date.getDay() + DAYS_PER_WEEK - 1) % DAYS_PER_WEEK;
  return new Date(date.getFullYear(), date.getMonth(), date.getDate() - back);
}

// The seven local days Mon..Sun starting at weekStart.
export function weekDays(weekStart: Date): Date[] {
  return Array.from(
    { length: DAYS_PER_WEEK },
    (_, i) =>
      new Date(
        weekStart.getFullYear(),
        weekStart.getMonth(),
        weekStart.getDate() + i,
      ),
  );
}

// ISO instants covering Mon 00:00 .. Sun 23:59:59.999 for the fetch.
export function weekBounds(weekStart: Date): [string, string] {
  const to = new Date(
    weekStart.getFullYear(),
    weekStart.getMonth(),
    weekStart.getDate() + DAYS_PER_WEEK - 1,
    23,
    59,
    59,
    999,
  );
  return [weekStart.toISOString(), to.toISOString()];
}

// Shift a day by whole weeks (nav steps +/- 7 days).
export function addWeeks(date: Date, weeks: number): Date {
  return new Date(
    date.getFullYear(),
    date.getMonth(),
    date.getDate() + weeks * DAYS_PER_WEEK,
  );
}

// True when `day` falls inside the Mon..Sun week starting at weekStart.
// `day` must be a UTC-midnight instant (Cally's getDayParts convention)
// while weekStart is local midnight — compare calendar-day tuples, never
// instants, or timezones behind UTC land a day off.
export function isWithinWeek(day: Date, weekStart: Date): boolean {
  const dayKey =
    day.getUTCFullYear() * 10000 + day.getUTCMonth() * 100 + day.getUTCDate();
  const startKey = localDayKey(weekStart);
  const endKey = localDayKey(weekDays(weekStart)[6]);
  return dayKey >= startKey && dayKey <= endKey;
}

// Compact local calendar-day number, YYYYMMDD.
function localDayKey(date: Date): number {
  return date.getFullYear() * 10000 + date.getMonth() * 100 + date.getDate();
}
