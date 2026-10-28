import { afterEach, describe, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render as baseRender,
  screen,
} from "@testing-library/react";
import { MemoryRouter } from "react-router";
import type { ReactElement } from "react";
import type { Note } from "../../lib/api";
import NoteList from "./NoteList";

afterEach(() => {
  cleanup();
});

// Rows embed NoteBody, which needs a router context for mention clicks.
function render(ui: ReactElement) {
  return baseRender(<MemoryRouter>{ui}</MemoryRouter>);
}

const NOTES: Note[] = [
  { id: "n1", title: "First", body_md: "one", pinned: true },
  { id: "n2", title: "Second", body_md: "two", pinned: false },
];

describe("NoteList", () => {
  it("renders one row per note", () => {
    render(<NoteList notes={NOTES} onOpen={vi.fn()} onTogglePin={vi.fn()} />);
    expect(screen.getByRole("button", { name: "First" })).toBeDefined();
    expect(screen.getByRole("button", { name: "Second" })).toBeDefined();
  });

  it("routes open and pin actions to the clicked note", () => {
    const onOpen = vi.fn();
    const onTogglePin = vi.fn();
    render(
      <NoteList notes={NOTES} onOpen={onOpen} onTogglePin={onTogglePin} />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Second" }));
    expect(onOpen).toHaveBeenCalledWith(NOTES[1]);

    fireEvent.click(screen.getByRole("button", { name: "Pin" }));
    expect(onTogglePin).toHaveBeenCalledWith(NOTES[1]);
  });

  it("renders no rows for an empty list", () => {
    render(<NoteList notes={[]} onOpen={vi.fn()} onTogglePin={vi.fn()} />);
    expect(screen.queryAllByRole("listitem")).toHaveLength(0);
  });
});
