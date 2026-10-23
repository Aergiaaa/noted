import { afterEach, describe, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import WalletsModal from "./WalletsModal";
import type { Wallet } from "../../lib/api";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

const wallets: Wallet[] = [
  { id: "wal_cash", name: "Cash", archived_at: null },
  { id: "wal_bank", name: "Bank", archived_at: "2026-10-01T00:00:00Z" },
];

function mockFetch(
  impl: (url: string, init: RequestInit) => Promise<Response>,
): ReturnType<typeof vi.fn> {
  const spy = vi.fn(impl as never);
  vi.stubGlobal("fetch", spy);
  return spy;
}

function jsonResponse(data: unknown, status = 200): Response {
  return new Response(JSON.stringify(data), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("WalletsModal", () => {
  it("lists wallets and badges the archived ones", () => {
    render(
      <WalletsModal wallets={wallets} onClose={vi.fn()} onChanged={vi.fn()} />,
    );

    expect(screen.getByText("Cash")).toBeDefined();
    expect(screen.getByText("Bank")).toBeDefined();
    expect(screen.getByText("archived")).toBeDefined();
    expect(screen.getByRole("button", { name: "Archive" })).toBeDefined();
    expect(screen.getByRole("button", { name: "Unarchive" })).toBeDefined();
  });

  it("creates a wallet and asks the page to refetch", async () => {
    const onChanged = vi.fn();
    const fetchSpy = mockFetch(async () => jsonResponse({}, 201));
    render(
      <WalletsModal
        wallets={wallets}
        onClose={vi.fn()}
        onChanged={onChanged}
      />,
    );

    fireEvent.change(screen.getByLabelText("New wallet name"), {
      target: { value: "E-wallet" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Add" }));

    await waitFor(() => expect(onChanged).toHaveBeenCalledTimes(1));
    expect(fetchSpy).toHaveBeenCalledWith(
      "/api/finance/wallets",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ name: "E-wallet" }),
      }),
    );
    const input = screen.getByLabelText("New wallet name") as HTMLInputElement;
    expect(input.value).toBe("");
  });

  it("renames a wallet inline, cancel keeps it untouched", async () => {
    const onChanged = vi.fn();
    const fetchSpy = mockFetch(async () => jsonResponse({}));
    render(
      <WalletsModal
        wallets={wallets}
        onClose={vi.fn()}
        onChanged={onChanged}
      />,
    );

    fireEvent.click(screen.getAllByRole("button", { name: "Rename" })[0]);
    const input = screen.getByDisplayValue("Cash");
    fireEvent.change(input, { target: { value: "Petty cash" } });
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(fetchSpy).not.toHaveBeenCalled();

    fireEvent.click(screen.getAllByRole("button", { name: "Rename" })[0]);
    fireEvent.change(screen.getByDisplayValue("Cash"), {
      target: { value: "Petty cash" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(onChanged).toHaveBeenCalledTimes(1));
    expect(fetchSpy).toHaveBeenCalledWith(
      "/api/finance/wallets/wal_cash",
      expect.objectContaining({
        method: "PATCH",
        body: JSON.stringify({ name: "Petty cash" }),
      }),
    );
  });

  it("archives and unarchives a wallet", async () => {
    const onChanged = vi.fn();
    const fetchSpy = mockFetch(async () => jsonResponse({}));
    render(
      <WalletsModal
        wallets={wallets}
        onClose={vi.fn()}
        onChanged={onChanged}
      />,
    );

    fireEvent.click(screen.getAllByRole("button", { name: "Archive" })[0]);
    fireEvent.click(screen.getByRole("button", { name: "Unarchive" }));

    await waitFor(() => expect(onChanged).toHaveBeenCalledTimes(2));
    expect(fetchSpy).toHaveBeenNthCalledWith(
      1,
      "/api/finance/wallets/wal_cash",
      expect.objectContaining({
        method: "PATCH",
        body: JSON.stringify({ archived: true }),
      }),
    );
    expect(fetchSpy).toHaveBeenNthCalledWith(
      2,
      "/api/finance/wallets/wal_bank",
      expect.objectContaining({
        method: "PATCH",
        body: JSON.stringify({ archived: false }),
      }),
    );
  });

  it("shows the server errors when a rename or archive fails", async () => {
    let patches = 0;
    mockFetch(async () => {
      patches += 1;
      return jsonResponse(
        {
          error: {
            code: "validation_error",
            message: patches === 1 ? "bad rename" : "bad archive",
          },
        },
        400,
      );
    });
    render(
      <WalletsModal wallets={wallets} onClose={vi.fn()} onChanged={vi.fn()} />,
    );

    fireEvent.click(screen.getAllByRole("button", { name: "Rename" })[0]);
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() =>
      expect(screen.getByRole("alert").textContent).toBe("bad rename"),
    );

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    fireEvent.click(screen.getAllByRole("button", { name: "Archive" })[0]);
    await waitFor(() =>
      expect(screen.getByRole("alert").textContent).toBe("bad archive"),
    );
  });

  it("shows the server error when a create fails", async () => {
    mockFetch(async () =>
      jsonResponse(
        { error: { code: "validation_error", message: "bad" } },
        400,
      ),
    );
    render(
      <WalletsModal wallets={wallets} onClose={vi.fn()} onChanged={vi.fn()} />,
    );

    fireEvent.change(screen.getByLabelText("New wallet name"), {
      target: { value: "x".repeat(80) },
    });
    fireEvent.click(screen.getByRole("button", { name: "Add" }));

    await waitFor(() =>
      expect(screen.getByRole("alert").textContent).toBe("bad"),
    );
  });
});
