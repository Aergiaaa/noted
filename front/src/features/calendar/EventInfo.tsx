import { useMemo } from "react";
import { Link } from "react-router";
import type { Event } from "../../lib/api";
import { formatDateTime, formatTime, toLocalInput } from "../../lib/format";
import {
  findMentions,
  mentionRoute,
  resolveMentions,
  type MentionSegment,
} from "../../lib/mentions";
import { createLinkTitleMap } from "../../lib/selectors";
import { eventColorDotClasses } from "./eventColor";
import Modal from "../../components/ui/Modal";
import LinkChip from "../links/LinkChip";
import { useLinkSources, type LinkSources } from "../links/useLinkSources";

interface EventInfoProps {
  event: Event;
  onClose: () => void;
  onEdit: (event: Event) => void;
}

// Read-only details card between clicking a block and editing, so the
// description and full span are visible before any change. It reads the
// event already fetched with the week — no extra request. Escape and
// backdrop close come from Modal's focus trap.
export default function EventInfo({ event, onClose, onEdit }: EventInfoProps) {
  const description = event.description?.trim();
  // Lazy: fetches only when the description actually holds tokens (F9).
  const mentionKinds = useMemo(
    () => [
      ...new Set(findMentions(description ?? "").map((token) => token.type)),
    ],
    [description],
  );
  const sources = useLinkSources(mentionKinds);

  return (
    <Modal onClose={onClose}>
      <div className="flex items-center gap-2">
        <span
          data-testid="event-color-dot"
          aria-hidden="true"
          className={`inline-block h-2 w-2 shrink-0 rounded-full ${eventColorDotClasses(event.id)}`}
        />
        <h2 className="break-words text-base font-semibold">{event.title}</h2>
      </div>
      <p className="mt-2 text-sm tabular-nums text-base-content/70">
        {formatEventRange(event.starts_at, event.ends_at)}
      </p>
      <div className="mt-3">
        <span className="label-text text-xs uppercase text-base-content/60">
          Description
        </span>
        {description ? (
          <DescriptionText description={description} sources={sources} />
        ) : (
          <p className="mt-1 text-sm text-base-content/50">No description.</p>
        )}
      </div>
      {/* Event.note_id is optional-typed (api.ts) — loose null check
          narrows both null and undefined. */}
      {event.note_id != null && (
        <div className="mt-3 flex flex-wrap gap-1">
          {/* Generic label: the calendar page has no notes list to title
              from (F9 "otherwise" branch) — the link still lands. */}
          <LinkChip kind="note" id={event.note_id} />
        </div>
      )}
      <div className="mt-4 flex justify-end gap-2">
        <button
          type="button"
          className="btn btn-ghost btn-sm"
          onClick={onClose}
        >
          Close
        </button>
        <button
          type="button"
          className="btn btn-primary btn-sm"
          onClick={() => onEdit(event)}
        >
          Edit
        </button>
      </div>
    </Modal>
  );
}

// Prose with mention tokens resolved to chips. While the title lists are
// still loading the tokens stay plain raw text — they become chips or
// (dead id) muted raw text once known, same as NoteBody's two-phase walk.
function DescriptionText({
  description,
  sources,
}: {
  description: string;
  sources: LinkSources;
}) {
  const titles = createLinkTitleMap(sources);
  if (sources.loading) {
    return <p className="mt-1 whitespace-pre-wrap text-sm">{description}</p>;
  }
  return (
    <p className="mt-1 whitespace-pre-wrap text-sm">
      {resolveMentions(description, titles).map((segment, index) => (
        <MentionSegmentView key={index} segment={segment} />
      ))}
    </p>
  );
}

function MentionSegmentView({ segment }: { segment: MentionSegment }) {
  if (segment.kind === "text") return <>{segment.text}</>;
  if (segment.title === null) {
    return <span className="text-base-content/40">{segment.raw}</span>;
  }
  return (
    <Link
      className="badge badge-info badge-sm"
      to={mentionRoute(segment.type, segment.id)}
    >
      {segment.title}
    </Link>
  );
}

function formatEventRange(starts: string, ends: string): string {
  const start = new Date(starts);
  const end = new Date(ends);
  const sameDay = start.toDateString() === end.toDateString();
  return sameDay
    ? `${fullWhen(starts)} – ${formatTime(ends)}`
    : `${fullWhen(starts)} – ${fullWhen(ends)}`;
}

function fullWhen(iso: string): string {
  return formatDateTime(toLocalInput(iso));
}
