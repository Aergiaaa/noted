import type { Event } from "../../lib/api";

const MIN_PER_DAY = 1440;
const MS_PER_MIN = 60000;

export interface PlacedEvent {
  event: Event;
  startMin: number;
  endMin: number;
  lane: number;
  laneCount: number;
  startsThisDay: boolean;
  endsThisDay: boolean;
}

interface Clip {
  event: Event;
  startMin: number;
  endMin: number;
  startsThisDay: boolean;
  endsThisDay: boolean;
}

export interface DayLayout {
  placed: PlacedEvent[];
  continuations: PlacedEvent[];
}

// Lay one day's events onto lanes: each event is clipped to the day
// window (multi-day events render once per day they overlap). Clips that
// neither start nor end on this day — the fully-covered middle days of a
// multi-day event — skip the lanes entirely and land in `continuations`:
// the grid renders them as thin strips in a reserved top band, so a
// passing-through event never eats a lane from real events below.
export function getDayLayout(events: Event[], day: Date): DayLayout {
  const dayStart = day.getTime();
  const dayEnd = dayStart + MIN_PER_DAY * MS_PER_MIN;

  const clips = events
    .map((event) => clipToDay(event, dayStart, dayEnd))
    .filter((clip): clip is Clip => clip !== null)
    .sort((a, b) => a.startMin - b.startMin || a.endMin - b.endMin);

  const passing: Clip[] = [];
  const overlapping = clips.filter((clip) => {
    if (clip.startsThisDay || clip.endsThisDay) return true;
    passing.push(clip);
    return false;
  });

  const laneEnds: number[] = [];
  const packed = overlapping.map((clip) => {
    const lane = laneEnds.findIndex((end) => end <= clip.startMin);
    if (lane === -1) {
      laneEnds.push(clip.endMin);
      return { ...clip, lane: laneEnds.length - 1 };
    }
    laneEnds[lane] = clip.endMin;
    return { ...clip, lane };
  });

  return {
    placed: packed.map((clip) => ({ ...clip, laneCount: laneEnds.length })),
    continuations: passing.map((clip) => ({
      ...clip,
      lane: 0,
      laneCount: 1,
    })),
  };
}

function clipToDay(
  event: Event,
  dayStart: number,
  dayEnd: number,
): Clip | null {
  const start = new Date(event.starts_at).getTime();
  const end = new Date(event.ends_at).getTime();
  const clipStart = Math.max(start, dayStart);
  const clipEnd = Math.min(end, dayEnd);
  if (clipEnd <= clipStart) return null;
  return {
    event,
    startMin: (clipStart - dayStart) / MS_PER_MIN,
    endMin: (clipEnd - dayStart) / MS_PER_MIN,
    startsThisDay: start >= dayStart,
    endsThisDay: end <= dayEnd,
  };
}
