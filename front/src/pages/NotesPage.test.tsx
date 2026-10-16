import { afterEach, describe, expect, it, vi } from "vitest";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import NotesPage from "./NotesPage";
import type { Note } from "../lib/api";

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function listResponse(notes: Note[]) {
  return jsonResponse(200, { data: notes });
}

function note(over: Partial<Note> = {}): Note {
  return {
    id: "n1",
    title: "Groceries",
    body_md: "buy milk",
    pinned: false,
    ...over,
  };
}

describe("NotesPage", () => {
  it("shows a skeleton while notes load, then the rows", async () => {
    let resolveList!: (r: Response) => void;
    const fetchMock = vi.fn().mockImplementationOnce(
      () =>
        new Promise<Response>((res) => {
          resolveList = res;
        }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const { container } = render(<NotesPage />);
    expect(screen.getByRole("heading", { name: "Notes" })).toBeDefined();
    expect(container.querySelector(".skeleton")).not.toBeNull();

    await act(async () => {
      resolveList(listResponse([note({ title: "Groceries" })]));
    });

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Groceries" })).toBeDefined(),
    );
    expect(container.querySelector(".skeleton")).toBeNull();
    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/notes");
  });

  it("renders an error and retries on failure", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response("<html>boom</html>", { status: 200 }))
      .mockResolvedValueOnce(listResponse([note()]));
    vi.stubGlobal("fetch", fetchMock);

    render(<NotesPage />);
    await waitFor(() => expect(screen.getByRole("alert")).toBeDefined());
    expect(screen.getByText("Couldn't load notes.")).toBeDefined();

    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Groceries" })).toBeDefined(),
    );
    expect(fetchMock.mock.calls[1]?.[0]).toBe("/api/notes");
  });

  it("offers an empty state whose CTA opens the create form", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(listResponse([])));

    render(<NotesPage />);
    await waitFor(() =>
      expect(screen.getByText("No notes yet.")).toBeDefined(),
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Create your first note" }),
    );
    expect(screen.getByRole("heading", { name: "New note" })).toBeDefined();
  });

  it("pin toggle PATCHes the note and refreshes the list", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(listResponse([note()]))
      .mockResolvedValueOnce(jsonResponse(200, {}))
      .mockResolvedValueOnce(listResponse([note({ pinned: true })]));
    vi.stubGlobal("fetch", fetchMock);

    render(<NotesPage />);
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Pin" })).toBeDefined(),
    );

    fireEvent.click(screen.getByRole("button", { name: "Pin" }));

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    expect(fetchMock.mock.calls[1]?.[0]).toBe("/api/notes/n1");
    expect(fetchMock.mock.calls[1]?.[1]).toMatchObject({
      method: "PATCH",
      body: JSON.stringify({ pinned: true }),
    });
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(3));
  });

  it("surfaces a failed pin toggle as an error", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(listResponse([note()]))
      .mockResolvedValueOnce(
        new Response("<html>boom</html>", { status: 200 }),
      );
    vi.stubGlobal("fetch", fetchMock);

    render(<NotesPage />);
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Pin" })).toBeDefined(),
    );

    fireEvent.click(screen.getByRole("button", { name: "Pin" }));

    await waitFor(() => expect(screen.getByRole("alert")).toBeDefined());
    expect(screen.getByText("Couldn't update the note.")).toBeDefined();
  });

  it("opens the edit form from a row and refetches after saving", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(listResponse([note()]))
      .mockResolvedValueOnce(jsonResponse(200, {}))
      .mockResolvedValueOnce(listResponse([note()]));
    vi.stubGlobal("fetch", fetchMock);

    render(<NotesPage />);
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Groceries" })).toBeDefined(),
    );

    fireEvent.click(screen.getByRole("button", { name: "Groceries" }));
    expect(screen.getByRole("heading", { name: "Edit note" })).toBeDefined();

    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    expect(fetchMock.mock.calls[1]?.[1]).toMatchObject({ method: "PATCH" });
  });

  it("opens the create form from the header button", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(listResponse([])));

    render(<NotesPage />);
    await waitFor(() =>
      expect(screen.getByText("No notes yet.")).toBeDefined(),
    );

    fireEvent.click(screen.getByRole("button", { name: "New note" }));
    expect(screen.getByRole("heading", { name: "New note" })).toBeDefined();
  });

  it("ignores results that land after unmount", async () => {
    let resolveList!: (r: Response) => void;
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementationOnce(
        () =>
          new Promise<Response>((res) => {
            resolveList = res;
          }),
      ),
    );
    const first = render(<NotesPage />);
    first.unmount();
    await act(async () => {
      resolveList(listResponse([note()]));
    });

    let rejectFetch!: (e: unknown) => void;
    vi.stubGlobal(
      "fetch",
      vi.fn().mockImplementationOnce(
        () =>
          new Promise<Response>((_, rej) => {
            rejectFetch = rej;
          }),
      ),
    );
    const second = render(<NotesPage />);
    second.unmount();
    await act(async () => {
      rejectFetch(new TypeError("too late"));
    });
  });
});
