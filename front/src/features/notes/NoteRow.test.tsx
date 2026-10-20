import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import type { Note } from "../../lib/api";
import NoteRow from "./NoteRow";

afterEach(() => {
  cleanup();
});

function note(over: Partial<Note> = {}): Note {
  return {
    id: "n1",
    title: "Groceries",
    body_md: "buy **milk**",
    pinned: false,
    ...over,
  };
}

describe("NoteRow", () => {
  it("opens the note when the title is clicked", () => {
    const onOpen = vi.fn();
    const target = note();
    render(<NoteRow note={target} onOpen={onOpen} onTogglePin={vi.fn()} />);

    fireEvent.click(screen.getByRole("button", { name: "Groceries" }));
    expect(onOpen).toHaveBeenCalledWith(target);
  });

  it("opens the note from the Edit button", () => {
    const onOpen = vi.fn();
    const target = note();
    render(<NoteRow note={target} onOpen={onOpen} onTogglePin={vi.fn()} />);

    fireEvent.click(screen.getByRole("button", { name: "Edit" }));
    expect(onOpen).toHaveBeenCalledWith(target);
  });

  it("toggles pin state from the row button", () => {
    const onTogglePin = vi.fn();
    const target = note({ pinned: false });
    render(
      <NoteRow note={target} onOpen={vi.fn()} onTogglePin={onTogglePin} />,
    );

    const pin = screen.getByRole("button", { name: "Pin" });
    expect(pin.getAttribute("aria-pressed")).toBe("false");
    fireEvent.click(pin);
    expect(onTogglePin).toHaveBeenCalledWith(target);
  });

  it("shows Unpin for a pinned note", () => {
    render(
      <NoteRow
        note={note({ pinned: true })}
        onOpen={vi.fn()}
        onTogglePin={vi.fn()}
      />,
    );
    const pin = screen.getByRole("button", { name: "Unpin" });
    expect(pin.getAttribute("aria-pressed")).toBe("true");
  });

  it("renders the body through the markdown renderer", () => {
    const { container } = render(
      <NoteRow note={note()} onOpen={vi.fn()} onTogglePin={vi.fn()} />,
    );
    expect(container.querySelector("strong")?.textContent).toBe("milk");
  });
});
