import { describe, expect, it } from "vitest";
import type { Event } from "../../lib/api";
import { getDayLayout } from "./weekLayout";

// Local-constructed instants keep every assertion correct in any test TZ.
function localISO(
  year: number,
  month: number,
  day: number,
  hour = 0,
  minute = 0,
): string {
  return new Date(year, month - 1, day, hour, minute).toISOString();
}

function event(id: string, startsAt: string, endsAt: string): Event {
  return {
    id,
    title: `event ${id}`,
    starts_at: startsAt,
    ends_at: endsAt,
    description: null,
    note_id: null,
  };
}

const MON = new Date(2026, 9, 12);
const TUE = new Date(2026, 9, 13);
const WED = new Date(2026, 9, 14);

describe("getDayLayout", () => {
  it("places a same-day event with minute offsets and full-width lane", () => {
    const e = event(
      "a",
      localISO(2026, 10, 12, 9, 30),
      localISO(2026, 10, 12, 11),
    );
    const monday = getDayLayout([e], MON);
    expect(monday.placed).toHaveLength(1);
    expect(monday.continuations).toHaveLength(0);
    expect(monday.placed[0]).toMatchObject({
      startMin: 9 * 60 + 30,
      endMin: 11 * 60,
      lane: 0,
      laneCount: 1,
      startsThisDay: true,
      endsThisDay: true,
    });
    expect(getDayLayout([e], TUE).placed).toHaveLength(0);
  });

  it("packs overlapping events into side-by-side lanes", () => {
    const a = event("a", localISO(2026, 10, 12, 9), localISO(2026, 10, 12, 11));
    const b = event(
      "b",
      localISO(2026, 10, 12, 10),
      localISO(2026, 10, 12, 12),
    );
    const monday = getDayLayout([a, b], MON);
    expect(monday.placed.map((c) => [c.event.id, c.lane, c.laneCount])).toEqual(
      [
        ["a", 0, 2],
        ["b", 1, 2],
      ],
    );
  });

  it("reuses a free lane after an earlier event ends", () => {
    const a = event("a", localISO(2026, 10, 12, 9), localISO(2026, 10, 12, 10));
    const b = event(
      "b",
      localISO(2026, 10, 12, 9, 30),
      localISO(2026, 10, 12, 11),
    );
    const c = event(
      "c",
      localISO(2026, 10, 12, 10),
      localISO(2026, 10, 12, 12),
    );
    const monday = getDayLayout([a, b, c], MON);
    const laneOf = Object.fromEntries(
      monday.placed.map((p) => [p.event.id, p.lane]),
    );
    expect(laneOf).toEqual({ a: 0, b: 1, c: 0 });
    expect(monday.placed[0]?.laneCount).toBe(2);
  });

  it("splits a multi-day event across day windows with edge flags", () => {
    // Mon 22:00 -> Tue 02:00 — start and end days, no middle day.
    const e = event("n", localISO(2026, 10, 12, 22), localISO(2026, 10, 13, 2));
    const monday = getDayLayout([e], MON);
    const tuesday = getDayLayout([e], TUE);
    expect(monday.placed).toHaveLength(1);
    expect(monday.placed[0]).toMatchObject({
      startMin: 22 * 60,
      endMin: 1440,
      startsThisDay: true,
      endsThisDay: false,
    });
    expect(tuesday.placed).toHaveLength(1);
    expect(tuesday.placed[0]).toMatchObject({
      startMin: 0,
      endMin: 2 * 60,
      startsThisDay: false,
      endsThisDay: true,
    });
  });

  it("routes a fully-covered middle day to continuations, not lanes", () => {
    // Mon 10:00 -> Wed 16:00 crosses all of Tuesday.
    const e = event(
      "trip",
      localISO(2026, 10, 12, 10),
      localISO(2026, 10, 14, 16),
    );
    const monday = getDayLayout([e], MON);
    expect(monday.placed.map((p) => [p.startsThisDay, p.endsThisDay])).toEqual([
      [true, false],
    ]);
    expect(monday.continuations).toHaveLength(0);

    const tuesday = getDayLayout([e], TUE);
    expect(tuesday.placed).toHaveLength(0);
    expect(tuesday.continuations).toHaveLength(1);
    expect(tuesday.continuations[0]).toMatchObject({
      startMin: 0,
      endMin: 1440,
      startsThisDay: false,
      endsThisDay: false,
    });

    const wednesday = getDayLayout([e], WED);
    expect(wednesday.placed).toHaveLength(1);
    expect(wednesday.placed[0]).toMatchObject({
      startsThisDay: false,
      endsThisDay: true,
    });
    expect(wednesday.continuations).toHaveLength(0);
  });

  it("keeps continuations out of the lane pool for real events", () => {
    const through = event(
      "m",
      localISO(2026, 10, 12, 10),
      localISO(2026, 10, 14, 10),
    );
    const local = event(
      "a",
      localISO(2026, 10, 13, 9),
      localISO(2026, 10, 13, 11),
    );
    const tuesday = getDayLayout([through, local], TUE);
    // The passing-through event no longer inflates laneCount on Tuesday.
    expect(tuesday.placed.map((p) => [p.event.id, p.laneCount])).toEqual([
      ["a", 1],
    ]);
    expect(tuesday.continuations.map((p) => p.event.id)).toEqual(["m"]);
  });

  it("clips an event that started before the day", () => {
    const e = event(
      "old",
      localISO(2026, 10, 11, 23),
      localISO(2026, 10, 12, 1),
    );
    const monday = getDayLayout([e], MON);
    expect(monday.placed[0]).toMatchObject({
      startMin: 0,
      endMin: 60,
      startsThisDay: false,
      endsThisDay: true,
    });
  });

  it("skips events that end exactly at the day start", () => {
    // Ends Mon 00:00 — belongs to Sunday, not Monday.
    const e = event("e", localISO(2026, 10, 11, 22), localISO(2026, 10, 12));
    expect(getDayLayout([e], MON).placed).toHaveLength(0);
  });

  it("skips events entirely outside the day", () => {
    const e = event("x", localISO(2026, 10, 20, 9), localISO(2026, 10, 20, 10));
    expect(getDayLayout([e], MON).placed).toHaveLength(0);
    expect(getDayLayout([e], TUE).placed).toHaveLength(0);
  });

  it("orders same-minute clips by end time", () => {
    const long = event(
      "long",
      localISO(2026, 10, 12, 9),
      localISO(2026, 10, 12, 11),
    );
    const short = event(
      "short",
      localISO(2026, 10, 12, 9),
      localISO(2026, 10, 12, 9, 30),
    );
    const monday = getDayLayout([long, short], MON);
    expect(monday.placed.map((p) => p.event.id)).toEqual(["short", "long"]);
  });
});
