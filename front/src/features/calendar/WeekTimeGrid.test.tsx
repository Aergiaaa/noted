import { act, cleanup, fireEvent, render } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Event } from "../../lib/api";
import { startOfWeek, weekDays } from "./week";
import WeekTimeGrid from "./WeekTimeGrid";

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

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
    title: id === "e1" ? "Dentist" : `event ${id}`,
    starts_at: startsAt,
    ends_at: endsAt,
    description: null,
    note_id: null,
  };
}

// 2026-10-12 (Mon) .. 2026-10-18 (Sun).
const DAYS = weekDays(new Date(2026, 9, 12));

function renderGrid(
  events: Event[],
  onSlotClick = () => {},
  onEventClick = () => {},
) {
  return render(
    <WeekTimeGrid
      days={DAYS}
      events={events}
      onSlotClick={onSlotClick}
      onEventClick={onEventClick}
    />,
  );
}

describe("WeekTimeGrid", () => {
  it("renders day headers, an hour gutter and one slot per hour", () => {
    const { container } = renderGrid([]);
    const slots = container.querySelectorAll(
      "button[aria-label^='Create event on']",
    );
    expect(slots).toHaveLength(7 * 24);
    expect(container.textContent).toContain("Mon 12");
    expect(container.textContent).toContain("Sun 18");
    expect(container.textContent).toContain("09");
  });

  it("renders an event as a block in its day column", () => {
    const e = event(
      "e1",
      localISO(2026, 10, 12, 9),
      localISO(2026, 10, 12, 10),
    );
    const { container } = renderGrid([e]);
    const block = container.querySelector("button[aria-label='Dentist 09:00']");
    expect(block).not.toBeNull();
    expect(block?.getAttribute("style")).toContain("top: 37.5%");
    expect(block?.getAttribute("style")).toContain("height: 4.166");
  });

  it("splits overlapping events into lanes", () => {
    const a = event(
      "e1",
      localISO(2026, 10, 12, 9),
      localISO(2026, 10, 12, 11),
    );
    const b = event(
      "e2",
      localISO(2026, 10, 12, 10),
      localISO(2026, 10, 12, 12),
    );
    const { container } = renderGrid([a, b]);
    const first = container.querySelector("button[aria-label='Dentist 09:00']");
    const second = container.querySelector(
      "button[aria-label='event e2 10:00']",
    );
    expect(first?.getAttribute("style")).toContain("left: 0%");
    expect(second?.getAttribute("style")).toContain("left: 50%");
    expect(second?.getAttribute("style")).toContain("width: calc(50%");
  });

  it("rounds only the outer edges of a multi-day block", () => {
    const e = event(
      "e1",
      localISO(2026, 10, 12, 22),
      localISO(2026, 10, 13, 2),
    );
    const { container } = renderGrid([e]);
    const blocks = container.querySelectorAll(
      "button[aria-label='Dentist 22:00']",
    );
    expect(blocks).toHaveLength(2);
    expect(blocks[0]?.className).toContain("rounded-r-none");
    expect(blocks[0]?.className).not.toContain("rounded-l-none");
    expect(blocks[1]?.className).toContain("rounded-l-none");
    expect(blocks[1]?.className).not.toContain("rounded-r-none");
  });

  it("shows one now-line on today's column", () => {
    const today = new Date();
    const week = weekDays(startOfWeek(today));
    const { container } = render(
      <WeekTimeGrid
        days={week}
        events={[]}
        onSlotClick={() => {}}
        onEventClick={() => {}}
      />,
    );
    expect(container.querySelectorAll("[data-testid='now-line']")).toHaveLength(
      1,
    );
  });

  it("moves the now-line every minute", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 9, 15, 12, 0));
    const { container } = renderGrid([]);
    const line = container.querySelector("[data-testid='now-line']");
    const before = line?.getAttribute("style");

    act(() => {
      vi.advanceTimersByTime(60000);
    });

    const after = container
      .querySelector("[data-testid='now-line']")
      ?.getAttribute("style");
    expect(after).not.toBe(before);
  });

  it("reports the slot clicked with its day and hour", () => {
    const onSlotClick = vi.fn();
    const { container } = renderGrid([], onSlotClick);
    const slot = container.querySelector(
      "button[aria-label='Create event on Wed 14 at 15:00']",
    );
    fireEvent.click(slot as HTMLElement);

    expect(onSlotClick).toHaveBeenCalledTimes(1);
    const [day, hour] = onSlotClick.mock.calls[0] as [Date, number];
    expect(day.getDate()).toBe(14);
    expect(hour).toBe(15);
  });

  it("reports the event clicked", () => {
    const e = event(
      "e1",
      localISO(2026, 10, 13, 9),
      localISO(2026, 10, 13, 10),
    );
    const onEventClick = vi.fn();
    const { container } = renderGrid([e], () => {}, onEventClick);
    fireEvent.click(
      container.querySelector(
        "button[aria-label='Dentist 09:00']",
      ) as HTMLElement,
    );

    expect(onEventClick).toHaveBeenCalledWith(e);
  });

  it("ignores events outside the rendered week", () => {
    const e = event("e1", localISO(2026, 11, 1, 9), localISO(2026, 11, 1, 10));
    const { container } = renderGrid([e]);
    expect(
      container.querySelector("button[aria-label='Dentist 09:00']"),
    ).toBeNull();
  });

  it("offers a create affordance on hours fully covered by a block", () => {
    const e = event(
      "e1",
      localISO(2026, 10, 12, 10),
      localISO(2026, 10, 12, 12),
    );
    const onSlotClick = vi.fn();
    const { container } = renderGrid([e], onSlotClick);

    const affordances = container.querySelectorAll(
      "button[aria-label^='Add event on']",
    );
    expect(affordances).toHaveLength(2);

    fireEvent.click(
      container.querySelector(
        "button[aria-label='Add event on Mon 12 at 11:00']",
      ) as HTMLElement,
    );
    expect(onSlotClick).toHaveBeenCalledTimes(1);
    const [day, hour] = onSlotClick.mock.calls[0] as [Date, number];
    expect(day.getDate()).toBe(12);
    expect(hour).toBe(11);
  });

  it("skips the affordance when lanes leave the slot half free", () => {
    const a = event(
      "e1",
      localISO(2026, 10, 12, 9),
      localISO(2026, 10, 12, 11),
    );
    const b = event(
      "e2",
      localISO(2026, 10, 12, 10),
      localISO(2026, 10, 12, 12),
    );
    const { container } = renderGrid([a, b]);
    expect(
      container.querySelectorAll("button[aria-label^='Add event on']"),
    ).toHaveLength(0);
  });

  it("skips the affordance when a block covers only part of the hour", () => {
    const e = event(
      "e1",
      localISO(2026, 10, 12, 9, 15),
      localISO(2026, 10, 12, 9, 45),
    );
    const { container } = renderGrid([e]);
    expect(
      container.querySelectorAll("button[aria-label^='Add event on']"),
    ).toHaveLength(0);
  });
});
