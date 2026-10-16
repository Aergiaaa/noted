import { afterEach, describe, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import type { Note } from "../../lib/api";
import NoteForm from "./NoteForm";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

const NOTE: Note = {
  id: "n1",
  title: "Groceries",
  body_md: "buy milk",
  pinned: true,
};

function renderForm(note: Note | null) {
  const onClose = vi.fn();
  const onSaved = vi.fn();
  const onDeleted = vi.fn();
  const view = render(
    <NoteForm
      note={note}
      onClose={onClose}
      onSaved={onSaved}
      onDeleted={onDeleted}
    />,
  );
  return { ...view, onClose, onSaved, onDeleted };
}

describe("NoteForm", () => {
  it("creates a note with the title, body and pinned flag", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(201, {}));
    vi.stubGlobal("fetch", fetchMock);
    const { onClose, onSaved } = renderForm(null);

    fireEvent.change(screen.getByLabelText("Title"), {
      target: { value: "  Idea  " },
    });
    fireEvent.change(screen.getByLabelText("Body"), {
      target: { value: "# hello" },
    });
    fireEvent.click(screen.getByLabelText("Pinned"));
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(onSaved).toHaveBeenCalled());
    expect(onClose).toHaveBeenCalled();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/notes");
    expect(fetchMock.mock.calls[0]?.[1]).toMatchObject({
      method: "POST",
      body: JSON.stringify({ title: "Idea", body_md: "# hello", pinned: true }),
    });
  });

  it("prefills the form and PATCHes an edited note", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, {}));
    vi.stubGlobal("fetch", fetchMock);
    renderForm(NOTE);

    expect(screen.getByLabelText("Title").getAttribute("value")).toBe(
      "Groceries",
    );
    expect((screen.getByLabelText("Body") as HTMLTextAreaElement).value).toBe(
      "buy milk",
    );

    fireEvent.click(screen.getByLabelText("Pinned"));
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/notes/n1");
    expect(fetchMock.mock.calls[0]?.[1]).toMatchObject({
      method: "PATCH",
      body: JSON.stringify({
        title: "Groceries",
        body_md: "buy milk",
        pinned: false,
      }),
    });
  });

  it("rejects an empty title before touching the network", () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    renderForm(null);

    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(screen.getByText("Title is required.")).toBeDefined();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("shows server validation fields after a failed save", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        jsonResponse(400, {
          error: {
            code: "validation_error",
            message: "validation failed",
            fields: { body_md: "too long" },
          },
        }),
      ),
    );
    renderForm(null);

    fireEvent.change(screen.getByLabelText("Title"), {
      target: { value: "x" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() =>
      expect(screen.getByText("body_md: too long")).toBeDefined(),
    );
  });

  it("deletes after an explicit confirmation", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);
    const { onClose, onDeleted } = renderForm(NOTE);

    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    expect(screen.getByText("Delete this note?")).toBeDefined();

    fireEvent.click(screen.getByRole("button", { name: "Keep it" }));
    expect(screen.queryByText("Delete this note?")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    fireEvent.click(screen.getByRole("button", { name: "Yes, delete" }));

    await waitFor(() => expect(onDeleted).toHaveBeenCalledWith("n1"));
    expect(onClose).toHaveBeenCalled();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0]?.[1]).toMatchObject({ method: "DELETE" });
  });

  it("shows an error and returns to the buttons when delete fails", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        jsonResponse(500, {
          error: { code: "internal_error", message: "server exploded" },
        }),
      ),
    );
    renderForm(NOTE);

    fireEvent.click(screen.getByRole("button", { name: "Delete" }));
    fireEvent.click(screen.getByRole("button", { name: "Yes, delete" }));

    await waitFor(() =>
      expect(screen.getByText("server exploded")).toBeDefined(),
    );
    expect(screen.queryByText("Delete this note?")).toBeNull();
    expect(screen.getByRole("button", { name: "Delete" })).toBeDefined();
  });

  it("closes on cancel, backdrop click and Escape", () => {
    const { container, onClose } = renderForm(null);

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onClose).toHaveBeenCalledTimes(1);

    const backdrop = container.firstElementChild as HTMLElement;
    fireEvent.click(backdrop);
    expect(onClose).toHaveBeenCalledTimes(2);

    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(onClose).toHaveBeenCalledTimes(2);

    fireEvent.keyDown(window, { key: "Tab" });
    expect(onClose).toHaveBeenCalledTimes(2);

    fireEvent.keyDown(window, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(3);
  });
});
