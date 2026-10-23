import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import WalletStrip from "./WalletStrip";
import type { WalletBalance } from "../../lib/api";

afterEach(cleanup);

function wallet(over: Partial<WalletBalance> = {}): WalletBalance {
  return {
    wallet_id: "wal_cash",
    name: "Cash",
    balance_cents: 250000,
    ...over,
  };
}

describe("WalletStrip", () => {
  it("renders one compact card per wallet with its balance", () => {
    render(
      <WalletStrip
        wallets={[
          wallet(),
          wallet({
            wallet_id: "wal_bank",
            name: "Bank",
            balance_cents: -1000000,
          }),
        ]}
        currency="IDR"
        activeWalletId={null}
        onSelect={vi.fn()}
        onManage={vi.fn()}
      />,
    );

    expect(screen.getByText("Cash")).toBeDefined();
    expect(screen.getByText("Bank")).toBeDefined();
    expect(screen.getByText("Rp 2.5K")).toBeDefined();
    expect(screen.getByText("-Rp 10K")).toBeDefined();
  });

  it("selects a wallet on click and clears the filter when it is active", () => {
    const onSelect = vi.fn();
    const { rerender } = render(
      <WalletStrip
        wallets={[wallet()]}
        currency="IDR"
        activeWalletId={null}
        onSelect={onSelect}
        onManage={vi.fn()}
      />,
    );

    const card = screen.getByRole("button", { name: /Cash/ });
    card.click();
    expect(onSelect).toHaveBeenCalledWith("wal_cash");

    rerender(
      <WalletStrip
        wallets={[wallet()]}
        currency="IDR"
        activeWalletId="wal_cash"
        onSelect={onSelect}
        onManage={vi.fn()}
      />,
    );
    screen.getByRole("button", { name: /Cash/ }).click();
    expect(onSelect).toHaveBeenLastCalledWith(null);
  });

  it("marks the active wallet", () => {
    render(
      <WalletStrip
        wallets={[wallet()]}
        currency="IDR"
        activeWalletId="wal_cash"
        onSelect={vi.fn()}
        onManage={vi.fn()}
      />,
    );

    const card = screen.getByRole("button", { name: /Cash/ });
    expect(card.getAttribute("aria-pressed")).toBe("true");
    expect(card.className).toContain("border-primary");
  });

  it("opens the manage modal from the ghost action", () => {
    const onManage = vi.fn();
    render(
      <WalletStrip
        wallets={[wallet()]}
        currency="IDR"
        activeWalletId={null}
        onSelect={vi.fn()}
        onManage={onManage}
      />,
    );

    screen.getByRole("button", { name: "Manage wallets" }).click();
    expect(onManage).toHaveBeenCalledTimes(1);
  });
});
