// ISO weeks (Monday first), always 6 rows so every month renders a
// uniform 42-cell grid — leading/trailing cells spill into neighbour months.
export function getMonthMatrix(year: number, monthIndex: number): Date[] {
  const first = new Date(year, monthIndex, 1);
  const shift = (first.getDay() + 6) % 7;

  const start = new Date(first);
  start.setDate(first.getDate() - shift);

  return Array.from({ length: 42 }, (_, i) => {
    const day = new Date(start);
    day.setDate(start.getDate() + i);
    return day;
  });
}
