import { describe, expect, it } from "vitest";
import { sanitizeBody, sanitizeLine } from "./sanitize";

const NUL = String.fromCharCode(0);
const BEL = String.fromCharCode(7);
const ESC = String.fromCharCode(27);
const DEL = String.fromCharCode(127);

describe("sanitizeLine", () => {
  it("keeps plain text intact and trims the edges", () => {
    expect(sanitizeLine("  Dentist  ")).toBe("Dentist");
  });

  it("strips every control character, tabs included", () => {
    expect(sanitizeLine(`Den${NUL}ti${BEL}st\t${ESC}${DEL}`)).toBe("Dentist");
  });

  it("reduces an all-control title to the empty string", () => {
    expect(sanitizeLine(`${NUL}${BEL}\t\n`)).toBe("");
  });
});

describe("sanitizeBody", () => {
  it("keeps newlines and tabs", () => {
    expect(sanitizeBody("line1\nline2\tindented")).toBe(
      "line1\nline2\tindented",
    );
  });

  it("normalizes CRLF and lone CR to LF", () => {
    expect(sanitizeBody("a\r\nb\rc")).toBe("a\nb\nc");
  });

  it("strips other control characters and trims the edges", () => {
    expect(sanitizeBody(`  Bring${NUL}${BEL} card.  `)).toBe("Bring card.");
  });

  it("reduces an all-control body to the empty string", () => {
    expect(sanitizeBody(`${NUL}${BEL}\n\t `)).toBe("");
  });
});
