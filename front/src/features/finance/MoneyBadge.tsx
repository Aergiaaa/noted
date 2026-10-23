import { formatMoney, formatMoneyCompact } from "../../lib/format";

interface MoneyBadgeProps {
  cents: number;
  currency: string;
  // Explicit sign for row amounts: "+" income, "−" expense. Transfers and
  // balances omit it — they sign themselves from cents (negatives turn
  // error-colored). Sign is text, never a background (UI.md).
  sign?: "+" | "−";
  // Compact K/M notation for dense lists, with the full value as tooltip.
  compact?: boolean;
}

export default function MoneyBadge({
  cents,
  currency,
  sign,
  compact = false,
}: MoneyBadgeProps) {
  const full = formatMoney(cents, currency);
  const text = compact ? formatMoneyCompact(cents, currency) : full;
  const tone =
    cents < 0 || sign === "−"
      ? "text-error"
      : sign === "+"
        ? "text-success"
        : "";
  const shown = sign !== undefined && cents >= 0 ? `${sign}${text}` : text;

  return (
    <span
      className={`tabular-nums ${tone}`.trim()}
      title={compact ? full : undefined}
    >
      {shown}
    </span>
  );
}
