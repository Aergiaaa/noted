// C0/C1 control characters never belong in user text; titles are single
// line, bodies keep newlines and tabs. The server re-sanitizes on write
// (back/internal/service) — this is the fast client-side path.

export function sanitizeLine(raw: string): string {
  return stripControls(raw).trim();
}

export function sanitizeBody(raw: string): string {
  return stripControls(raw.replace(/\r\n?/g, "\n"), true).trim();
}

function stripControls(raw: string, keepBreaks = false): string {
  let out = "";
  for (const ch of raw) {
    const code = ch.charCodeAt(0);
    if (code <= 0x1f) {
      if (keepBreaks && (code === 0x09 || code === 0x0a)) out += ch;
      continue;
    }
    if (code >= 0x7f && code <= 0x9f) continue;
    out += ch;
  }
  return out;
}
