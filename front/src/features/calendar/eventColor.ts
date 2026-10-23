// Stable per-event colors, derived from a hash of the event id — no
// schema, no picker (owner decision: auto-assign over a stored column).
// Six DaisyUI semantic tokens, light enough for the /20 fill on the dark
// theme; error/red is deliberately out (it reads as alarm). Every class
// string is a literal — Tailwind can't see interpolated names (same
// reason the grid uses a literal grid-cols).
const EVENT_COLORS = [
  {
    block: "border-primary/40 bg-primary/20 hover:bg-primary/30",
    dot: "bg-primary",
  },
  {
    block: "border-secondary/40 bg-secondary/20 hover:bg-secondary/30",
    dot: "bg-secondary",
  },
  {
    block: "border-accent/40 bg-accent/20 hover:bg-accent/30",
    dot: "bg-accent",
  },
  {
    block: "border-info/40 bg-info/20 hover:bg-info/30",
    dot: "bg-info",
  },
  {
    block: "border-success/40 bg-success/20 hover:bg-success/30",
    dot: "bg-success",
  },
  {
    block: "border-warning/40 bg-warning/20 hover:bg-warning/30",
    dot: "bg-warning",
  },
] as const;

// djb2 over the id bytes: tiny, stable across sessions and refetches,
// and well-spread for ULID-ish ids.
export function eventColorIndex(id: string): number {
  let hash = 5381;
  for (let i = 0; i < id.length; i++) {
    hash = ((hash << 5) + hash + id.charCodeAt(i)) | 0;
  }
  return Math.abs(hash) % EVENT_COLORS.length;
}

export function eventColorClasses(id: string): string {
  return EVENT_COLORS[eventColorIndex(id)].block;
}

export function eventColorDotClasses(id: string): string {
  return EVENT_COLORS[eventColorIndex(id)].dot;
}
