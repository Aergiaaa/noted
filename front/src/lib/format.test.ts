import { describe, expect, it } from "vitest";
import {
  formatDateTime,
  formatTransactionRouting,
  formatDate,
  formatDayHeader,
  formatHourLabel,
  formatMoney,
  formatMoneyCompact,
  formatMonthTitle,
  formatTime,
  fromDayKey,
  parseMoneyToCents,
  parseSplitAmountToCents,
  toDayKey,
  toLocalInput,
} from "./format";

// Intl separates the currency symbol from the amount with a non-breaking
// space for some currencies (IDR); tests compare the readable form.
const readable = (s: string) => s.replace(/ /g, " ");

describe("formatTime", () => {
  it("renders HH:MM from an instant in local time", () => {
    const iso = new Date(2026, 9, 13, 10, 5).toISOString();
    expect(formatTime(iso)).toBe("10:05");
  });

  it("zero-pads single-digit hours and minutes", () => {
    const iso = new Date(2026, 9, 13, 7, 5).toISOString();
    expect(formatTime(iso)).toBe("07:05");
  });
});

describe("formatMonthTitle", () => {
  it("names the month and year of the given date", () => {
    expect(formatMonthTitle(new Date(2026, 9, 1))).toBe("October 2026");
    expect(formatMonthTitle(new Date(2026, 0, 31))).toBe("January 2026");
  });
});

describe("toLocalInput", () => {
  it("converts an RFC3339 instant to a datetime-local value", () => {
    const iso = new Date(2026, 9, 13, 10, 5).toISOString();
    const value = toLocalInput(iso);

    expect(value).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/);
    expect(new Date(value).getTime()).toBe(new Date(iso).getTime());
  });
});

describe("toDayKey / fromDayKey", () => {
  it("round-trips a local date through its YYYY-MM-DD key", () => {
    const date = new Date(2026, 9, 5);
    expect(toDayKey(date)).toBe("2026-10-05");
    const back = fromDayKey("2026-10-05");
    expect(back.getFullYear()).toBe(2026);
    expect(back.getMonth()).toBe(9);
    expect(back.getDate()).toBe(5);
    expect(back.getHours()).toBe(0);
  });
});

describe("formatDayHeader", () => {
  it("renders a short weekday and day number", () => {
    expect(formatDayHeader(new Date(2026, 9, 12))).toBe("Mon 12");
    expect(formatDayHeader(new Date(2026, 9, 18))).toBe("Sun 18");
  });
});

describe("formatHourLabel", () => {
  it("zero-pads the hour", () => {
    expect(formatHourLabel(0)).toBe("00");
    expect(formatHourLabel(9)).toBe("09");
    expect(formatHourLabel(23)).toBe("23");
  });
});

describe("formatDateTime", () => {
  it("renders the picker trigger for a naive local datetime", () => {
    expect(formatDateTime("2026-10-15T14:37")).toBe("Thu 15 Oct 2026, 14:37");
    expect(formatDateTime("2026-01-03T07:05")).toBe("Sat 3 Jan 2026, 07:05");
  });
});

describe("formatDate", () => {
  it("renders a local date without the time", () => {
    expect(formatDate(new Date(2026, 9, 15))).toBe("Thu 15 Oct 2026");
    expect(formatDate(new Date(2026, 0, 3))).toBe("Sat 3 Jan 2026");
  });
});

describe("formatMoney", () => {
  it("renders whole rupiah without decimal noise (id-ID grouping)", () => {
    expect(readable(formatMoney(2500000, "IDR", "id-ID"))).toBe("Rp 25.000");
  });

  it("keeps the minus sign on negative amounts", () => {
    expect(readable(formatMoney(-15000000, "IDR", "id-ID"))).toBe(
      "-Rp 150.000",
    );
  });

  it("shows cents for two-decimal currencies", () => {
    expect(readable(formatMoney(5055, "USD", "en-US"))).toBe("$50.55");
  });

  it("drops trailing zeros for whole two-decimal amounts", () => {
    expect(readable(formatMoney(5000, "USD", "en-US"))).toBe("$50");
  });

  it("groups by the caller's locale", () => {
    expect(readable(formatMoney(2500000, "IDR", "en-US"))).toBe("Rp 25,000");
  });

  it("falls back to the browser locale when no locale is given", () => {
    expect(formatMoney(5055, "USD")).toMatch(/50\.55/);
  });
});

describe("formatMoneyCompact", () => {
  it("shortens large rupiah to K notation", () => {
    expect(readable(formatMoneyCompact(12345678, "IDR"))).toBe("Rp 123K");
  });

  it("keeps the sign in compact form", () => {
    expect(readable(formatMoneyCompact(-15000000, "IDR"))).toBe("-Rp 150K");
  });

  it("leaves sub-thousand amounts untouched", () => {
    expect(readable(formatMoneyCompact(50000, "IDR"))).toBe("Rp 500");
  });

  it("uses B notation for billions", () => {
    expect(readable(formatMoneyCompact(250000000000, "IDR"))).toBe("Rp 2.5B");
  });

  it("compacts two-decimal currencies too", () => {
    expect(readable(formatMoneyCompact(123456, "USD"))).toBe("$1.2K");
  });
});

describe("parseMoneyToCents", () => {
  it("converts a decimal amount to cents", () => {
    expect(parseMoneyToCents("50.55")).toBe(5055);
    expect(parseMoneyToCents("50.5")).toBe(5050);
  });

  it("treats whole amounts as whole units", () => {
    expect(parseMoneyToCents("50")).toBe(5000);
    expect(parseMoneyToCents("  50  ")).toBe(5000);
  });

  it("accepts the one-cent minimum", () => {
    expect(parseMoneyToCents("0.01")).toBe(1);
  });

  it("rejects zero and negative amounts", () => {
    expect(parseMoneyToCents("0")).toBeNull();
    expect(parseMoneyToCents("0.00")).toBeNull();
    expect(parseMoneyToCents("-5")).toBeNull();
  });

  it("rejects more than two decimal places", () => {
    expect(parseMoneyToCents("50.555")).toBeNull();
  });

  it("rejects non-numeric input", () => {
    expect(parseMoneyToCents("")).toBeNull();
    expect(parseMoneyToCents("abc")).toBeNull();
    expect(parseMoneyToCents(".")).toBeNull();
  });

  it("rejects amounts above the 1e12 cents cap", () => {
    expect(parseMoneyToCents("10000000000")).toBe(1e12);
    expect(parseMoneyToCents("10000000000.01")).toBeNull();
  });
});

describe("parseSplitAmountToCents", () => {
  it("joins the primary digits and the three-digit group", () => {
    expect(parseSplitAmountToCents("200", "000")).toBe(20000000);
  });

  it("treats an empty group as 000", () => {
    expect(parseSplitAmountToCents("200", "")).toBe(20000000);
  });

  it("zero-pads a partial group", () => {
    expect(parseSplitAmountToCents("200", "5")).toBe(20000500);
    expect(parseSplitAmountToCents("200", "50")).toBe(20005000);
  });

  it("reads a group-only entry as a sub-thousand amount", () => {
    expect(parseSplitAmountToCents("", "500")).toBe(50000);
  });

  it("rejects a zero or empty entry", () => {
    expect(parseSplitAmountToCents("", "")).toBeNull();
    expect(parseSplitAmountToCents("0", "")).toBeNull();
  });

  it("rejects a group longer than three digits or with non-digits", () => {
    expect(parseSplitAmountToCents("200", "1234")).toBeNull();
    expect(parseSplitAmountToCents("200", "12a")).toBeNull();
  });

  it("rejects a non-numeric or decimal primary", () => {
    expect(parseSplitAmountToCents("abc", "000")).toBeNull();
    expect(parseSplitAmountToCents("50.55", "000")).toBeNull();
  });

  it("accepts the 1e12-cents cap and rejects past it", () => {
    expect(parseSplitAmountToCents("10000000", "000")).toBe(1e12);
    expect(parseSplitAmountToCents("10000000", "001")).toBeNull();
  });
});

describe("formatTransactionRouting", () => {
  const names = new Map([
    ["wal_cash", "Cash"],
    ["wal_bank", "Bank"],
  ]);
  const base = {
    amount_cents: 1,
    currency: "IDR",
    occurred_on: "2026-10-20",
    memo: null,
    task_id: null,
    event_id: null,
    note_id: null,
  };

  it("routes income into its wallet", () => {
    const tx = {
      ...base,
      kind: "income",
      wallet_id: "wal_cash",
      from_wallet_id: null,
      to_wallet_id: null,
    } as const;
    expect(formatTransactionRouting(tx, names)).toBe("\u2192 Cash");
  });

  it("routes an expense out of its wallet", () => {
    const tx = {
      ...base,
      kind: "expense",
      wallet_id: "wal_bank",
      from_wallet_id: null,
      to_wallet_id: null,
    } as const;
    expect(formatTransactionRouting(tx, names)).toBe("Bank \u2192");
  });

  it("routes a transfer between both sides", () => {
    const tx = {
      ...base,
      kind: "transfer",
      wallet_id: null,
      from_wallet_id: "wal_bank",
      to_wallet_id: "wal_cash",
    } as const;
    expect(formatTransactionRouting(tx, names)).toBe("Bank \u2192 Cash");
  });

  it("shows an unknown wallet id as-is", () => {
    const tx = {
      ...base,
      kind: "expense",
      wallet_id: "wal_gone",
      from_wallet_id: null,
      to_wallet_id: null,
    } as const;
    expect(formatTransactionRouting(tx, names)).toBe("wal_gone \u2192");
  });

  it("renders a null wallet side as empty", () => {
    const tx = {
      ...base,
      kind: "expense",
      wallet_id: null,
      from_wallet_id: null,
      to_wallet_id: null,
    } as const;
    expect(formatTransactionRouting(tx, names)).toBe(" \u2192");
  });
});
