import { describe, expect, it } from "vitest";
import { formatMonthTitle, formatTime, toLocalInput } from "./format";

describe("formatTime", () => {
  it("renders HH:MM from an instant in local time", () => {
    const iso = new Date(2026, 9, 13, 10, 5).toISOString();
    expect(formatTime(iso)).toBe("10:05");
  });

  it("zero-pads single-digit hours and minutes", () => {
    const iso = new Date(2026, 9, 13, 7, 5).toISOString();
    expect(formatTime(iso)).toBe("07:05");
  });
});

describe("formatMonthTitle", () => {
  it("names the month and year of the given date", () => {
    expect(formatMonthTitle(new Date(2026, 9, 1))).toBe("October 2026");
    expect(formatMonthTitle(new Date(2026, 0, 31))).toBe("January 2026");
  });
});

describe("toLocalInput", () => {
  it("converts an RFC3339 instant to a datetime-local value", () => {
    const iso = new Date(2026, 9, 13, 10, 5).toISOString();
    const value = toLocalInput(iso);

    expect(value).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/);
    expect(new Date(value).getTime()).toBe(new Date(iso).getTime());
  });
});
