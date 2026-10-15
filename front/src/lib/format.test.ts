import { describe, expect, it } from "vitest";
import {
  formatDateTime,
  formatDayHeader,
  formatHourLabel,
  formatMonthTitle,
  formatTime,
  fromDayKey,
  toDayKey,
  toLocalInput,
} from "./format";

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

describe("toDayKey / fromDayKey", () => {
  it("round-trips a local date through its YYYY-MM-DD key", () => {
    const date = new Date(2026, 9, 5);
    expect(toDayKey(date)).toBe("2026-10-05");
    const back = fromDayKey("2026-10-05");
    expect(back.getFullYear()).toBe(2026);
    expect(back.getMonth()).toBe(9);
    expect(back.getDate()).toBe(5);
    expect(back.getHours()).toBe(0);
  });
});

describe("formatDayHeader", () => {
  it("renders a short weekday and day number", () => {
    expect(formatDayHeader(new Date(2026, 9, 12))).toBe("Mon 12");
    expect(formatDayHeader(new Date(2026, 9, 18))).toBe("Sun 18");
  });
});

describe("formatHourLabel", () => {
  it("zero-pads the hour", () => {
    expect(formatHourLabel(0)).toBe("00");
    expect(formatHourLabel(9)).toBe("09");
    expect(formatHourLabel(23)).toBe("23");
  });
});

describe("formatDateTime", () => {
  it("renders the picker trigger for a naive local datetime", () => {
    expect(formatDateTime("2026-10-15T14:37")).toBe("Thu 15 Oct 2026, 14:37");
    expect(formatDateTime("2026-01-03T07:05")).toBe("Sat 3 Jan 2026, 07:05");
  });
});
