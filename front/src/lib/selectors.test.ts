import { describe, expect, it } from "vitest";
import type { Task } from "./api";
import {
  getOverdueTasks,
  pickDefaultWallet,
  pickTransferTo,
} from "./selectors";

function task(over: Partial<Task> = {}): Task {
  return {
    id: "t1",
    title: "Task",
    status: "open",
    due_date: null,
    event_id: null,
    note_id: null,
    completed_at: null,
    ...over,
  };
}

describe("getOverdueTasks", () => {
  it("keeps open tasks due strictly before today", () => {
    const tasks = [task({ id: "t1", due_date: "2026-10-14" })];
    expect(getOverdueTasks(tasks, "2026-10-15")).toEqual(tasks);
  });

  it("drops tasks due today or later", () => {
    const tasks = [
      task({ id: "t1", due_date: "2026-10-15" }),
      task({ id: "t2", due_date: "2026-10-16" }),
    ];
    expect(getOverdueTasks(tasks, "2026-10-15")).toEqual([]);
  });

  it("drops done tasks even when their due day passed", () => {
    const tasks = [task({ status: "done", due_date: "2026-10-01" })];
    expect(getOverdueTasks(tasks, "2026-10-15")).toEqual([]);
  });

  it("drops open tasks without a due date", () => {
    expect(getOverdueTasks([task()], "2026-10-15")).toEqual([]);
  });
});

describe("pickDefaultWallet", () => {
  it("prefers the seeded Cash wallet wherever it sits in the list", () => {
    const wallets = [
      { wallet_id: "wal_bank", name: "Bank", balance_cents: 0 },
      { wallet_id: "wal_cash", name: "Cash", balance_cents: 0 },
    ];
    expect(pickDefaultWallet(wallets)).toBe("wal_cash");
  });

  it("falls back to the first wallet when Cash is archived", () => {
    const wallets = [{ wallet_id: "wal_bank", name: "Bank", balance_cents: 0 }];
    expect(pickDefaultWallet(wallets)).toBe("wal_bank");
  });

  it("answers empty when no wallet is active", () => {
    expect(pickDefaultWallet([])).toBe("");
  });
});

describe("pickTransferTo", () => {
  it("picks the first wallet that is not the source", () => {
    const wallets = [
      { wallet_id: "wal_cash", name: "Cash", balance_cents: 0 },
      { wallet_id: "wal_bank", name: "Bank", balance_cents: 0 },
    ];
    expect(pickTransferTo(wallets, "wal_cash")).toBe("wal_bank");
  });

  it("falls back to the source when it is the only wallet", () => {
    const wallets = [{ wallet_id: "wal_cash", name: "Cash", balance_cents: 0 }];
    expect(pickTransferTo(wallets, "wal_cash")).toBe("wal_cash");
  });
});
