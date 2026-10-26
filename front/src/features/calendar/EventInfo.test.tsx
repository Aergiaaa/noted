import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Event } from "../../lib/api";
import { formatDateTime, toLocalInput } from "../../lib/format";
import { eventColorDotClasses } from "./eventColor";
import EventInfo from "./EventInfo";

afterEach(() => cleanup());

function makeEvent(overrides: Partial<Event> = {}): Event {
  return {
    id: "e1",
    title: "Dentist",
    starts_at: new Date(2026, 9, 15, 9, 0).toISOString(),
    ends_at: new Date(2026, 9, 15, 10, 0).toISOString(),
    description: null,
    note_id: null,
    ...overrides,
  };
}

function fullWhen(iso: string): string {
  return formatDateTime(toLocalInput(iso));
}

describe("EventInfo", () => {
  it("shows the title and a same-day time range", () => {
    const event = makeEvent();
    render(<EventInfo event={event} onClose={() => {}} onEdit={() => {}} />);

    expect(screen.getByRole("heading", { name: "Dentist" })).toBeDefined();
    expect(
      screen.getByText(`${fullWhen(event.starts_at)} – 10:00`),
    ).toBeDefined();
  });

  it("marks the card with the event's hashed color dot", () => {
    render(
      <EventInfo event={makeEvent()} onClose={() => {}} onEdit={() => {}} />,
    );

    const dot = screen.getByTestId("event-color-dot");
    expect(dot.className).toContain(eventColorDotClasses("e1"));
  });

  it("spells out both dates when the event crosses midnight", () => {
    const event = makeEvent({
      starts_at: new Date(2026, 9, 15, 22, 0).toISOString(),
      ends_at: new Date(2026, 9, 16, 2, 0).toISOString(),
    });
    render(<EventInfo event={event} onClose={() => {}} onEdit={() => {}} />);

    expect(
      screen.getByText(
        `${fullWhen(event.starts_at)} – ${fullWhen(event.ends_at)}`,
      ),
    ).toBeDefined();
  });

  it("shows the description, or a muted fallback when there is none", () => {
    const { rerender } = render(
      <EventInfo
        event={makeEvent({ description: "Bring insurance card." })}
        onClose={() => {}}
        onEdit={() => {}}
      />,
    );
    expect(screen.getByText("Bring insurance card.")).toBeDefined();

    rerender(
      <EventInfo
        event={makeEvent({ description: null })}
        onClose={() => {}}
        onEdit={() => {}}
      />,
    );
    expect(screen.getByText("No description.")).toBeDefined();
  });

  it("hands the event to edit without closing first", () => {
    const onEdit = vi.fn();
    const onClose = vi.fn();
    const event = makeEvent();
    render(<EventInfo event={event} onClose={onClose} onEdit={onEdit} />);

    fireEvent.click(screen.getByRole("button", { name: "Edit" }));
    expect(onEdit).toHaveBeenCalledWith(event);
    expect(onClose).not.toHaveBeenCalled();
  });

  it("closes on Close, Escape, or a backdrop click but not on the card", () => {
    const onClose = vi.fn();
    const { container } = render(
      <EventInfo event={makeEvent()} onClose={onClose} onEdit={() => {}} />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(onClose).toHaveBeenCalledTimes(1);

    fireEvent.keyDown(window, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(2);

    fireEvent.keyDown(window, { key: "Enter" });
    expect(onClose).toHaveBeenCalledTimes(2);

    fireEvent.click(container.querySelector(".modal") as HTMLElement);
    expect(onClose).toHaveBeenCalledTimes(3);

    fireEvent.click(container.querySelector(".modal-box") as HTMLElement);
    expect(onClose).toHaveBeenCalledTimes(3);
  });
});
