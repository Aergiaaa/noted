import { useEffect, useState } from "react";
import {
  api,
  type Event,
  type EventsResponse,
  type Note,
  type NotesResponse,
  type Task,
  type TasksResponse,
} from "../../lib/api";

export type LinkKind = "task" | "event" | "note";

// The events endpoint needs a from/to pair (API.md); a wide window covers
// every event a link could ever target without a dedicated all-events mode.
export const EVENTS_WIDE_PATH = `/events?from=${encodeURIComponent("2000-01-01T00:00:00Z")}&to=${encodeURIComponent("2100-01-01T00:00:00Z")}`;

export interface LinkSources {
  tasks: Task[];
  events: Event[];
  notes: Note[];
  loading: boolean;
}

// Fetch the entity lists that pickers and chips resolve titles from.
// Callers pass a module-level kind list (the effect keys on the joined
// string, so inline arrays don't refetch); fetch order is fixed
// task → event → note so call sequences stay deterministic in tests.
export function useLinkSources(kinds: readonly LinkKind[]): LinkSources {
  const key = kinds.join(",");
  const [sources, setSources] = useState<LinkSources>({
    tasks: [],
    events: [],
    notes: [],
    loading: key !== "",
  });

  useEffect(() => {
    if (key === "") return;

    const wanted = new Set(key.split(",") as LinkKind[]);
    let cancelled = false;

    const loaded: LinkSources = {
      tasks: [],
      events: [],
      notes: [],
      loading: false,
    };

    const jobs: Promise<void>[] = [];

    if (wanted.has("task")) {
      jobs.push(
        api<TasksResponse>("/tasks").then((res) => {
          loaded.tasks = res.data;
        }),
      );
    }

    if (wanted.has("event")) {
      jobs.push(
        api<EventsResponse>(EVENTS_WIDE_PATH).then((res) => {
          loaded.events = res.data;
        }),
      );
    }

    if (wanted.has("note")) {
      jobs.push(
        api<NotesResponse>("/notes").then((res) => {
          loaded.notes = res.data;
        }),
      );
    }

    Promise.all(jobs)
      .then(() => {
        if (!cancelled) setSources(loaded);
      })
      .catch(() => {
        if (!cancelled) setSources((prev) => ({ ...prev, loading: false }));
      });
    return () => {
      cancelled = true;
    };
  }, [key]);

  return sources;
}
