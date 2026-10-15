import { describe, expect, it } from "vitest";
import { addWeeks, startOfWeek, weekBounds, weekDays } from "./week";

describe("startOfWeek", () => {
  it("walks back to Monday from any weekday", () => {
    // 2026-10-14 is a Wednesday; its week starts Monday 2026-10-12.
    const wednesday = new Date(2026, 9, 14, 15, 30);
    const start = startOfWeek(wednesday);
    expect(start.getFullYear()).toBe(2026);
    expect(start.getMonth()).toBe(9);
    expect(start.getDate()).toBe(12);
    expect(start.getHours()).toBe(0);
    expect(start.getMinutes()).toBe(0);
  });

  it("keeps Monday as itself and folds Sunday into the same week", () => {
    const monday = startOfWeek(new Date(2026, 9, 12, 9));
    expect(monday.getDate()).toBe(12);
    // Sunday 2026-10-18 belongs to the week that started 2026-10-12.
    const sunday = startOfWeek(new Date(2026, 9, 18, 23));
    expect(sunday.getDate()).toBe(12);
  });
});

describe("weekDays", () => {
  it("lists seven consecutive local days", () => {
    const days = weekDays(new Date(2026, 9, 12));
    expect(days).toHaveLength(7);
    expect(days[0]?.getDate()).toBe(12);
    expect(days[6]?.getDate()).toBe(18);
  });
});

describe("weekBounds", () => {
  it("covers Monday 00:00 through Sunday 23:59:59.999 local", () => {
    const [from, to] = weekBounds(new Date(2026, 9, 12));
    const fromDate = new Date(from);
    const toDate = new Date(to);
    expect(fromDate.getHours()).toBe(0);
    expect(fromDate.getDate()).toBe(12);
    expect(toDate.getHours()).toBe(23);
    expect(toDate.getMinutes()).toBe(59);
    expect(toDate.getDate()).toBe(18);
    expect(toDate.getMilliseconds()).toBe(999);
  });
});

describe("addWeeks", () => {
  it("steps across month boundaries", () => {
    const next = addWeeks(new Date(2026, 9, 29), 1);
    expect(next.getMonth()).toBe(10);
    expect(next.getDate()).toBe(5);
    const back = addWeeks(new Date(2026, 9, 1), -1);
    expect(back.getMonth()).toBe(8);
    expect(back.getDate()).toBe(24);
  });
});
