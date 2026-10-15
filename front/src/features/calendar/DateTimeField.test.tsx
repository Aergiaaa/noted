import { cleanup, fireEvent, render, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import DateTimeField from "./DateTimeField";

afterEach(() => {
  cleanup();
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

function pickerHost(container: HTMLElement): HTMLElement {
  return container.querySelector("calendar-date") as HTMLElement;
}

describe("DateTimeField", () => {
  it("shows the formatted datetime on a closed trigger", () => {
    const { container, getByLabelText } = render(
      <DateTimeField label="Starts" value={VALUE} onChange={() => {}} />,
    );
    const trigger = getByLabelText("Starts");
    expect(trigger.getAttribute("aria-expanded")).toBe("false");
    expect(trigger.textContent).toBe("Thu 15 Oct 2026, 14:37");
    expect(container.querySelector("calendar-date")).toBeNull();
  });

  it("opens and closes the picker via the trigger", () => {
    const { container, getByLabelText } = render(
      <DateTimeField label="Starts" value={VALUE} onChange={() => {}} />,
    );
    const trigger = getByLabelText("Starts");

    fireEvent.click(trigger);
    expect(trigger.getAttribute("aria-expanded")).toBe("true");
    expect(container.querySelector("calendar-date")).toBeTruthy();

    fireEvent.click(trigger);
    expect(trigger.getAttribute("aria-expanded")).toBe("false");
    expect(container.querySelector("calendar-date")).toBeNull();
  });

  it("closes when the backdrop is clicked", () => {
    const { container, getByLabelText, getByTestId } = render(
      <DateTimeField label="Starts" value={VALUE} onChange={() => {}} />,
    );
    fireEvent.click(getByLabelText("Starts"));
    expect(container.querySelector("calendar-date")).toBeTruthy();

    fireEvent.click(getByTestId("picker-backdrop"));
    expect(container.querySelector("calendar-date")).toBeNull();
  });

  it("picking a day keeps hour and minute", async () => {
    const onChange = vi.fn();
    const { container, getByLabelText } = render(
      <DateTimeField label="Starts" value={VALUE} onChange={onChange} />,
    );
    fireEvent.click(getByLabelText("Starts"));
    const day20 = await dayButton(pickerHost(container), "20");
    fireEvent.click(day20 as HTMLElement);

    expect(onChange).toHaveBeenCalledWith("2026-10-20T14:37");
  });

  it("picking an hour keeps date and minute", () => {
    const onChange = vi.fn();
    const { getByLabelText } = render(
      <DateTimeField label="Starts" value={VALUE} onChange={onChange} />,
    );
    fireEvent.click(getByLabelText("Starts"));

    fireEvent.change(getByLabelText("Starts hour"), {
      target: { value: "09" },
    });
    expect(onChange).toHaveBeenCalledWith("2026-10-15T09:37");
  });

  it("picking a minute keeps date and hour", () => {
    const onChange = vi.fn();
    const { getByLabelText } = render(
      <DateTimeField label="Starts" value={VALUE} onChange={onChange} />,
    );
    fireEvent.click(getByLabelText("Starts"));

    fireEvent.change(getByLabelText("Starts minute"), {
      target: { value: "05" },
    });
    expect(onChange).toHaveBeenCalledWith("2026-10-15T14:05");
  });

  it("preselects the current hour and minute in the selects", () => {
    const { getByLabelText } = render(
      <DateTimeField label="Ends" value={VALUE} onChange={() => {}} />,
    );
    fireEvent.click(getByLabelText("Ends"));

    expect((getByLabelText("Ends hour") as HTMLSelectElement).value).toBe("14");
    expect((getByLabelText("Ends minute") as HTMLSelectElement).value).toBe(
      "37",
    );
  });
});
