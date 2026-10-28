import { describe, expect, it } from "vitest";
import {
  findMentions,
  insertMention,
  mentionRoute,
  resolveMentions,
  type MentionSegment,
} from "./mentions";

describe("findMentions", () => {
  it("extracts a single well-formed token with its span", () => {
    const text = "see @[task:t_1] for details";
    expect(findMentions(text)).toEqual([
      { type: "task", id: "t_1", raw: "@[task:t_1]", index: 4 },
    ]);
  });

  it("extracts every token in order across lines", () => {
    const text = "@[event:e9]\nand @[note:n9] too";
    expect(findMentions(text)).toEqual([
      { type: "event", id: "e9", raw: "@[event:e9]", index: 0 },
      { type: "note", id: "n9", raw: "@[note:n9]", index: 16 },
    ]);
  });

  it("ignores malformed tokens so they stay plain text", () => {
    expect(findMentions("bad @[task:] and @[widget:x] and @[task]")).toEqual(
      [],
    );
  });

  it("ignores an empty body", () => {
    expect(findMentions("")).toEqual([]);
  });
});

describe("insertMention", () => {
  it("drops the token at the cursor and places the cursor after it", () => {
    expect(insertMention("buy milk", 8, "task", "t1")).toEqual({
      text: "buy milk@[task:t1]",
      cursor: 18,
    });
  });

  it("splices into the middle without eating surrounding text", () => {
    expect(insertMention("see  now", 4, "event", "e9")).toEqual({
      text: "see @[event:e9] now",
      cursor: 15,
    });
  });

  it("replaces a selection when one is given", () => {
    expect(
      insertMention("old text", 0, "note", "n1", { start: 0, end: 8 }),
    ).toEqual({
      text: "@[note:n1]",
      cursor: 10,
    });
  });
});

describe("mentionRoute", () => {
  it("maps each type to its module deep link", () => {
    expect(mentionRoute("task", "t1")).toBe("/tasks?id=t1");
    expect(mentionRoute("event", "e9")).toBe("/calendar?event=e9");
    expect(mentionRoute("note", "n9")).toBe("/notes?id=n9");
  });
});

describe("resolveMentions", () => {
  const titles = new Map([
    ["task:t1", "Buy keyboard"],
    ["event:e9", "Dentist"],
  ]);

  it("splits plain text around a live mention and fills its title", () => {
    const segments = resolveMentions("see @[task:t1] ok", titles);
    expect(segments).toEqual([
      { kind: "text", text: "see " },
      {
        kind: "mention",
        type: "task",
        id: "t1",
        raw: "@[task:t1]",
        title: "Buy keyboard",
      },
      { kind: "text", text: " ok" },
    ] satisfies MentionSegment[]);
  });

  it("marks a dead id muted with no title", () => {
    const segments = resolveMentions("@[note:gone]", titles);
    expect(segments).toEqual([
      {
        kind: "mention",
        type: "note",
        id: "gone",
        raw: "@[note:gone]",
        title: null,
      },
    ]);
  });

  it("leaves malformed tokens in a plain-text segment", () => {
    expect(resolveMentions("bad @[task:] here", titles)).toEqual([
      { kind: "text", text: "bad @[task:] here" },
    ]);
  });

  it("returns one text segment for a body with no tokens", () => {
    expect(resolveMentions("plain", titles)).toEqual([
      { kind: "text", text: "plain" },
    ]);
  });
});
