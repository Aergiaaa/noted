import { useEffect, useMemo, useState } from "react";
import {
  api,
  errorLines,
  type BalanceResponse,
  type Transaction,
  type TransactionsResponse,
  type Wallet,
  type WalletsResponse,
} from "../lib/api";
import WalletStrip from "../features/finance/WalletStrip";
import WalletsModal from "../features/finance/WalletsModal";
import TransactionList from "../features/finance/TransactionList";
import TransactionForm from "../features/finance/TransactionForm";
import TransferForm from "../features/finance/TransferForm";
import MoneyBadge from "../features/finance/MoneyBadge";
import ErrorState from "../components/ui/ErrorState";
import EmptyState from "../components/ui/EmptyState";
import type { LinkField } from "../features/links/LinkChip";
import { useLinkSources } from "../features/links/useLinkSources";
import { createLinkTitleResolver } from "../lib/selectors";

const LOAD_FAILED = "Couldn't load finance.";
const VOID_FAILED = "Couldn't void the transaction.";
const UNLINK_FAILED = "Couldn't unlink the transaction.";
const EMPTY_MESSAGE = "No transactions yet.";
const CREATE_CTA = "Record your first transaction";

// Chip titles resolve from these lists (F9) — fetched once per mount.
const TX_CHIP_SOURCES = ["task", "event", "note"] as const;

// Account = the two always-settled-together reads (balance + full wallet
// list); history = one transactions page + its cursor. Both start null —
// null IS the first-load gate (retry re-gates by nulling them), so no
// separate loading flag or placeholder balance exists.
type Account = { balance: BalanceResponse; wallets: Wallet[] };
type History = { rows: Transaction[]; cursor: string | null };

// One modal at a time — the union makes "create + manage" unrepresentable
// (same shape as F7's FormTarget).
type ModalTarget =
  { kind: "transaction" } | { kind: "transfer" } | { kind: "wallets" } | null;

// Opaque history cursor rides the query (API.md); URLSearchParams keeps
// the ordering stable for tests.
function txPath(walletFilter: string | null, cursor?: string): string {
  const params = new URLSearchParams();

  if (walletFilter !== null) params.set("wallet_id", walletFilter);
  if (cursor !== undefined) params.set("cursor", cursor);

  const query = params.toString();
  return query === ""
    ? "/finance/transactions"
    : `/finance/transactions?${query}`;
}

export default function FinancePage() {
  const [account, setAccount] = useState<Account | null>(null);
  const [history, setHistory] = useState<History | null>(null);
  // Per-scope errors: a history failure must not wipe a balance error or
  // vice versa; the page shows whichever fired first. This is why two
  // slots survive the union pass — one slot would let a silent success
  // clear the other scope's error.
  const [balanceError, setBalanceError] = useState<string | null>(null);
  const [historyError, setHistoryError] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);
  const [walletFilter, setWalletFilter] = useState<string | null>(null);
  const [modal, setModal] = useState<ModalTarget>(null);

  // Account data — balance + full wallet list. Independent of the history
  // filter, so a strip toggle must not re-run it.
  useEffect(() => {
    let cancelled = false;
    Promise.all([
      api<BalanceResponse>("/finance/balance"),
      api<WalletsResponse>("/finance/wallets?archived=1"),
    ])
      .then(([nextBalance, wallets]) => {
        if (!cancelled) {
          setAccount({ balance: nextBalance, wallets: wallets.data });
          setBalanceError(null);
        }
      })

      .catch((err: unknown) => {
        if (!cancelled) setBalanceError(errorLines(err, LOAD_FAILED)[0]);
      });
    return () => {
      cancelled = true;
    };
  }, [nonce]);

  // History — the only scope the strip filter touches. Rows swap in place;
  // the cancelled flag drops a response that lands after its replacement.
  useEffect(() => {
    let cancelled = false;
    api<TransactionsResponse>(txPath(walletFilter))
      .then((tx) => {
        if (!cancelled) {
          setHistory({ rows: tx.data, cursor: tx.next_cursor });
          setHistoryError(null);
        }
      })

      .catch((err: unknown) => {
        if (!cancelled) setHistoryError(errorLines(err, LOAD_FAILED)[0]);
      });
    return () => {
      cancelled = true;
    };
  }, [nonce, walletFilter]);

  // Declared after the history effect so mount call order stays
  // balance → wallets → transactions → task/event/note (test routers
  // match on URL, but the sequence keeps the strict call-list assert).
  const linkSources = useLinkSources(TX_CHIP_SOURCES);
  const linkTitle = useMemo(
    () => createLinkTitleResolver(linkSources),
    [linkSources],
  );

  const walletNames = useMemo(
    () =>
      new Map(
        (account?.wallets ?? []).map((wallet) => [wallet.id, wallet.name]),
      ),
    [account],
  );

  const error = balanceError ?? historyError;

  // A mutation refresh: silent — old data stays mounted, errors clear.
  const refetch = () => {
    setBalanceError(null);
    setHistoryError(null);
    setNonce((n) => n + 1);
  };

  // A failed load restarts from scratch: null both scopes so the
  // skeleton shows while the retry is in flight (same as the pre-union
  // ready reset).
  const retry = () => {
    setBalanceError(null);
    setHistoryError(null);
    setAccount(null);
    setHistory(null);
    setNonce((n) => n + 1);
  };

  const loadMore = async (activeCursor: string) => {
    try {
      const page = await api<TransactionsResponse>(
        txPath(walletFilter, activeCursor),
      );
      // Non-null by construction: the button only renders inside the
      // loaded history. Assert, don't branch — a null guard here
      // would be dead code the coverage gate can't reach.
      setHistory((prev) => ({
        rows: [...prev!.rows, ...page.data],
        cursor: page.next_cursor,
      }));
    } catch (err: unknown) {
      setHistoryError(errorLines(err, LOAD_FAILED)[0]);
    }
  };

  const voidTransaction = async (id: string) => {
    try {
      await api(`/finance/transactions/${id}`, { method: "DELETE" });
      refetch();
    } catch (err: unknown) {
      setHistoryError(errorLines(err, VOID_FAILED)[0]);
    }
  };

  // Unlink one FK field: "" clears it (API.md PATCH semantics), then the
  // history refetches so the chip disappears with the stored link.
  const unlinkTransaction = async (id: string, field: LinkField) => {
    try {
      await api(`/finance/transactions/${id}`, {
        method: "PATCH",
        body: { [field]: "" },
      });
      refetch();
    } catch (err: unknown) {
      setHistoryError(errorLines(err, UNLINK_FAILED)[0]);
    }
  };

  if (error !== null) {
    return (
      <section>
        <h1 className="text-xl font-semibold">Finance</h1>
        <ErrorState message={error} onRetry={retry} />
      </section>
    );
  }

  if (account === null || history === null) {
    return (
      <section>
        <h1 className="text-xl font-semibold">Finance</h1>
        <div className="mt-4 space-y-3">
          <div className="skeleton h-8 w-40" />
          <div className="skeleton h-14 w-64" />
          {Array.from({ length: 3 }, (_, i) => (
            <div key={i} className="skeleton h-4" />
          ))}
        </div>
      </section>
    );
  }

  return (
    <section>
      <header className="flex flex-wrap items-center justify-between gap-2">
        <h1 className="text-xl font-semibold">Finance</h1>
        <div className="flex gap-2">
          <button
            type="button"
            className="btn btn-primary btn-sm"
            onClick={() => setModal({ kind: "transaction" })}
          >
            Transaction
          </button>
          <button
            type="button"
            className="btn btn-sm"
            onClick={() => setModal({ kind: "transfer" })}
          >
            Transfer
          </button>
        </div>
      </header>

      <div className="mt-4 card bg-base-200 border border-base-300 p-4">
        <span className="text-xs uppercase text-base-content/60">
          Total balance
        </span>
        <div className="mt-1 text-2xl font-semibold">
          <MoneyBadge
            cents={account.balance.total.balance_cents}
            currency={account.balance.currency}
          />
        </div>
        <WalletStrip
          wallets={account.balance.wallets}
          currency={account.balance.currency}
          activeWalletId={walletFilter}
          onSelect={setWalletFilter}
          onManage={() => setModal({ kind: "wallets" })}
        />
      </div>

      <div className="mt-4 card bg-base-200 border border-base-300 p-4">
        <h2 className="text-sm font-semibold">History</h2>
        <div className="mt-2">
          {history.rows.length === 0 ? (
            <EmptyState
              message={EMPTY_MESSAGE}
              action={{
                label: CREATE_CTA,
                onClick: () => setModal({ kind: "transaction" }),
              }}
            />
          ) : (
            <TransactionList
              rows={history.rows}
              walletNames={walletNames}
              onVoid={(id) => void voidTransaction(id)}
              cursor={history.cursor}
              onLoadMore={(c) => void loadMore(c)}
              linkTitle={linkTitle}
              onUnlink={(id, field) => void unlinkTransaction(id, field)}
            />
          )}
        </div>
      </div>

      {modal?.kind === "transaction" && (
        <TransactionForm
          wallets={account.balance.wallets}
          currency={account.balance.currency}
          onClose={() => setModal(null)}
          onSaved={refetch}
        />
      )}
      {modal?.kind === "transfer" && (
        <TransferForm
          wallets={account.balance.wallets}
          currency={account.balance.currency}
          onClose={() => setModal(null)}
          onSaved={refetch}
        />
      )}
      {modal?.kind === "wallets" && (
        <WalletsModal
          wallets={account.wallets}
          onClose={() => setModal(null)}
          onChanged={refetch}
        />
      )}
    </section>
  );
}
