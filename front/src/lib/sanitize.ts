// C0/C1 control characters never belong in user text; titles are single
// line, bodies keep newlines and tabs. The server re-sanitizes on write
// (back/internal/service) — this is the fast client-side path.

// Unicode control ranges: C0 = U+0000..U+001F, DEL + C1 = U+007F..U+009F.
const C0_MAX = 0x1f;
const DEL_AND_C1_MIN = 0x7f;
const C1_MAX = 0x9f;

// The only controls bodies keep: horizontal tab + line feed.
const TAB = 0x09;
const LF = 0x0a;

export function sanitizeLine(raw: string): string {
  return stripControls(raw).trim();
}

export function sanitizeBody(raw: string): string {
  // CRLF and lone CR normalize to LF before stripping (Windows bodies).
  return stripControls(raw.replace(/\r\n?/g, "\n"), true).trim();
}

function stripControls(raw: string, keepBreaks = false): string {
  let out = "";
  for (const ch of raw) {
    const code = ch.charCodeAt(0);
    if (code <= C0_MAX) {
      if (keepBreaks && (code === TAB || code === LF)) out += ch;
      continue;
    }
    if (code >= DEL_AND_C1_MIN && code <= C1_MAX) continue;
    out += ch;
  }
  return out;
}
