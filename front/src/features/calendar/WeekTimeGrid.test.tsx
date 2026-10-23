import { act, cleanup, fireEvent, render } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Event } from "../../lib/api";
import { startOfWeek, weekDays } from "./week";
import { eventColorClasses } from "./eventColor";
import WeekTimeGrid from "./WeekTimeGrid";

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.restoreAllMocks();
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

function classNameTokens(el: Element | null | undefined): string[] {
  return (el?.getAttribute("class") ?? "").split(/\s+/);
}

// jsdom has no layout: stub the row's box as 24 x 60px hour columns so
// mousemove clientX maps to hours (1440 minutes wide, one px per minute).
function stubDayRow(container: HTMLElement, index = 0): HTMLElement {
  const row = container.querySelectorAll("[data-testid='day-row']")[index];
  vi.spyOn(row as HTMLElement, "getBoundingClientRect").mockReturnValue({
    left: 0,
    top: 0,
    right: 1440,
    bottom: 64,
    width: 1440,
    height: 64,
    x: 0,
    y: 0,
    toJSON: () => ({}),
  } as DOMRect);
  return row as HTMLElement;
}

describe("WeekTimeGrid", () => {
  it("renders a sticky time axis over day rows with one slot per hour", () => {
    const { container } = renderGrid([]);
    const slots = container.querySelectorAll(
      "button[aria-label^='Create event on']",
    );
    expect(slots).toHaveLength(7 * 24);
    expect(container.textContent).toContain("Mon 12");
    expect(container.textContent).toContain("Sun 18");
    expect(container.textContent).toContain("09");
    // The axis must paint above the grid when it scrolls (the old z bug),
    // and day labels above the blocks scrolled underneath them.
    expect(
      classNameTokens(container.querySelector("[data-testid='time-axis']")),
    ).toContain("z-40");
    expect(
      classNameTokens(container.querySelector("[data-testid='day-label']")),
    ).toContain("z-30");
  });

  it("renders an event as a block across time in its day row", () => {
    const e = event(
      "e1",
      localISO(2026, 10, 12, 9),
      localISO(2026, 10, 12, 10),
    );
    const { container } = renderGrid([e]);
    const block = container.querySelector("button[aria-label='Dentist 09:00']");
    expect(block).not.toBeNull();
    expect(block?.getAttribute("style")).toContain("left: 37.5%");
    expect(block?.getAttribute("style")).toContain("width: calc(4.166");
  });

  it("colors blocks and continuation strips from the event id hash", () => {
    const e1 = event(
      "e1",
      localISO(2026, 10, 12, 9),
      localISO(2026, 10, 12, 10),
    );
    const through = event(
      "e2",
      localISO(2026, 10, 12, 10),
      localISO(2026, 10, 14, 10),
    );
    const { container } = renderGrid([e1, through]);

    const block = container.querySelector("button[aria-label='Dentist 09:00']");
    for (const cls of eventColorClasses("e1").split(" ")) {
      expect(classNameTokens(block)).toContain(cls);
    }

    const strip = container.querySelector("[data-testid='continuation-strip']");
    for (const cls of eventColorClasses("e2").split(" ")) {
      expect(classNameTokens(strip)).toContain(cls);
    }
  });

  it("stacks overlapping events into lanes", () => {
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
    expect(first?.getAttribute("style")).toContain("top: 0%");
    expect(second?.getAttribute("style")).toContain("top: 50%");
    expect(second?.getAttribute("style")).toContain("height: calc(50%");
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

  it("renders a middle day of a multi-day event as a top continuation strip", () => {
    const e = event(
      "e1",
      localISO(2026, 10, 12, 10),
      localISO(2026, 10, 14, 16),
    );
    const onEventClick = vi.fn();
    const { container } = renderGrid([e], () => {}, onEventClick);

    // Only Tuesday (the covered middle day) gets a band; Mon/Wed render
    // the real start/end blocks with no band above them.
    const bands = container.querySelectorAll(
      "[data-testid='continuation-band']",
    );
    expect(bands).toHaveLength(1);
    const strips = bands[0]?.querySelectorAll(
      "[data-testid='continuation-strip']",
    );
    expect(strips).toHaveLength(1);
    const strip = strips?.[0] as HTMLElement;
    expect(strip.getAttribute("aria-label")).toBe("Dentist continues");
    // Full width — no time-positioned left/width — and above the time grid.
    expect(strip.getAttribute("style")).toBeNull();
    expect(
      strip.parentElement?.nextElementSibling?.getAttribute("data-testid"),
    ).toBe("day-row");

    fireEvent.click(strip);
    expect(onEventClick).toHaveBeenCalledWith(e);

    // Start and end days still render their clipped time blocks (the
    // label carries the event's real start time on both edges).
    expect(
      container.querySelectorAll("button[aria-label='Dentist 10:00']"),
    ).toHaveLength(2);
  });

  it("stacks several continuing events in one band", () => {
    const trip = event(
      "e1",
      localISO(2026, 10, 12, 10),
      localISO(2026, 10, 14, 16),
    );
    const conference = event(
      "e2",
      localISO(2026, 10, 11, 8),
      localISO(2026, 10, 15, 18),
    );
    const { container } = renderGrid([trip, conference]);
    const bands = container.querySelectorAll(
      "[data-testid='continuation-band']",
    );
    // Conference starts Sun (before the rendered week), so Monday is
    // already a continuation day for it; Tue holds both; Wed only the
    // conference (trip ends Wed 16:00).
    expect(bands).toHaveLength(3);
    expect(
      bands[0]?.querySelectorAll("[data-testid='continuation-strip']"),
    ).toHaveLength(1);
    expect(
      bands[1]?.querySelectorAll("[data-testid='continuation-strip']"),
    ).toHaveLength(2);
    expect(
      bands[2]?.querySelectorAll("[data-testid='continuation-strip']"),
    ).toHaveLength(1);
  });

  it("does not cover middle-day hours for a passing-through event", () => {
    const e = event(
      "e1",
      localISO(2026, 10, 12, 10),
      localISO(2026, 10, 14, 10),
    );
    const { container } = renderGrid([e]);
    // Tuesday holds no placed block — the strip lives in the band, so no
    // hour is covered and no create-affordance appears there.
    expect(
      container.querySelectorAll("button[aria-label^='Add event on Tue']"),
    ).toHaveLength(0);
  });

  it("shows one now-line on today's row", () => {
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
    // Hidden until the pointer sits on that specific hour cell.
    expect(classNameTokens(affordances[0])).toContain("opacity-0");
    expect(classNameTokens(affordances[0])).not.toContain("opacity-100");

    const row = stubDayRow(container);
    fireEvent.mouseMove(row, { clientX: 11 * 60 + 30 });
    expect(
      classNameTokens(
        container.querySelector(
          "button[aria-label='Add event on Mon 12 at 11:00']",
        ),
      ),
    ).toContain("opacity-100");
    expect(
      classNameTokens(
        container.querySelector(
          "button[aria-label='Add event on Mon 12 at 10:00']",
        ),
      ),
    ).not.toContain("opacity-100");

    fireEvent.click(
      container.querySelector(
        "button[aria-label='Add event on Mon 12 at 11:00']",
      ) as HTMLElement,
    );
    expect(onSlotClick).toHaveBeenCalledTimes(1);
    const [day, hour] = onSlotClick.mock.calls[0] as [Date, number];
    expect(day.getDate()).toBe(12);
    expect(hour).toBe(11);

    fireEvent.mouseLeave(row);
    expect(
      classNameTokens(
        container.querySelector(
          "button[aria-label='Add event on Mon 12 at 11:00']",
        ),
      ),
    ).not.toContain("opacity-100");
  });

  it("reveals a covered affordance only in the hovered day's column", () => {
    const mon = event(
      "e1",
      localISO(2026, 10, 12, 10),
      localISO(2026, 10, 12, 12),
    );
    const tue = event(
      "e2",
      localISO(2026, 10, 13, 10),
      localISO(2026, 10, 13, 12),
    );
    const { container } = renderGrid([mon, tue]);
    const monday = stubDayRow(container, 0);
    const tuesday = stubDayRow(container, 1);

    fireEvent.mouseMove(monday, { clientX: 11 * 60 + 30 });
    expect(
      classNameTokens(
        container.querySelector(
          "button[aria-label='Add event on Mon 12 at 11:00']",
        ),
      ),
    ).toContain("opacity-100");
    // Same hour, other column: the dayIndex half of hoverCell must hold it shut.
    expect(
      classNameTokens(
        container.querySelector(
          "button[aria-label='Add event on Tue 13 at 11:00']",
        ),
      ),
    ).not.toContain("opacity-100");

    fireEvent.mouseMove(tuesday, { clientX: 11 * 60 + 30 });
    expect(
      classNameTokens(
        container.querySelector(
          "button[aria-label='Add event on Tue 13 at 11:00']",
        ),
      ),
    ).toContain("opacity-100");
    expect(
      classNameTokens(
        container.querySelector(
          "button[aria-label='Add event on Mon 12 at 11:00']",
        ),
      ),
    ).not.toContain("opacity-100");
  });

  it("stretches rows and slots to fill the available height", () => {
    const { container } = renderGrid([]);
    // Rows and the grid root flex into the scroller's height; slots fill
    // the row so blocks scale with it. min-h keeps the week scrollable on
    // short viewports instead of squashing. The row lives in a column
    // beside the day label so a continuation band can sit above it.
    const row = container.querySelector("[data-testid='day-row']");
    expect(classNameTokens(row?.parentElement)).toContain("flex-1");
    expect(classNameTokens(row?.parentElement?.parentElement)).toContain(
      "min-h-16",
    );
    const slot = container.querySelector(
      "button[aria-label='Create event on Mon 12 at 09:00']",
    );
    expect(classNameTokens(slot)).toContain("h-full");
    expect(classNameTokens(slot)).toContain("flex-1");
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
