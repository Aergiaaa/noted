import "cally";
import { fromDayKey, toDayKey } from "../../lib/format";

interface MiniMonthProps {
  selected: Date;
  onSelect: (day: Date) => void;
}

// Cally renders into shadow DOM. Its built-in header (own month nav) is
// hidden via cally-headerless — the sidebar follows the page selection.
// `onchange` lowercase: React never delivers Cally's non-bubbling change.
export default function MiniMonth({ selected, onSelect }: MiniMonthProps) {
  return (
    <calendar-date
      className="cally cally-headerless"
      value={toDayKey(selected)}
      firstDayOfWeek={1}
      onchange={(e) =>
        onSelect(fromDayKey((e.target as HTMLInputElement).value))
      }
    >
      <calendar-month />
    </calendar-date>
  );
}
