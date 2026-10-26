import { describe, expect, it } from "vitest";
import {
  eventColorClasses,
  eventColorDotClasses,
  eventColorIndex,
} from "./eventColor";

describe("eventColorIndex", () => {
  it("is stable for the same id", () => {
    expect(eventColorIndex("e1")).toBe(eventColorIndex("e1"));
  });

  it("returns an index inside the palette", () => {
    for (const id of ["", "e1", "wal_cash", "x".repeat(100)]) {
      const index = eventColorIndex(id);
      expect(index).toBeGreaterThanOrEqual(0);
      expect(index).toBeLessThan(6);
    }
  });

  it("spreads ids across every palette slot", () => {
    const seen = new Set<number>();
    for (let i = 0; i < 200; i++) seen.add(eventColorIndex(`e${i}`));
    expect(seen.size).toBe(6);
  });
});

describe("eventColorClasses", () => {
  it("returns the palette's border, fill, and hover classes", () => {
    const classes = eventColorClasses("e1");
    expect(classes.split(" ")).toHaveLength(3);
    expect(classes).toMatch(/border-\S+\/40 bg-\S+\/20 hover:bg-\S+\/30/);
  });

  it("returns a different class set for a different slot", () => {
    const ids = Array.from({ length: 50 }, (_, i) => `e${i}`);
    const first = eventColorClasses(ids[0] as string);
    const other = ids.find((id) => eventColorClasses(id) !== first);
    expect(other).toBeDefined();
  });
});

describe("eventColorDotClasses", () => {
  it("returns a single background token for dots", () => {
    expect(eventColorDotClasses("e1")).toMatch(/^bg-\S+$/);
  });
});
