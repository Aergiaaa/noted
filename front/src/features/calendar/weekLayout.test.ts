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

describe("getDayLayout", () => {
  it("places a same-day event with minute offsets and full-width lane", () => {
    const e = event(
      "a",
      localISO(2026, 10, 12, 9, 30),
      localISO(2026, 10, 12, 11),
    );
    const monday = getDayLayout([e], MON);
    expect(monday).toHaveLength(1);
    expect(monday[0]).toMatchObject({
      startMin: 9 * 60 + 30,
      endMin: 11 * 60,
      lane: 0,
      laneCount: 1,
      startsThisDay: true,
      endsThisDay: true,
    });
    expect(getDayLayout([e], TUE)).toHaveLength(0);
  });

  it("packs overlapping events into side-by-side lanes", () => {
    const a = event("a", localISO(2026, 10, 12, 9), localISO(2026, 10, 12, 11));
    const b = event(
      "b",
      localISO(2026, 10, 12, 10),
      localISO(2026, 10, 12, 12),
    );
    const monday = getDayLayout([a, b], MON);
    expect(monday.map((c) => [c.event.id, c.lane, c.laneCount])).toEqual([
      ["a", 0, 2],
      ["b", 1, 2],
    ]);
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
    const laneOf = Object.fromEntries(monday.map((p) => [p.event.id, p.lane]));
    expect(laneOf).toEqual({ a: 0, b: 1, c: 0 });
    expect(monday[0]?.laneCount).toBe(2);
  });

  it("splits a multi-day event across day windows with edge flags", () => {
    // Mon 22:00 -> Tue 02:00.
    const e = event("n", localISO(2026, 10, 12, 22), localISO(2026, 10, 13, 2));
    const monday = getDayLayout([e], MON);
    const tuesday = getDayLayout([e], TUE);
    expect(monday).toHaveLength(1);
    expect(monday[0]).toMatchObject({
      startMin: 22 * 60,
      endMin: 1440,
      startsThisDay: true,
      endsThisDay: false,
    });
    expect(tuesday).toHaveLength(1);
    expect(tuesday[0]).toMatchObject({
      startMin: 0,
      endMin: 2 * 60,
      startsThisDay: false,
      endsThisDay: true,
    });
  });

  it("clips an event that started before the day", () => {
    const e = event(
      "old",
      localISO(2026, 10, 11, 23),
      localISO(2026, 10, 12, 1),
    );
    const monday = getDayLayout([e], MON);
    expect(monday[0]).toMatchObject({
      startMin: 0,
      endMin: 60,
      startsThisDay: false,
      endsThisDay: true,
    });
  });

  it("skips events that end exactly at the day start", () => {
    // Ends Mon 00:00 — belongs to Sunday, not Monday.
    const e = event("e", localISO(2026, 10, 11, 22), localISO(2026, 10, 12));
    expect(getDayLayout([e], MON)).toHaveLength(0);
  });

  it("skips events entirely outside the day", () => {
    const e = event("x", localISO(2026, 10, 20, 9), localISO(2026, 10, 20, 10));
    expect(getDayLayout([e], MON)).toHaveLength(0);
    expect(getDayLayout([e], TUE)).toHaveLength(0);
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
    expect(monday.map((p) => p.event.id)).toEqual(["short", "long"]);
  });
});
