// Local-time week math for the time grid (Monday-start weeks, UI.md).

// Monday 00:00:00.000 local of the week containing date.
export function startOfWeek(date: Date): Date {
  const back = (date.getDay() + 6) % 7;
  return new Date(date.getFullYear(), date.getMonth(), date.getDate() - back);
}

// The seven local days Mon..Sun starting at weekStart.
export function weekDays(weekStart: Date): Date[] {
  return Array.from(
    { length: 7 },
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
    weekStart.getDate() + 6,
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
    date.getDate() + weeks * 7,
  );
}
