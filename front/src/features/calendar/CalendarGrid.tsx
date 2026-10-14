import type { Event } from "../../lib/api";
import { toDayKey } from "../../lib/format";
import { getEventsForDay } from "../../lib/selectors";
import EventChip from "./EventChip";

const WEEKDAYS = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];
const CHIPS_PER_DAY = 3;

interface CalendarGridProps {
  days: Date[];
  month: Date;
  today: Date;
  selectedDay: Date | null;
  events: Event[];
  onSelectDay: (day: Date) => void;
  onSelectEvent: (event: Event) => void;
}

export default function CalendarGrid({
  days,
  month,
  today,
  selectedDay,
  events,
  onSelectDay,
  onSelectEvent,
}: CalendarGridProps) {
  const todayKey = toDayKey(today);
  const selectedKey = selectedDay === null ? null : toDayKey(selectedDay);

  return (
    <div className="mt-4">
      <div className="grid grid-cols-7 gap-px text-center text-xs uppercase text-base-content/60">
        {WEEKDAYS.map((weekday) => (
          <div key={weekday} className="py-1">
            {weekday}
          </div>
        ))}
      </div>
      <div className="grid grid-cols-7">
        {days.map((day) => {
          const key = toDayKey(day);
          const dayEvents = getEventsForDay(events, day);
          const visible = dayEvents.slice(0, CHIPS_PER_DAY);
          const hidden = dayEvents.length - visible.length;
          const inMonth =
            day.getMonth() === month.getMonth() &&
            day.getFullYear() === month.getFullYear();
          const cellClass = [
            "min-h-24 cursor-pointer border border-base-300 p-1 hover:bg-base-300",
            inMonth ? "" : "opacity-40",
            key === todayKey ? "ring-1 ring-primary ring-inset" : "",
            key === selectedKey ? "bg-primary/10" : "",
          ].join(" ");

          return (
            <div
              key={key}
              role="gridcell"
              aria-label={key}
              className={cellClass}
              onClick={() => onSelectDay(day)}
            >
              <span className="text-base-content/70">{day.getDate()}</span>
              <div className="mt-1 space-y-0.5">
                {visible.map((event) => (
                  <EventChip
                    key={event.id}
                    event={event}
                    onOpen={onSelectEvent}
                  />
                ))}
                {hidden > 0 && (
                  <span className="text-base-content/60">+{hidden} more</span>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
