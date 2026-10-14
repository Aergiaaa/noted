import { describe, expect, it } from "vitest";
import { getMonthMatrix } from "./month";

describe("getMonthMatrix", () => {
  it("returns 42 days (6 full weeks) for any month", () => {
    for (const [y, m] of [
      [2026, 9],
      [2026, 1],
      [2027, 11],
    ] as const) {
      expect(getMonthMatrix(y, m)).toHaveLength(42);
    }
  });

  it("starts on the Monday on or before the 1st (ISO weeks)", () => {
    // October 2026 starts on a Thursday → grid starts Mon Sep 28.
    const [first] = getMonthMatrix(2026, 9);

    expect(first.getFullYear()).toBe(2026);
    expect(first.getMonth()).toBe(8);
    expect(first.getDate()).toBe(28);
    expect(first.getDay()).toBe(1);
  });

  it("ends 41 days after the first cell", () => {
    const days = getMonthMatrix(2026, 9);
    const last = days[41];
    const expected = new Date(days[0]);
    expected.setDate(expected.getDate() + 41);

    expect(last.getFullYear()).toBe(expected.getFullYear());
    expect(last.getMonth()).toBe(expected.getMonth());
    expect(last.getDate()).toBe(expected.getDate());
  });

  it("contains every day of the month", () => {
    const days = getMonthMatrix(2026, 9).filter((d) => d.getMonth() === 9);

    expect(days).toHaveLength(31);
  });
});
