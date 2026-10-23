import type { Task, WalletBalance } from "./api";
import { WALLET_SEED_CASH_ID } from "./api";

// Overdue = still open and its due calendar day is strictly before today.
// Comparison is plain DATE string order (YYYY-MM-DD sorts lexicographically).
export function getOverdueTasks(tasks: Task[], today: string): Task[] {
  return tasks.filter(
    (task) =>
      task.status === "open" && task.due_date !== null && task.due_date < today,
  );
}

// New transactions start in the seeded Cash wallet when it is active
// (the balance list is name-ordered, so position alone would pick Bank).
export function pickDefaultWallet(wallets: WalletBalance[]): string {
  const cash = wallets.find(
    (wallet) => wallet.wallet_id === WALLET_SEED_CASH_ID,
  );
  if (cash !== undefined) return cash.wallet_id;
  return wallets[0]?.wallet_id ?? "";
}

// A transfer needs two pots: default the far side to the first wallet
// that is not the source — with a single wallet that is the source
// itself, which the form rejects as same-wallet before any request.
export function pickTransferTo(
  wallets: WalletBalance[],
  fromWalletID: string,
): string {
  const other = wallets.find((wallet) => wallet.wallet_id !== fromWalletID);
  return other?.wallet_id ?? fromWalletID;
}
