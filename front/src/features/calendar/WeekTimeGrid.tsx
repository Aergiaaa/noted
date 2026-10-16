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
const MIN_PER_DAY = 1440;
const HOURS_PER_DAY = 24;

interface WeekTimeGridProps {
  days: Date[];
  events: Event[];
  onSlotClick: (day: Date, hour: number) => void;
  onEventClick: (event: Event) => void;
}

// Week grid, transposed: one row per day (dates on the y axis), 24 hour
// columns across (time on the x axis). The time axis sticks to the top
// above every row (z-40) and day labels stick to the left (z-30) while
// the container scrolls horizontally. Blocks position left/width by time
// of day, top/height by lane. Clicking an empty hour creates at that
// slot; clicking a block edits.
export default function WeekTimeGrid({
  days,
  events,
  onSlotClick,
  onEventClick,
}: WeekTimeGridProps) {
  const [now, setNow] = useState(() => new Date());
  // Hour cell under the pointer in whichever day row the cursor is in —
  // a covered hour's affordance reveals only when its own cell is
  // hovered, so a 1-6pm block shows the "+" just under the cursor.
  const [hoverCell, setHoverCell] = useState<{
    dayIndex: number;
    hour: number;
  } | null>(null);

  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), NOW_TICK_MS);
    return () => clearInterval(id);
  }, []);

  const dayLayouts = days.map((day) => getDayLayout(events, day));
  const nowKey = toDayKey(now);
  const nowMin = now.getHours() * 60 + now.getMinutes();

  return (
    <div className="flex min-w-[1200px] flex-1 flex-col">
      {/* Literal grid-cols: Tailwind can't see interpolated classes. The
          header's 5rem label column matches the rows' w-20, so the 24 hour
          columns line up exactly. */}
      <div
        data-testid="time-axis"
        className="sticky top-0 z-40 grid grid-cols-[5rem_repeat(24,minmax(0,1fr))] border-b border-base-300 bg-base-200"
      >
        <div
          className="sticky left-0 z-10 border-r border-base-300 bg-base-200"
          aria-hidden="true"
        />
        {HOURS.map((hour) => (
          <div
            key={hour}
            className="py-1 text-center text-[10px] tabular-nums text-base-content/50"
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
            className="flex min-h-16 flex-1 border-b border-base-300"
          >
            <div
              data-testid="day-label"
              className={`sticky left-0 z-30 flex w-20 shrink-0 items-center border-r border-base-300 bg-base-200 px-2 text-xs ${
                key === nowKey
                  ? "font-semibold text-primary"
                  : "text-base-content/70"
              }`}
            >
              {formatDayHeader(day)}
            </div>
            <div
              data-testid="day-row"
              className="relative flex flex-1"
              onMouseMove={(e) => {
                const rect = e.currentTarget.getBoundingClientRect();
                setHoverCell({
                  dayIndex,
                  hour: Math.floor(
                    ((e.clientX - rect.left) / rect.width) * HOURS_PER_DAY,
                  ),
                });
              }}
              onMouseLeave={() => setHoverCell(null)}
            >
              {HOURS.map((hour) => (
                <button
                  key={hour}
                  type="button"
                  className="h-full flex-1 border-l border-base-300 hover:bg-base-300"
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
                  className={`absolute z-20 flex h-5 w-5 items-center justify-center rounded-full bg-base-300/90 text-sm leading-none text-base-content/70 transition-opacity duration-150 focus-visible:opacity-100 [@media(hover:none)]:opacity-100 hover:bg-base-200 hover:text-primary ${
                    hoverCell?.dayIndex === dayIndex && hoverCell.hour === hour
                      ? "opacity-100"
                      : "opacity-0"
                  }`}
                  style={{
                    left: `calc(${((hour + 1) / HOURS_PER_DAY) * 100}% - 24px)`,
                    top: 4,
                  }}
                  aria-label={`Add event on ${formatDayHeader(day)} at ${formatHourLabel(hour)}:00`}
                  onClick={() => onSlotClick(day, hour)}
                >
                  <span aria-hidden="true">+</span>
                </button>
              ))}
              {key === nowKey && (
                <div
                  data-testid="now-line"
                  className="pointer-events-none absolute inset-y-0 z-20 w-0.5 bg-[#f87171]"
                  style={{ left: `${(nowMin / MIN_PER_DAY) * 100}%` }}
                  aria-hidden="true"
                />
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}

// True when a block hides the slot button underneath for the whole hour:
// full-height lane (laneCount 1) and spanning it end to end. Partial
// coverage or stacked lanes leave enough of the slot clickable.
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
  const left = (block.startMin / MIN_PER_DAY) * 100;
  const width = ((block.endMin - block.startMin) / MIN_PER_DAY) * 100;
  const top = (block.lane / block.laneCount) * 100;
  const height = (1 / block.laneCount) * 100;
  const rounding = `${block.startsThisDay ? "" : "rounded-l-none"} ${block.endsThisDay ? "" : "rounded-r-none"}`;

  return (
    <button
      type="button"
      className={`absolute z-10 overflow-hidden rounded border border-primary/40 bg-primary/20 px-1 text-left text-xs hover:bg-primary/30 ${rounding}`}
      style={{
        left: `${left}%`,
        width: `calc(${width}% - 2px)`,
        top: `${top}%`,
        height: `calc(${height}% - 2px)`,
        marginLeft: 1,
        marginTop: 1,
      }}
      aria-label={`${block.event.title} ${formatTime(block.event.starts_at)}`}
      onClick={() => onClick(block.event)}
    >
      <span className="truncate">{block.event.title}</span>
    </button>
  );
}
