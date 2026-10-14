import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import CalendarGrid from "./CalendarGrid";
import { getMonthMatrix } from "./month";
import type { Event } from "../../lib/api";

afterEach(cleanup);

const OCTOBER_2026 = getMonthMatrix(2026, 9);
const TODAY = new Date(2026, 9, 14, 12);
const SELECTED = new Date(2026, 9, 20, 12);

function ev(id: string, day: number, title: string, hour = 10): Event {
  const starts_at = new Date(2026, 9, day, hour, 0).toISOString();
  return {
    id,
    title,
    starts_at,
    ends_at: starts_at,
    description: null,
    note_id: null,
  };
}

function renderGrid(
  overrides: Partial<Parameters<typeof CalendarGrid>[0]> = {},
) {
  const onSelectDay = vi.fn();
  const onSelectEvent = vi.fn();
  render(
    <CalendarGrid
      days={OCTOBER_2026}
      month={new Date(2026, 9, 1)}
      today={TODAY}
      selectedDay={SELECTED}
      events={[]}
      onSelectDay={onSelectDay}
      onSelectEvent={onSelectEvent}
      {...overrides}
    />,
  );
  return { onSelectDay, onSelectEvent };
}

describe("CalendarGrid", () => {
  it("renders weekday headers and 42 day cells", () => {
    renderGrid();

    for (const name of ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"]) {
      expect(screen.getAllByText(name).length).toBeGreaterThan(0);
    }
    expect(screen.getAllByRole("gridcell")).toHaveLength(42);
  });

  it("dims cells outside the viewed month", () => {
    renderGrid();

    const spill = screen.getByRole("gridcell", { name: "2026-09-28" });
    const inMonth = screen.getByRole("gridcell", { name: "2026-10-13" });
    expect(spill.className).toContain("opacity-40");
    expect(inMonth.className).not.toContain("opacity-40");
  });

  it("highlights today and the selected day", () => {
    renderGrid();

    const today = screen.getByRole("gridcell", { name: "2026-10-14" });
    const selected = screen.getByRole("gridcell", { name: "2026-10-20" });
    expect(today.className).toContain("ring-primary");
    expect(selected.className).toContain("bg-primary/10");
  });

  it("picks a day when an empty cell is clicked", () => {
    const { onSelectDay, onSelectEvent } = renderGrid();

    screen.getByRole("gridcell", { name: "2026-10-13" }).click();

    expect(onSelectDay).toHaveBeenCalledTimes(1);
    expect(onSelectDay.mock.calls[0][0].getDate()).toBe(13);
    expect(onSelectEvent).not.toHaveBeenCalled();
  });

  it("opens an event when its chip is clicked instead of the day", () => {
    const { onSelectDay, onSelectEvent } = renderGrid({
      events: [ev("e1", 13, "Dentist")],
    });

    screen.getByRole("button", { name: /Dentist/ }).click();

    expect(onSelectEvent).toHaveBeenCalledTimes(1);
    expect(onSelectDay).not.toHaveBeenCalled();
  });

  it("caps visible chips per day and shows the overflow count", () => {
    renderGrid({
      events: [
        ev("e1", 13, "Standup", 9),
        ev("e2", 13, "Review", 10),
        ev("e3", 13, "1:1", 11),
        ev("e4", 13, "Retro", 12),
      ],
    });

    expect(screen.getByRole("button", { name: /Standup/ })).toBeDefined();
    expect(screen.queryByRole("button", { name: /Retro/ })).toBeNull();
    expect(screen.getByText("+1 more")).toBeDefined();
  });

  it("shows every chip when three or fewer share a day", () => {
    renderGrid({
      events: [ev("e1", 13, "A", 9), ev("e2", 13, "B", 10)],
    });

    expect(screen.queryByText(/\+1 more/)).toBeNull();
    expect(screen.getByRole("button", { name: /B/ })).toBeDefined();
  });
});
