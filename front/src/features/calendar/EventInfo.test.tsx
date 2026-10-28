import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter } from "react-router";
import type { Event } from "../../lib/api";
import { formatDateTime, toLocalInput } from "../../lib/format";
import { eventColorDotClasses } from "./eventColor";
import EventInfo from "./EventInfo";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

const TASK = {
  id: "t1",
  title: "Buy keyboard",
  status: "open",
  due_date: null,
  event_id: null,
  note_id: null,
  completed_at: null,
};

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

// Router wrapper for the linked-note chip (react-router Link context).
function info(
  event: Event,
  handlers: { onClose?: () => void; onEdit?: (e: Event) => void } = {},
) {
  return (
    <MemoryRouter initialEntries={["/calendar"]}>
      <EventInfo
        event={event}
        onClose={handlers.onClose ?? (() => {})}
        onEdit={handlers.onEdit ?? (() => {})}
      />
    </MemoryRouter>
  );
}

function fullWhen(iso: string): string {
  return formatDateTime(toLocalInput(iso));
}

describe("EventInfo", () => {
  it("shows the title and a same-day time range", () => {
    const event = makeEvent();
    render(info(event));

    expect(screen.getByRole("heading", { name: "Dentist" })).toBeDefined();
    expect(
      screen.getByText(`${fullWhen(event.starts_at)} – 10:00`),
    ).toBeDefined();
  });

  it("marks the card with the event's hashed color dot", () => {
    render(info(makeEvent()));

    const dot = screen.getByTestId("event-color-dot");
    expect(dot.className).toContain(eventColorDotClasses("e1"));
  });

  it("spells out both dates when the event crosses midnight", () => {
    const event = makeEvent({
      starts_at: new Date(2026, 9, 15, 22, 0).toISOString(),
      ends_at: new Date(2026, 9, 16, 2, 0).toISOString(),
    });
    render(info(event));

    expect(
      screen.getByText(
        `${fullWhen(event.starts_at)} – ${fullWhen(event.ends_at)}`,
      ),
    ).toBeDefined();
  });

  it("shows the description, or a muted fallback when there is none", () => {
    const { rerender } = render(
      info(makeEvent({ description: "Bring insurance card." })),
    );
    expect(screen.getByText("Bring insurance card.")).toBeDefined();

    rerender(info(makeEvent({ description: null })));
    expect(screen.getByText("No description.")).toBeDefined();
  });

  it("links the attached note with the generic label", () => {
    const { rerender } = render(info(makeEvent({ note_id: "n9" })));
    const chip = screen.getByRole("link", { name: "note" });
    expect(chip.getAttribute("href")).toBe("/notes?id=n9");

    rerender(info(makeEvent({ note_id: null })));
    expect(screen.queryByRole("link", { name: "note" })).toBeNull();
  });

  it("hands the event to edit without closing first", () => {
    const onEdit = vi.fn();
    const onClose = vi.fn();
    const event = makeEvent();
    render(info(event, { onClose, onEdit }));

    fireEvent.click(screen.getByRole("button", { name: "Edit" }));
    expect(onEdit).toHaveBeenCalledWith(event);
    expect(onClose).not.toHaveBeenCalled();
  });

  it("closes on Close, Escape, or a backdrop click but not on the card", () => {
    const onClose = vi.fn();
    const { container } = render(info(makeEvent(), { onClose }));

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

  it("never fetches when the description has no mention tokens", () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    render(info(makeEvent({ description: "Bring insurance card." })));
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("resolves a mention token to a live chip deep-linking the module", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ data: [TASK] }));
    vi.stubGlobal("fetch", fetchMock);
    const { container } = render(
      info(makeEvent({ description: "Ping @[task:t1] about it" })),
    );

    const chip = await screen.findByRole("link", { name: "Buy keyboard" });
    expect(chip.getAttribute("href")).toBe("/tasks?id=t1");
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(container.textContent).toContain("about it");
  });

  it("mutes a mention whose id no longer exists, never a raw id link", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(jsonResponse({ data: [] })),
    );
    const { container } = render(
      info(makeEvent({ description: "Ping @[task:ghost] now" })),
    );

    const muted = await screen.findByText("@[task:ghost]");
    expect(muted.className).toContain("text-base-content/40");
    expect(container.querySelectorAll("a")).toHaveLength(0);
    expect(container.textContent).toContain("now");
  });

  it("keeps description tokens as plain text while the titles are loading", () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(() => new Promise(() => {})),
    );
    render(info(makeEvent({ description: "Ping @[task:t1]" })));

    expect(screen.getByText("Ping @[task:t1]")).toBeDefined();
    expect(document.querySelector("a.badge")).toBeNull();
  });
});
