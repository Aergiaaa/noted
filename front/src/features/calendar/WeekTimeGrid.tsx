import { useEffect, useState } from "react";
import type { Event } from "../../lib/api";
import {
  formatDayHeader,
  formatHourLabel,
  formatTime,
  toDayKey,
} from "../../lib/format";
import { getDayLayout, type PlacedEvent } from "./weekLayout";

const HOURS = Array.from({ length: 24 }, (_, hour) => hour);
const NOW_TICK_MS = 60000;
// Hour rows are h-14 (3.5rem) — the covered-slot affordance is positioned
// in px against the column, so the two must stay in step.
const SLOT_PX = 56;

interface WeekTimeGridProps {
  days: Date[];
  events: Event[];
  onSlotClick: (day: Date, hour: number) => void;
  onEventClick: (event: Event) => void;
}

// Week time-grid: 7 day columns over 24 hour rows, red now-line, events
// as positioned blocks (lane-packed by getWeekLayout). Clicking an empty
// hour creates at that slot; clicking a block edits.
export default function WeekTimeGrid({
  days,
  events,
  onSlotClick,
  onEventClick,
}: WeekTimeGridProps) {
  const [now, setNow] = useState(() => new Date());
  // Hour row under the pointer in whichever day column the cursor is in —
  // a covered hour's affordance reveals only when its own row is hovered,
  // so a 1-6pm block shows the "+" just under the cursor, not for all hours.
  const [hoverHour, setHoverHour] = useState<number | null>(null);

  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), NOW_TICK_MS);
    return () => clearInterval(id);
  }, []);

  const dayLayouts = days.map((day) => getDayLayout(events, day));
  const nowKey = toDayKey(now);
  const nowMin = now.getHours() * 60 + now.getMinutes();

  return (
    <div className="min-w-[700px]">
      <div className="sticky top-0 z-10 grid grid-cols-[3rem_repeat(7,minmax(0,1fr))] border-b border-base-300 bg-base-200">
        <div aria-hidden="true" />
        {days.map((day) => {
          const key = toDayKey(day);
          return (
            <div
              key={key}
              className={`px-1 py-1 text-center text-xs ${
                key === nowKey
                  ? "font-semibold text-primary"
                  : "text-base-content/70"
              }`}
            >
              {formatDayHeader(day)}
            </div>
          );
        })}
      </div>
      <div className="grid grid-cols-[3rem_repeat(7,minmax(0,1fr))]">
        <div>
          {HOURS.map((hour) => (
            <div
              key={hour}
              className="h-14 pr-1 text-right text-[10px] tabular-nums text-base-content/50"
            >
              {formatHourLabel(hour)}
            </div>
          ))}
        </div>
        {days.map((day, dayIndex) => {
          const key = toDayKey(day);
          return (
            <div
              key={key}
              data-testid="day-column"
              className="relative border-l border-base-300"
              onMouseMove={(e) => {
                const rect = e.currentTarget.getBoundingClientRect();
                setHoverHour(Math.floor((e.clientY - rect.top) / SLOT_PX));
              }}
              onMouseLeave={() => setHoverHour(null)}
            >
              {HOURS.map((hour) => (
                <button
                  key={hour}
                  type="button"
                  className="block h-14 w-full border-t border-base-300 hover:bg-base-300"
                  aria-label={`Create event on ${formatDayHeader(day)} at ${formatHourLabel(hour)}:00`}
                  onClick={() => onSlotClick(day, hour)}
                />
              ))}
              {dayLayouts[dayIndex].map((block) => (
                <EventBlock
                  key={`${block.event.id}-${block.lane}`}
                  block={block}
                  onClick={onEventClick}
                />
              ))}
              {HOURS.filter((hour) =>
                dayLayouts[dayIndex].some((block) =>
                  isHourCovered(block, hour),
                ),
              ).map((hour) => (
                <button
                  key={hour}
                  type="button"
                  className={`absolute right-1 z-30 flex h-5 w-5 items-center justify-center rounded-full bg-base-300/90 text-sm leading-none text-base-content/70 transition-opacity duration-150 focus-visible:opacity-100 [@media(hover:none)]:opacity-100 hover:bg-base-200 hover:text-primary ${hoverHour === hour ? "opacity-100" : "opacity-0"}`}
                  style={{ top: hour * SLOT_PX + 3 }}
                  aria-label={`Add event on ${formatDayHeader(day)} at ${formatHourLabel(hour)}:00`}
                  onClick={() => onSlotClick(day, hour)}
                >
                  <span aria-hidden="true">+</span>
                </button>
              ))}
              {key === nowKey && (
                <div
                  data-testid="now-line"
                  className="pointer-events-none absolute inset-x-0 z-20 h-0.5 bg-[#f87171]"
                  style={{ top: `${(nowMin / 1440) * 100}%` }}
                  aria-hidden="true"
                />
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

// True when a block hides the slot button underneath for the whole hour:
// full-width (laneCount 1) and spanning it end to end. Partial coverage
// or half-width lanes leave enough of the slot clickable.
function isHourCovered(block: PlacedEvent, hour: number): boolean {
  return (
    block.laneCount === 1 &&
    block.startMin <= hour * 60 &&
    block.endMin >= (hour + 1) * 60
  );
}

interface EventBlockProps {
  block: PlacedEvent;
  onClick: (event: Event) => void;
}

function EventBlock({ block, onClick }: EventBlockProps) {
  const top = (block.startMin / 1440) * 100;
  const height = ((block.endMin - block.startMin) / 1440) * 100;
  const left = (block.lane / block.laneCount) * 100;
  const width = (1 / block.laneCount) * 100;
  const rounding = `${block.startsThisDay ? "" : "rounded-l-none"} ${block.endsThisDay ? "" : "rounded-r-none"}`;

  return (
    <button
      type="button"
      className={`absolute z-10 overflow-hidden rounded border border-primary/40 bg-primary/20 px-1 text-left text-xs hover:bg-primary/30 ${rounding}`}
      style={{
        top: `${top}%`,
        height: `${height}%`,
        left: `${left}%`,
        width: `calc(${width}% - 2px)`,
        marginLeft: 1,
      }}
      aria-label={`${block.event.title} ${formatTime(block.event.starts_at)}`}
      onClick={() => onClick(block.event)}
    >
      <span className="truncate">{block.event.title}</span>
    </button>
  );
}
