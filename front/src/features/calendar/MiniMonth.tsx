import "cally";
import { fromDayKey, toDayKey } from "../../lib/format";
import { isWithinWeek, startOfWeek } from "./week";

interface MiniMonthProps {
  selected: Date;
  onSelect: (day: Date) => void;
}

// Cally renders into shadow DOM. Its built-in header (own month nav) is
// hidden via cally-headerless — the sidebar follows the page selection.
// `onchange` lowercase: React never delivers Cally's non-bubbling change.
// getDayParts lives on calendar-date (not calendar-month) — cally threads
// it through context; React would silently drop a function prop the
// element has no accessor for. It tags the grid's displayed week (cally
// hands UTC-midnight dates); index.css paints those parts as a soft row
// tint via ::part, lighter than the picked day.
export default function MiniMonth({ selected, onSelect }: MiniMonthProps) {
  const weekStart = startOfWeek(selected);
  return (
    <calendar-date
      className="cally cally-headerless"
      value={toDayKey(selected)}
      firstDayOfWeek={1}
      getDayParts={(day: Date) =>
        isWithinWeek(day, weekStart) ? "noted-week" : ""
      }
      onchange={(e) =>
        onSelect(fromDayKey((e.target as HTMLInputElement).value))
      }
    >
      <calendar-month className="cally-weekrow" />
    </calendar-date>
  );
}
