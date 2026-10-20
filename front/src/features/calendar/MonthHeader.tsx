import { formatMonthTitle } from "../../lib/format";

interface MonthHeaderProps {
  month: Date;
  onPrevWeek: () => void;
  onNextWeek: () => void;
}

// Title flips with the selected day's month; arrows step whole weeks.
export default function MonthHeader({
  month,
  onPrevWeek,
  onNextWeek,
}: MonthHeaderProps) {
  return (
    <div className="flex items-center gap-2">
      <h2 className="text-sm font-medium">{formatMonthTitle(month)}</h2>
      <button
        type="button"
        aria-label="Previous week"
        className="btn btn-ghost btn-sm"
        onClick={onPrevWeek}
      >
        ‹
      </button>
      <button
        type="button"
        aria-label="Next week"
        className="btn btn-ghost btn-sm"
        onClick={onNextWeek}
      >
        ›
      </button>
    </div>
  );
}
