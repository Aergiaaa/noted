import { describe, expect, it } from "vitest";
import type { Event, Task, Note, Transaction, WalletBalance } from "./api";
import {
  getEventsToday,
  getTasksToday,
  getOverdueTasks,
  getRecentTransactions,
  getPinnedNotes,
  pickDefaultWallet,
  pickTransferTo,
  createLinkTitleMap,
  createLinkTitleResolver,
} from "./selectors";

const TODAY = "2026-10-29";
const YESTERDAY = "2026-10-28";
const TOMORROW = "2026-10-30";

function makeEvent(id: string, startsAt: string): Event {
  return {
    id,
    title: `Event ${id}`,
    starts_at: startsAt,
    ends_at: startsAt,
    description: null,
    note_id: null,
  };
}

function makeTask(
  id: string,
  status: "open" | "done",
  dueDate: string | null,
): Task {
  return {
    id,
    title: `Task ${id}`,
    status,
    due_date: dueDate,
    event_id: null,
    note_id: null,
    completed_at: null,
  };
}

function makeNote(id: string, pinned: boolean): Note {
  return {
    id,
    title: `Note ${id}`,
    body_md: "",
    pinned,
  };
}

function makeTransaction(id: string, occurredOn: string): Transaction {
  return {
    id,
    kind: "expense",
    amount_cents: 1000,
    currency: "USD",
    occurred_on: occurredOn,
    memo: null,
    wallet_id: "wal_cash",
    from_wallet_id: null,
    to_wallet_id: null,
    task_id: null,
    event_id: null,
    note_id: null,
  };
}

function makeWallet(id: string, name: string): WalletBalance {
  return { wallet_id: id, name, balance_cents: 0 };
}

describe("selectors - dashboard", () => {
  describe("getEventsToday", () => {
    it("returns events whose starts_at date matches today", () => {
      const events = [
        makeEvent("e1", `${TODAY}T09:00:00Z`),
        makeEvent("e2", `${TODAY}T14:00:00Z`),
        makeEvent("e3", `${YESTERDAY}T10:00:00Z`),
        makeEvent("e4", `${TOMORROW}T10:00:00Z`),
      ];

      const result = getEventsToday(events, TODAY);

      expect(result).toHaveLength(2);
      expect(result.map((e) => e.id)).toEqual(["e1", "e2"]);
    });

    it("returns empty when no events match today", () => {
      const events = [
        makeEvent("e1", `${YESTERDAY}T10:00:00Z`),
        makeEvent("e2", `${TOMORROW}T10:00:00Z`),
      ];

      const result = getEventsToday(events, TODAY);

      expect(result).toHaveLength(0);
    });
  });

  describe("getTasksToday", () => {
    it("returns open tasks with due_date equal to today", () => {
      const tasks = [
        makeTask("t1", "open", TODAY),
        makeTask("t2", "open", TODAY),
        makeTask("t3", "done", TODAY),
        makeTask("t4", "open", YESTERDAY),
        makeTask("t5", "open", TOMORROW),
        makeTask("t6", "open", null),
      ];

      const result = getTasksToday(tasks, TODAY);

      expect(result).toHaveLength(2);
      expect(result.map((t) => t.id)).toEqual(["t1", "t2"]);
    });

    it("returns empty when no tasks match today", () => {
      const tasks = [
        makeTask("t1", "done", TODAY),
        makeTask("t2", "open", YESTERDAY),
      ];

      const result = getTasksToday(tasks, TODAY);

      expect(result).toHaveLength(0);
    });
  });

  describe("getOverdueTasks", () => {
    it("returns open tasks with due_date strictly before today", () => {
      const tasks = [
        makeTask("t1", "open", YESTERDAY),
        makeTask("t2", "open", "2026-10-20"),
        makeTask("t3", "done", YESTERDAY),
        makeTask("t4", "open", TODAY),
        makeTask("t5", "open", TOMORROW),
        makeTask("t6", "open", null),
      ];

      const result = getOverdueTasks(tasks, TODAY);

      expect(result).toHaveLength(2);
      expect(result.map((t) => t.id)).toEqual(["t1", "t2"]);
    });

    it("returns empty when no tasks are overdue", () => {
      const tasks = [
        makeTask("t1", "done", YESTERDAY),
        makeTask("t2", "open", TODAY),
        makeTask("t3", "open", TOMORROW),
      ];

      const result = getOverdueTasks(tasks, TODAY);

      expect(result).toHaveLength(0);
    });
  });

  describe("getRecentTransactions", () => {
    it("returns the first N transactions (already sorted DESC by API)", () => {
      const transactions = [
        makeTransaction("tx1", TODAY),
        makeTransaction("tx2", YESTERDAY),
        makeTransaction("tx3", "2026-10-20"),
        makeTransaction("tx4", "2026-10-15"),
      ];

      const result = getRecentTransactions(transactions, 2);

      expect(result).toHaveLength(2);
      expect(result.map((t) => t.id)).toEqual(["tx1", "tx2"]);
    });

    it("returns all when limit exceeds length", () => {
      const transactions = [makeTransaction("tx1", TODAY)];

      const result = getRecentTransactions(transactions, 5);

      expect(result).toHaveLength(1);
    });

    it("returns empty when limit is 0", () => {
      const transactions = [makeTransaction("tx1", TODAY)];

      const result = getRecentTransactions(transactions, 0);

      expect(result).toHaveLength(0);
    });
  });

  describe("getPinnedNotes", () => {
    it("returns pinned notes up to limit", () => {
      const notes = [
        makeNote("n1", true),
        makeNote("n2", true),
        makeNote("n3", false),
        makeNote("n4", true),
      ];

      const result = getPinnedNotes(notes, 2);

      expect(result).toHaveLength(2);
      expect(result.map((n) => n.id)).toEqual(["n1", "n2"]);
    });

    it("returns all pinned when limit exceeds count", () => {
      const notes = [makeNote("n1", true), makeNote("n2", false)];

      const result = getPinnedNotes(notes, 5);

      expect(result).toHaveLength(1);
      expect(result[0].id).toBe("n1");
    });

    it("returns empty when no notes are pinned", () => {
      const notes = [makeNote("n1", false), makeNote("n2", false)];

      const result = getPinnedNotes(notes, 5);

      expect(result).toHaveLength(0);
    });
  });
});

describe("selectors - finance (F9)", () => {
  describe("pickDefaultWallet", () => {
    it("returns wal_cash when present", () => {
      const wallets = [
        makeWallet("wal_bank", "Bank"),
        makeWallet("wal_cash", "Cash"),
      ];

      const result = pickDefaultWallet(wallets);

      expect(result).toBe("wal_cash");
    });

    it("returns first wallet when wal_cash is absent", () => {
      const wallets = [
        makeWallet("wal_bank", "Bank"),
        makeWallet("wal_other", "Other"),
      ];

      const result = pickDefaultWallet(wallets);

      expect(result).toBe("wal_bank");
    });

    it("returns empty string when wallets is empty", () => {
      const result = pickDefaultWallet([]);

      expect(result).toBe("");
    });
  });

  describe("pickTransferTo", () => {
    it("returns first wallet that is not the source", () => {
      const wallets = [
        makeWallet("wal_cash", "Cash"),
        makeWallet("wal_bank", "Bank"),
        makeWallet("wal_other", "Other"),
      ];

      const result = pickTransferTo(wallets, "wal_cash");

      expect(result).toBe("wal_bank");
    });

    it("returns source wallet when only one wallet exists", () => {
      const wallets = [makeWallet("wal_cash", "Cash")];

      const result = pickTransferTo(wallets, "wal_cash");

      expect(result).toBe("wal_cash");
    });

    it("returns source wallet when wallets is empty", () => {
      const result = pickTransferTo([], "wal_cash");

      expect(result).toBe("wal_cash");
    });
  });

  describe("createLinkTitleMap", () => {
    it("maps tasks, events, and notes to kind:id keys", () => {
      const tasks = [makeTask("t1", "open", TODAY)];
      const events = [makeEvent("e1", `${TODAY}T10:00:00Z`)];
      const notes = [makeNote("n1", true)];

      const result = createLinkTitleMap({ tasks, events, notes });

      expect(result.get("task:t1")).toBe("Task t1");
      expect(result.get("event:e1")).toBe("Event e1");
      expect(result.get("note:n1")).toBe("Note n1");
    });

    it("handles undefined sources gracefully", () => {
      const result = createLinkTitleMap({});

      expect(result.size).toBe(0);
    });

    it("handles partial sources", () => {
      const tasks = [makeTask("t1", "open", TODAY)];
      const result = createLinkTitleMap({ tasks });

      expect(result.get("task:t1")).toBe("Task t1");
      expect(result.has("event:e1")).toBe(false);
    });
  });

  describe("createLinkTitleResolver", () => {
    it("resolves titles for known IDs", () => {
      const tasks = [makeTask("t1", "open", TODAY)];
      const resolver = createLinkTitleResolver({ tasks });

      expect(resolver("task", "t1")).toBe("Task t1");
    });

    it("returns undefined for unknown IDs", () => {
      const tasks = [makeTask("t1", "open", TODAY)];
      const resolver = createLinkTitleResolver({ tasks });

      expect(resolver("task", "unknown")).toBeUndefined();
      expect(resolver("event", "e1")).toBeUndefined();
    });
  });
});
