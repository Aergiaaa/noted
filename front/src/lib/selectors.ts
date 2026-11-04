import type { Event, Note, Task, WalletBalance, Transaction } from "./api";
import { WALLET_SEED_CASH_ID } from "./api";
import type { MentionType } from "./mentions";

// Overdue = still open and its due calendar day is strictly before today.
// Comparison is plain DATE string order (YYYY-MM-DD sorts lexicographically).
export function getOverdueTasks(tasks: Task[], today: string): Task[] {
  return tasks.filter(
    (task) =>
      task.status === "open" && task.due_date !== null && task.due_date < today,
  );
}

// Events today: starts_at date equals today (local date derived from UTC instant).
export function getEventsToday(events: Event[], today: string): Event[] {
  return events.filter((event) => event.starts_at.startsWith(today));
}

// Tasks today: open tasks with due_date equal to today.
export function getTasksToday(tasks: Task[], today: string): Task[] {
  return tasks.filter(
    (task) =>
      task.status === "open" &&
      task.due_date !== null &&
      task.due_date === today,
  );
}

// Recent transactions: most recent N by occurred_on (already sorted DESC by API).
export function getRecentTransactions(
  transactions: Transaction[],
  limit: number,
): Transaction[] {
  return transactions.slice(0, limit);
}

// Pinned notes: pinned=true, already ordered by updated_at DESC from API.
export function getPinnedNotes(notes: Note[], limit: number): Note[] {
  return notes.filter((note) => note.pinned).slice(0, limit);
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

// Chip titles: a `kind:id` → title map over whatever lists the page
// already fetched. A miss (soft-deleted peer, still-loading sources) means
// the renderer falls back to the generic label or a muted raw token —
// never a raw id (F9). The map feeds resolveMentions; the resolver
// closure feeds the LinkChip-style callers.
export function createLinkTitleMap(sources: {
  tasks?: readonly Task[];
  events?: readonly Event[];
  notes?: readonly Note[];
}): Map<string, string> {
  const titles = new Map<string, string>();
  for (const task of sources.tasks ?? [])
    titles.set(`task:${task.id}`, task.title);
  for (const event of sources.events ?? []) {
    titles.set(`event:${event.id}`, event.title);
  }
  for (const note of sources.notes ?? [])
    titles.set(`note:${note.id}`, note.title);
  return titles;
}

export function createLinkTitleResolver(sources: {
  tasks?: readonly Task[];
  events?: readonly Event[];
  notes?: readonly Note[];
}): (kind: MentionType, id: string) => string | undefined {
  const titles = createLinkTitleMap(sources);
  return (kind, id) => titles.get(`${kind}:${id}`);
}
