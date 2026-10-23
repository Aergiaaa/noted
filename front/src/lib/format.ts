import type { Transaction } from "./api";

// All calendar rendering is browser-local (DOMAIN.md: instants UTC in,
// local out). Values are pre-formatted strings, so no Intl options means
// no locale drift between machines. Money is the deliberate exception
// below — currency symbols and grouping ARE locale work.

const MONTHS = [
  "January",
  "February",
  "March",
  "April",
  "May",
  "June",
  "July",
  "August",
  "September",
  "October",
  "November",
  "December",
];

const MONTHS_SHORT = [
  "Jan",
  "Feb",
  "Mar",
  "Apr",
  "May",
  "Jun",
  "Jul",
  "Aug",
  "Sep",
  "Oct",
  "Nov",
  "Dec",
];

export function formatTime(iso: string): string {
  const date = new Date(iso);
  return `${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

export function formatMonthTitle(date: Date): string {
  return `${MONTHS[date.getMonth()]} ${date.getFullYear()}`;
}

// datetime-local input value (YYYY-MM-DDTHH:MM) for an instant.
export function toLocalInput(iso: string): string {
  const date = new Date(iso);
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

// Stable key for a local day: YYYY-MM-DD (grid aria-labels, comparisons).
export function toDayKey(date: Date): string {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

// Naive local datetime (YYYY-MM-DDTHH:MM) for the date-time picker
// trigger: "Thu 15 Oct 2026, 14:37".
export function formatDateTime(value: string): string {
  const day = new Date(value);
  return `${WEEKDAYS[day.getDay()]} ${day.getDate()} ${MONTHS_SHORT[day.getMonth()]} ${day.getFullYear()}, ${value.slice(11)}`;
}

// Date-only companion for the DatePicker trigger: "Thu 15 Oct 2026".
export function formatDate(date: Date): string {
  return `${WEEKDAYS[date.getDay()]} ${date.getDate()} ${MONTHS_SHORT[date.getMonth()]} ${date.getFullYear()}`;
}

// Parse a YYYY-MM-DD day key back into a local Date (midnight).
export function fromDayKey(key: string): Date {
  const [year, month, day] = key.split("-").map(Number);
  return new Date(year, month - 1, day);
}

const WEEKDAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

// Weekday header for the time grid: "Mon 12".
export function formatDayHeader(date: Date): string {
  return `${WEEKDAYS[date.getDay()]} ${date.getDate()}`;
}

// Compact hour label for the time gutter: "09".
export function formatHourLabel(hour: number): string {
  return pad(hour);
}

function pad(n: number): string {
  return String(n).padStart(2, "0");
}

const MONEY_MAX_CENTS = 1e12;

// Full-precision money for headers and tooltips: browser locale when no
// locale is given; whole amounts drop trailing zeros (Rp 25.000, $50)
// while real cents always show ($50.55).
export function formatMoney(
  cents: number,
  currency: string,
  locale?: string,
): string {
  return new Intl.NumberFormat(locale, {
    style: "currency",
    currency,
    currencyDisplay: "narrowSymbol",
    minimumFractionDigits: 0,
    maximumFractionDigits: 2,
  }).format(cents / 100);
}

// Dense-list money (rows, wallet strip): pinned en-US so amounts read
// K/M everywhere — an id-ID browser would otherwise say "Rp 123 rb".
export function formatMoneyCompact(cents: number, currency: string): string {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency,
    currencyDisplay: "narrowSymbol",
    notation: "compact",
  }).format(cents / 100);
}

// Decimal major-unit input ("50.55") to wire cents (5055). Returns null
// for anything invalid (not numeric, <= 0, > 2 decimals, above the
// 1e12-cents cap) so the caller can show the inline field error.
export function parseMoneyToCents(input: string): number | null {
  const trimmed = input.trim();
  if (!/^\d+(\.\d{1,2})?$/.test(trimmed)) return null;
  const cents = Math.round(parseFloat(trimmed) * 100);
  if (cents < 1 || cents > MONEY_MAX_CENTS) return null;
  return cents;
}

// Split amount entry: primary digits + a 3-digit trailing group —
// [200][000] = 200,000 major units. An empty group defaults to 000.
// Whole major units only (no decimals); null for anything invalid so
// the caller can show the inline field error.
export function parseSplitAmountToCents(
  primary: string,
  group: string,
): number | null {
  const trimmedGroup = group.trim();
  if (!/^\d{0,3}$/.test(trimmedGroup)) return null;
  const major = `${primary.trim()}${trimmedGroup.padStart(3, "0")}`;
  return parseMoneyToCents(major);
}

// Wallet routing for one history row: income → wallet, expense wallet →,
// transfer from → to. Unknown ids render as themselves rather than a
// blank cell; a null side renders empty (defensive against malformed rows).
export function formatTransactionRouting(
  tx: Pick<
    Transaction,
    "kind" | "wallet_id" | "from_wallet_id" | "to_wallet_id"
  >,
  walletNames: Map<string, string>,
): string {
  const name = (id: string | null): string =>
    id === null ? "" : (walletNames.get(id) ?? id);

  return tx.kind === "transfer"
    ? `${name(tx.from_wallet_id)} → ${name(tx.to_wallet_id)}`
    : tx.kind === "income"
      ? `→ ${name(tx.wallet_id)}`
      : `${name(tx.wallet_id)} →`;
}
