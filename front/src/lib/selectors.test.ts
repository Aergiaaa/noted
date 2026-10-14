import { describe, expect, it } from "vitest";
import type { Event } from "./api";
import { getEventsForDay, isSameDay } from "./selectors";

// Local-constructed dates round-tripped through toISOString keep these
// assertions correct in any test TZ (the "local day" is what we bucket by).
function localISO(
  year: number,
  month: number,
  day: number,
  hour = 0,
  minute = 0,
): string {
  return new Date(year, month - 1, day, hour, minute).toISOString();
}

function event(id: string, startsAt: string, title = `event ${id}`): Event {
  return {
    id,
    title,
    starts_at: startsAt,
    ends_at: startsAt,
    description: null,
    note_id: null,
  };
}

describe("isSameDay", () => {
  it("is true for two instants on the same local day", () => {
    expect(
      isSameDay(localISO(2026, 10, 13, 0, 5), localISO(2026, 10, 13, 23, 55)),
    ).toBe(true);
  });

  it("is false across a local midnight boundary", () => {
    expect(
      isSameDay(localISO(2026, 10, 13, 23, 30), localISO(2026, 10, 14, 0, 30)),
    ).toBe(false);
  });

  it("is false for different months and years", () => {
    expect(isSameDay(localISO(2026, 10, 13), localISO(2026, 11, 13))).toBe(
      false,
    );
    expect(isSameDay(localISO(2026, 10, 13), localISO(2025, 10, 13))).toBe(
      false,
    );
  });
});

describe("getEventsForDay", () => {
  it("keeps only events starting on that local day, sorted by starts_at", () => {
    const events = [
      event("b", localISO(2026, 10, 13, 15)),
      event("a", localISO(2026, 10, 13, 9)),
      event("other", localISO(2026, 10, 14, 9)),
    ];
    const day = new Date(2026, 9, 13, 12);

    const got = getEventsForDay(events, day);

    expect(got.map((e) => e.id)).toEqual(["a", "b"]);
  });

  it("is empty when the day has no events", () => {
    const events = [event("x", localISO(2026, 10, 13, 9))];

    expect(getEventsForDay(events, new Date(2026, 9, 20))).toEqual([]);
  });
});
