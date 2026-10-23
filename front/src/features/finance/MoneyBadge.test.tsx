import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import MoneyBadge from "./MoneyBadge";

afterEach(cleanup);

// Intl separates symbol and amount with an NBSP (see format.test.ts).
function text(el: HTMLElement) {
  return (el.textContent ?? "").replace(/\u00a0/g, " ");
}

describe("MoneyBadge", () => {
  it("renders a compact positive amount with an explicit plus sign", () => {
    render(<MoneyBadge cents={50000000} currency="IDR" sign="+" compact />);
    const amount = screen.getByText("+Rp 500K");
    expect(amount.className).toContain("text-success");
    expect(amount.getAttribute("title")).toMatch(/500[,.]000/);
  });

  it("renders a compact expense with a minus sign in the error color", () => {
    render(<MoneyBadge cents={50000000} currency="IDR" sign="−" compact />);
    const amount = screen.getByText("−Rp 500K");
    expect(amount.className).toContain("text-error");
    expect(amount.getAttribute("title")).toMatch(/500[,.]000/);
  });

  it("signs negative balances themselves in the error color", () => {
    render(<MoneyBadge cents={-15000000} currency="IDR" compact />);
    const amount = screen.getByText("-Rp 150K");
    expect(amount.className).toContain("text-error");
    expect(amount.getAttribute("title")).toMatch(/150[,.]000/);
  });

  it("renders full precision without a tooltip when not compact", () => {
    render(<MoneyBadge cents={5055} currency="USD" />);
    expect(text(screen.getByText("$50.55"))).toBe("$50.55");
    expect(screen.getByText("$50.55").getAttribute("title")).toBeNull();
  });

  it("keeps transfers unsigned", () => {
    render(<MoneyBadge cents={12345678} currency="IDR" compact />);
    const amount = screen.getByText("Rp 123K");
    expect(amount.className).not.toContain("text-error");
    expect(amount.className).not.toContain("text-success");
  });
});
