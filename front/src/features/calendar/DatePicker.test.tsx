import { cleanup, fireEvent, render, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import DatePicker from "./DatePicker";

afterEach(() => {
  cleanup();
  setViewport(1024, 768);
});

const VALUE = "2026-10-15T14:37";

// Cally renders into shadow roots after an async atomico microtask.
async function dayButton(host: HTMLElement, day: string) {
  await waitFor(() =>
    expect(
      host.shadowRoot?.querySelectorAll("button").length ?? 0,
    ).toBeGreaterThan(0),
  );
  const month = host.querySelector("calendar-month") as HTMLElement;
  return Array.from(month.shadowRoot?.querySelectorAll("button") ?? []).find(
    (b) => b.textContent === day,
  );
}

function pickerHost(): HTMLElement {
  return document.querySelector(
    '[data-testid="picker-panel"] calendar-date',
  ) as HTMLElement;
}

function stubRect(trigger: HTMLElement, rect: Partial<DOMRect>) {
  trigger.getBoundingClientRect = () =>
    ({
      top: 0,
      left: 0,
      right: 0,
      bottom: 0,
      width: 0,
      height: 0,
      x: 0,
      y: 0,
      toJSON: () => ({}),
      ...rect,
    }) as DOMRect;
}

function setViewport(width: number, height: number) {
  Object.defineProperty(window, "innerWidth", { value: width });
  Object.defineProperty(window, "innerHeight", { value: height });
}

describe("DatePicker", () => {
  it("shows the formatted date on a closed trigger", () => {
    const { getByLabelText } = render(
      <DatePicker label="Starts" value={VALUE} onChange={() => {}} />,
    );
    const trigger = getByLabelText("Starts");
    expect(trigger.getAttribute("aria-expanded")).toBe("false");
    expect(trigger.textContent).toBe("Thu 15 Oct 2026");
    expect(document.querySelector("calendar-date")).toBeNull();
  });

  it("opens and closes the picker via the trigger", () => {
    const { getByLabelText } = render(
      <DatePicker label="Starts" value={VALUE} onChange={() => {}} />,
    );
    const trigger = getByLabelText("Starts");

    fireEvent.click(trigger);
    expect(trigger.getAttribute("aria-expanded")).toBe("true");
    expect(document.querySelector("calendar-date")).toBeTruthy();

    fireEvent.click(trigger);
    expect(trigger.getAttribute("aria-expanded")).toBe("false");
    expect(document.querySelector("calendar-date")).toBeNull();
  });

  it("closes the panel on Escape without the event reaching window", () => {
    const { getByLabelText } = render(
      <DatePicker label="Starts" value={VALUE} onChange={() => {}} />,
    );
    const trigger = getByLabelText("Starts");

    // Escape while closed: no-op.
    fireEvent.keyDown(trigger, { key: "Escape" });
    expect(trigger.getAttribute("aria-expanded")).toBe("false");

    fireEvent.click(trigger);
    expect(document.querySelector("calendar-date")).toBeTruthy();

    // Modal listens for Esc on window; an Escape leak would close the form.
    const windowEscapes: KeyboardEvent[] = [];
    const onWindowKey = (e: Event) => {
      if ((e as KeyboardEvent).key === "Escape")
        windowEscapes.push(e as KeyboardEvent);
    };
    window.addEventListener("keydown", onWindowKey);

    // Other keys never close the panel.
    fireEvent.keyDown(trigger, { key: "Enter" });
    expect(trigger.getAttribute("aria-expanded")).toBe("true");

    fireEvent.keyDown(trigger, { key: "Escape" });
    window.removeEventListener("keydown", onWindowKey);

    expect(trigger.getAttribute("aria-expanded")).toBe("false");
    expect(document.querySelector("calendar-date")).toBeNull();
    expect(windowEscapes).toEqual([]);
  });

  it("renders the open picker in a portal on body, outside the field", () => {
    const { container, getByLabelText, getByTestId } = render(
      <DatePicker label="Starts" value={VALUE} onChange={() => {}} />,
    );
    fireEvent.click(getByLabelText("Starts"));

    const panel = getByTestId("picker-panel");
    expect(panel.parentElement).toBe(document.body);
    expect(container.querySelector('[data-testid="picker-panel"]')).toBeNull();
    expect(getByTestId("picker-backdrop").parentElement).toBe(document.body);
  });

  it("anchors the panel to the right of the trigger, outside the card", () => {
    const { getByLabelText, getByTestId } = render(
      <DatePicker label="Starts" value={VALUE} onChange={() => {}} />,
    );
    const trigger = getByLabelText("Starts");
    stubRect(trigger, { top: 120, left: 300, right: 580 });

    fireEvent.click(trigger);
    const panel = getByTestId("picker-panel");
    // trigger.right + modal padding (24) + gap (12) = card edge + 12.
    expect(panel.style.left).toBe("616px");
    expect(panel.style.top).toBe("120px");
  });

  it("flips the panel to the left when the right side has no room", () => {
    setViewport(500, 768);
    const { getByLabelText, getByTestId } = render(
      <DatePicker label="Starts" value={VALUE} onChange={() => {}} />,
    );
    const trigger = getByLabelText("Starts");
    stubRect(trigger, { top: 50, left: 400, right: 650 });

    fireEvent.click(trigger);
    // 400 - 24 - 12 - 288 = 76.
    expect(getByTestId("picker-panel").style.left).toBe("76px");
  });

  it("clamps a left flip to the viewport edge", () => {
    setViewport(500, 768);
    const { getByLabelText, getByTestId } = render(
      <DatePicker label="Starts" value={VALUE} onChange={() => {}} />,
    );
    const trigger = getByLabelText("Starts");
    stubRect(trigger, { top: 50, left: 100, right: 300 });

    fireEvent.click(trigger);
    expect(getByTestId("picker-panel").style.left).toBe("8px");
  });

  it("clamps the panel up when it would hang off the bottom", () => {
    setViewport(1024, 768);
    const { getByLabelText, getByTestId } = render(
      <DatePicker label="Starts" value={VALUE} onChange={() => {}} />,
    );
    const trigger = getByLabelText("Starts");
    stubRect(trigger, { top: 700, left: 300, right: 580 });

    fireEvent.click(trigger);
    // viewport 768 - panel height (280) - margin 8 = 480.
    expect(getByTestId("picker-panel").style.top).toBe("480px");
  });

  it("keeps the panel inside short viewports", () => {
    setViewport(1024, 250);
    const { getByLabelText, getByTestId } = render(
      <DatePicker label="Starts" value={VALUE} onChange={() => {}} />,
    );
    const trigger = getByLabelText("Starts");
    stubRect(trigger, { top: 100, left: 300, right: 580 });

    fireEvent.click(trigger);
    expect(getByTestId("picker-panel").style.top).toBe("8px");
  });

  it("closes when the backdrop is clicked", () => {
    const { getByLabelText, getByTestId } = render(
      <DatePicker label="Starts" value={VALUE} onChange={() => {}} />,
    );
    fireEvent.click(getByLabelText("Starts"));
    expect(document.querySelector("calendar-date")).toBeTruthy();

    fireEvent.click(getByTestId("picker-backdrop"));
    expect(document.querySelector("calendar-date")).toBeNull();
  });

  it("picking a day keeps the time part", async () => {
    const onChange = vi.fn();
    const { getByLabelText } = render(
      <DatePicker label="Starts" value={VALUE} onChange={onChange} />,
    );
    fireEvent.click(getByLabelText("Starts"));
    const day20 = await dayButton(pickerHost(), "20");
    fireEvent.click(day20 as HTMLElement);

    expect(onChange).toHaveBeenCalledWith("2026-10-20T14:37");
  });
});
