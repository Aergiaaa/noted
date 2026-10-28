// Inline mention tokens (DOMAIN.md D8): @[task:ID] / @[event:ID] /
// @[note:ID] inside note body_md and event description. Opaque plain text
// to the server — this module owns the grammar, the insert helper, and the
// render-time resolution to titles. Pure: no DOM, no fetch.

export type MentionType = "task" | "event" | "note";

export interface MentionToken {
  type: MentionType;
  id: string;
  raw: string;
  index: number;
}

export type MentionSegment =
  | { kind: "text"; text: string }
  | {
      kind: "mention";
      type: MentionType;
      id: string;
      raw: string;
      // null = dead/deleted id — the renderer mutes it (FEATURES F9).
      title: string | null;
    };

// ID charset: ULID (Crockford base32) and UUIDv4 (hex + hyphens) both fit;
// a lone bad character fails the whole token so it stays plain text.
const MENTION_RE = /@\[(task|event|note):([A-Za-z0-9_-]+)\]/g;

export function findMentions(text: string): MentionToken[] {
  const tokens: MentionToken[] = [];
  for (const match of text.matchAll(MENTION_RE)) {
    const [raw, type, id] = match;
    tokens.push({
      type: type as MentionType,
      id,
      raw,
      index: match.index,
    });
  }
  return tokens;
}

export function insertMention(
  text: string,
  cursor: number,
  type: MentionType,
  id: string,
  selection?: { start: number; end: number },
): { text: string; cursor: number } {
  const start = selection?.start ?? cursor;
  const end = selection?.end ?? cursor;
  const token = `@[${type}:${id}]`;
  return {
    text: text.slice(0, start) + token + text.slice(end),
    cursor: start + token.length,
  };
}

export function mentionRoute(type: MentionType, id: string): string {
  if (type === "task") return `/tasks?id=${encodeURIComponent(id)}`;
  if (type === "event") return `/calendar?event=${encodeURIComponent(id)}`;
  return `/notes?id=${encodeURIComponent(id)}`;
}

// Split prose into plain runs and mention spans; titles come from a
// `type:id` → title map the caller built (lazy-fetched only when a token
// is present). A missing entry means dead id → title null.
export function resolveMentions(
  text: string,
  titles: Map<string, string>,
): MentionSegment[] {
  const segments: MentionSegment[] = [];
  let last = 0;
  for (const token of findMentions(text)) {
    if (token.index > last) {
      segments.push({ kind: "text", text: text.slice(last, token.index) });
    }
    const title = titles.get(`${token.type}:${token.id}`) ?? null;
    segments.push({
      kind: "mention",
      type: token.type,
      id: token.id,
      raw: token.raw,
      title,
    });
    last = token.index + token.raw.length;
  }
  if (last < text.length) {
    segments.push({ kind: "text", text: text.slice(last) });
  }
  return segments;
}
