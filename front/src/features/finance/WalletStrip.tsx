import type { WalletBalance } from "../../lib/api";
import MoneyBadge from "./MoneyBadge";

interface WalletStripProps {
  wallets: WalletBalance[];
  currency: string;
  activeWalletId: string | null;
  onSelect: (walletId: string | null) => void;
  onManage: () => void;
}

// Compact balance strip: one card per active wallet; clicking toggles the
// history's wallet filter (click again to clear), the manage action opens
// the wallet modal (UI.md Finance).
export default function WalletStrip({
  wallets,
  currency,
  activeWalletId,
  onSelect,
  onManage,
}: WalletStripProps) {
  return (
    <div className="mt-3 flex flex-wrap items-center gap-2">
      {wallets.map((wallet) => {
        const active = wallet.wallet_id === activeWalletId;
        return (
          <button
            key={wallet.wallet_id}
            type="button"
            data-testid="wallet-card"
            aria-pressed={active}
            className={`card justify-between gap-3 border bg-base-200 px-3 py-2 text-left hover:bg-base-300 ${
              active ? "border-primary" : "border-base-300"
            }`}
            onClick={() => onSelect(active ? null : wallet.wallet_id)}
          >
            <span className="text-xs text-base-content/60">{wallet.name}</span>
            <MoneyBadge
              cents={wallet.balance_cents}
              currency={currency}
              compact
            />
          </button>
        );
      })}
      <button type="button" className="btn btn-ghost btn-sm" onClick={onManage}>
        Manage wallets
      </button>
    </div>
  );
}
