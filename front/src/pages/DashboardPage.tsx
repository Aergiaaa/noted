import { useState } from "react";
import { Link } from "react-router";
import { useFetch } from "../lib/useFetch";
import { type DashboardResponse } from "../lib/api";
import { formatTime, toDayKey } from "../lib/format";
import {
  getEventsToday,
  getTasksToday,
  getOverdueTasks,
  getRecentTransactions,
  getPinnedNotes,
} from "../lib/selectors";
import MoneyBadge from "../features/finance/MoneyBadge";
import EmptyState from "../components/ui/EmptyState";
import ErrorState from "../components/ui/ErrorState";

const RECENT_TX_LIMIT = 5;
const PINNED_NOTES_LIMIT = 5;

// Frontend owns the date (API.md): always send explicit date, render in browser-local.
function getTodayKey(): string {
  return toDayKey(new Date());
}

export default function DashboardPage() {
  const [todayKey] = useState<string>(() => getTodayKey());

  const path = `/dashboard?date=${todayKey}`;
  const { data, error, loading, retry } = useFetch<DashboardResponse>(
    path,
    "Failed to load dashboard",
  );

  if (loading) {
    return (
      <section>
        <div
          className="flex items-center gap-2"
          role="status"
          aria-live="polite"
          aria-label="Loading dashboard"
        >
          <span className="loading loading-spinner loading-sm" />
          <span className="text-sm text-base-content/70">
            Loading dashboard…
          </span>
        </div>
        <div className="grid gap-4 mt-6 md:grid-cols-2 xl:grid-cols-3">
          {Array.from({ length: 5 }).map((_, i) => (
            <div
              key={i}
              className="card bg-base-200 border border-base-300 p-4"
            >
              <div className="skeleton h-5 w-1/3 mb-3" />
              <div className="space-y-2">
                <div className="skeleton h-4 w-full" />
                <div className="skeleton h-4 w-3/4" />
                <div className="skeleton h-4 w-1/2" />
              </div>
            </div>
          ))}
        </div>
      </section>
    );
  }

  if (error !== null) {
    return (
      <section>
        <ErrorState message={error} onRetry={retry} />
      </section>
    );
  }

  // useFetch guarantees data is non-null when loading is false and error is null
  const dashboard = data!;

  const eventsToday = getEventsToday(dashboard.events_today, todayKey);
  const tasksToday = getTasksToday(dashboard.tasks_today, todayKey);
  const overdueTasks = getOverdueTasks(dashboard.overdue_tasks, todayKey);
  const recentTxs = getRecentTransactions(
    dashboard.recent_transactions,
    RECENT_TX_LIMIT,
  );
  const pinnedNotes = getPinnedNotes(
    dashboard.pinned_notes,
    PINNED_NOTES_LIMIT,
  );

  return (
    <section>
      <header className="flex items-center justify-between mb-6">
        <h1 className="text-xl font-semibold">Today at a glance</h1>
        <time dateTime={todayKey} className="text-sm text-base-content/70">
          {new Date(todayKey).toLocaleDateString(undefined, {
            weekday: "long",
            month: "short",
            day: "numeric",
          })}
        </time>
      </header>

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {/* Events today */}
        <Card title="Events today" to="/calendar">
          {eventsToday.length === 0 ? (
            <EmptyState message="No events today" />
          ) : (
            <ul className="space-y-2" role="list">
              {eventsToday.map((ev) => (
                <li
                  key={ev.id}
                  className="flex items-center justify-between gap-2"
                >
                  <Link
                    to={`/calendar?event=${ev.id}`}
                    className="flex-1 truncate hover:underline"
                  >
                    {ev.title}
                  </Link>
                  <time className="text-xs text-base-content/70 whitespace-nowrap">
                    {formatTime(ev.starts_at)}
                  </time>
                </li>
              ))}
            </ul>
          )}
        </Card>

        {/* Tasks today */}
        <Card title="Tasks today" to="/tasks">
          {tasksToday.length === 0 ? (
            <EmptyState message="No tasks due today" />
          ) : (
            <ul className="space-y-2" role="list">
              {tasksToday.map((task) => (
                <li key={task.id}>
                  <Link to={`/tasks?id=${task.id}`} className="hover:underline">
                    {task.title}
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </Card>

        {/* Overdue tasks */}
        <Card title="Overdue tasks" to="/tasks">
          {overdueTasks.length === 0 ? (
            <EmptyState message="No overdue tasks" />
          ) : (
            <ul className="space-y-2" role="list">
              {overdueTasks.map((task) => (
                <li key={task.id} className="text-warning">
                  <Link to={`/tasks?id=${task.id}`} className="hover:underline">
                    {task.title}
                  </Link>{" "}
                  {task.due_date && (
                    <span className="text-xs text-base-content/70">
                      Due {task.due_date}
                    </span>
                  )}
                </li>
              ))}
            </ul>
          )}
        </Card>

        {/* Balance + Recent transactions */}
        <Card
          title="Balance"
          to="/finance"
          className="md:col-span-2 xl:col-span-1"
        >
          <div className="mb-3 text-2xl font-semibold">
            <MoneyBadge
              cents={dashboard.balance.total.balance_cents}
              currency={dashboard.balance.currency}
            />
          </div>
          {recentTxs.length === 0 ? (
            <EmptyState message="No recent transactions" />
          ) : (
            <ul className="space-y-2" role="list">
              {recentTxs.map((tx) => (
                <li
                  key={tx.id}
                  className="flex items-center justify-between gap-2 text-sm"
                >
                  <span className="truncate">{tx.memo ?? tx.kind}</span>
                  <MoneyBadge
                    cents={tx.amount_cents}
                    currency={tx.currency}
                    sign={
                      tx.kind === "income"
                        ? "+"
                        : tx.kind === "expense"
                          ? "−"
                          : undefined
                    }
                  />
                </li>
              ))}
            </ul>
          )}
        </Card>

        {/* Pinned notes */}
        <Card
          title="Pinned notes"
          to="/notes"
          className="md:col-span-2 xl:col-span-1"
        >
          {pinnedNotes.length === 0 ? (
            <EmptyState message="No pinned notes" />
          ) : (
            <ul className="space-y-2" role="list">
              {pinnedNotes.map((note) => (
                <li key={note.id}>
                  <Link to={`/notes?id=${note.id}`} className="hover:underline">
                    {note.title}
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </Card>
      </div>
    </section>
  );
}

interface CardProps {
  title: string;
  to: string;
  children: React.ReactNode;
  className?: string;
}

// `to` (not `href`): every in-app navigation goes through react-router,
// so a card click never reloads the document — raw <a> was the only one
// in the app and it cost a full page load per click.
function Card({ title, to, children, className = "" }: CardProps) {
  return (
    <article
      className={`card bg-base-200 border border-base-300 p-4 ${className}`}
    >
      <header className="flex items-center justify-between mb-3">
        <h2 className="font-semibold text-base">{title}</h2>
        <Link
          to={to}
          className="text-xs text-primary hover:underline"
          aria-label={`View all ${title.toLowerCase()}`}
        >
          View all →
        </Link>
      </header>
      <div>{children}</div>
    </article>
  );
}
