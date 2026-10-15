import { cleanup, fireEvent, render, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import MiniMonth from "./MiniMonth";

afterEach(() => {
  cleanup();
});

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

describe("MiniMonth", () => {
  it("renders day buttons inside cally's shadow DOM", async () => {
    const { container } = render(
      <MiniMonth selected={new Date(2026, 9, 15)} onSelect={() => {}} />,
    );
    const host = container.querySelector("calendar-date") as HTMLElement;
    expect(host.getAttribute("class")).toContain("cally");
    expect(await dayButton(host, "15")).toBeTruthy();
  });

  it("selecting a day calls onSelect with that local day", async () => {
    const onSelect = vi.fn();
    const { container } = render(
      <MiniMonth selected={new Date(2026, 9, 15)} onSelect={onSelect} />,
    );
    const host = container.querySelector("calendar-date") as HTMLElement;
    const day20 = await dayButton(host, "20");
    fireEvent.click(day20 as HTMLElement);

    expect(onSelect).toHaveBeenCalledTimes(1);
    const picked = onSelect.mock.calls[0]?.[0] as Date;
    expect(picked.getFullYear()).toBe(2026);
    expect(picked.getMonth()).toBe(9);
    expect(picked.getDate()).toBe(20);
  });

  it("pages the mini calendar to the selected month", async () => {
    const { container, rerender } = render(
      <MiniMonth selected={new Date(2026, 9, 15)} onSelect={() => {}} />,
    );
    const host = container.querySelector("calendar-date") as HTMLElement;
    await dayButton(host, "15");

    rerender(
      <MiniMonth selected={new Date(2026, 10, 5)} onSelect={() => {}} />,
    );
    await waitFor(() => {
      const month = host.querySelector("calendar-month") as HTMLElement;
      const heading = month.shadowRoot?.querySelector("#h");
      expect(heading?.textContent).toContain("November");
    });
  });
});
