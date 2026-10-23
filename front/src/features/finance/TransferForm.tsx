import { useState, type SubmitEvent } from "react";
import { api, errorLines, type WalletBalance } from "../../lib/api";
import { pickDefaultWallet, pickTransferTo } from "../../lib/selectors";
import { parseSplitAmountToCents, toDayKey } from "../../lib/format";
import { sanitizeLine } from "../../lib/sanitize";
import Modal from "../../components/ui/Modal";

const SAVE_FAILED = "Couldn't move the money.";
const AMOUNT_INVALID = "Enter a valid amount.";
const DATE_REQUIRED = "Date is required.";
const SAME_WALLET = "Choose two different wallets.";
const MEMO_MAX_CHAR = 500;

interface TransferFormProps {
  wallets: WalletBalance[];
  currency: string;
  onClose: () => void;
  onSaved: () => void;
}

// Move money between two pots (single kind=transfer row, API.md). The
// same-wallet rule is checked client-side first — the server enforces it
// too, but the picker should not be able to send it.
export default function TransferForm({
  wallets,
  currency,
  onClose,
  onSaved,
}: TransferFormProps) {
  const [fromWalletId, setFromWalletId] = useState(() =>
    pickDefaultWallet(wallets),
  );
  const [toWalletId, setToWalletId] = useState(() =>
    pickTransferTo(wallets, pickDefaultWallet(wallets)),
  );
  const [amount, setAmount] = useState("");
  const [amountGroup, setAmountGroup] = useState("");
  const [occurredOn, setOccurredOn] = useState(() => toDayKey(new Date()));
  const [memo, setMemo] = useState("");
  const [errors, setErrors] = useState<string[]>([]);
  const [saving, setSaving] = useState(false);

  const onSubmit = async (submitEvent: SubmitEvent) => {
    submitEvent.preventDefault();
    const cents = parseSplitAmountToCents(amount, amountGroup);
    const nextErrors: string[] = [];
    if (cents === null) nextErrors.push(AMOUNT_INVALID);
    if (occurredOn === "") nextErrors.push(DATE_REQUIRED);
    if (fromWalletId === toWalletId) nextErrors.push(SAME_WALLET);
    if (nextErrors.length > 0) {
      setErrors(nextErrors);
      return;
    }

    setErrors([]);
    setSaving(true);
    const body: Record<string, unknown> = {
      from_wallet_id: fromWalletId,
      to_wallet_id: toWalletId,
      amount_cents: cents,
      occurred_on: occurredOn,
    };
    const cleanMemo = sanitizeLine(memo);
    if (cleanMemo !== "") body.memo = cleanMemo;

    try {
      await api("/finance/transfers", { method: "POST", body });
      onSaved();
      onClose();
    } catch (err: unknown) {
      setErrors(errorLines(err, SAVE_FAILED));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal onClose={onClose}>
      <h2 className="text-base font-semibold">New transfer</h2>
      <form className="mt-3 space-y-3" onSubmit={onSubmit} noValidate>
        <label className="block">
          <span className="label-text text-xs uppercase text-base-content/60">
            From
          </span>
          <select
            className="select select-bordered select-sm mt-1 w-full"
            aria-label="From"
            value={fromWalletId}
            onChange={(e) => setFromWalletId(e.target.value)}
          >
            {wallets.map((wallet) => (
              <option key={wallet.wallet_id} value={wallet.wallet_id}>
                {wallet.name}
              </option>
            ))}
          </select>
        </label>
        <label className="block">
          <span className="label-text text-xs uppercase text-base-content/60">
            To
          </span>
          <select
            className="select select-bordered select-sm mt-1 w-full"
            aria-label="To"
            value={toWalletId}
            onChange={(e) => setToWalletId(e.target.value)}
          >
            {wallets.map((wallet) => (
              <option key={wallet.wallet_id} value={wallet.wallet_id}>
                {wallet.name}
              </option>
            ))}
          </select>
        </label>
        <label className="block">
          <span className="label-text text-xs uppercase text-base-content/60">
            Amount
          </span>
          <div className="mt-1 flex items-center gap-2">
            <input
              className="input input-bordered input-sm w-full"
              aria-label="Amount"
              inputMode="numeric"
              placeholder="0"
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
            />
            <input
              className="input input-bordered input-sm w-20"
              aria-label="Amount group"
              inputMode="numeric"
              maxLength={3}
              placeholder="000"
              value={amountGroup}
              onChange={(e) => setAmountGroup(e.target.value)}
            />
            <span className="text-xs text-base-content/60">{currency}</span>
          </div>
        </label>
        <label className="block">
          <span className="label-text text-xs uppercase text-base-content/60">
            Date
          </span>
          <input
            type="date"
            className="input input-bordered input-sm mt-1 w-full"
            aria-label="Date"
            value={occurredOn}
            onChange={(e) => setOccurredOn(e.target.value)}
          />
        </label>
        <label className="block">
          <span className="label-text text-xs uppercase text-base-content/60">
            Memo
          </span>
          <input
            className="input input-bordered input-sm mt-1 w-full"
            aria-label="Memo"
            maxLength={MEMO_MAX_CHAR}
            value={memo}
            onChange={(e) => setMemo(e.target.value)}
          />
        </label>

        {errors.length > 0 && (
          <ul className="text-xs text-error" role="alert">
            {errors.map((line) => (
              <li key={line}>{line}</li>
            ))}
          </ul>
        )}

        <div className="mt-4 flex justify-end gap-2">
          <button
            type="button"
            className="btn btn-ghost btn-sm"
            onClick={onClose}
          >
            Cancel
          </button>
          <button
            type="submit"
            className="btn btn-primary btn-sm"
            disabled={saving}
          >
            {saving ? "Moving…" : "Move"}
          </button>
        </div>
      </form>
    </Modal>
  );
}
