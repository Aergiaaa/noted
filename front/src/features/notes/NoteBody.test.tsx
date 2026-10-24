import { afterEach, describe, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { MemoryRouter, useLocation } from "react-router";
import NoteBody from "./NoteBody";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

const TASK = {
  id: "t1",
  title: "Buy keyboard",
  status: "open",
  due_date: null,
  event_id: null,
  note_id: null,
  completed_at: null,
};

function renderBody(body: string, className?: string) {
  return render(
    <MemoryRouter initialEntries={["/notes"]}>
      <NoteBody body={body} className={className} />
    </MemoryRouter>,
  );
}

function LocationProbe() {
  const location = useLocation();
  return <div data-testid="loc">{location.pathname + location.search}</div>;
}

function renderWithProbe(body: string) {
  return render(
    <MemoryRouter initialEntries={["/notes"]}>
      <NoteBody body={body} />
      <LocationProbe />
    </MemoryRouter>,
  );
}

describe("NoteBody", () => {
  it("renders markdown headings and emphasis", () => {
    const { container } = renderBody("# Title\n\nSome **bold** text.");
    expect(container.querySelector("h1")?.textContent).toBe("Title");
    expect(container.querySelector("strong")?.textContent).toBe("bold");
  });

  it("sanitizes scripts, event handlers and javascript: links", () => {
    const { container } = renderBody(
      "<script>window.__pwned = 1</script>\n\n" +
        '<img src="x" onerror="window.__pwned = 2">\n\n' +
        "[click](javascript:window.__pwned=3)",
    );
    expect(container.querySelector("script")).toBeNull();
    expect(container.querySelector("img[onerror]")).toBeNull();
    expect(
      Array.from(container.querySelectorAll("a")).every(
        (a) => !a.getAttribute("href")?.startsWith("javascript:"),
      ),
    ).toBe(true);
  });

  it("renders an empty body as an empty box", () => {
    const { container } = renderBody("");
    expect(container.firstElementChild).not.toBeNull();
    expect(container.textContent).toBe("");
  });

  it("keeps its base classes without an extra class", () => {
    const { container } = renderBody("hi");
    expect(container.firstElementChild?.className).toContain("text-sm");
    expect(container.firstElementChild?.className).not.toContain("undefined");
  });

  it("appends an extra class when one is passed", () => {
    const { container } = renderBody("hi", "line-clamp-3");
    expect(container.firstElementChild?.className).toContain("line-clamp-3");
  });

  it("never fetches while the body has no mention tokens", () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    renderBody("Just some plain notes.");
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("replaces a resolvable token with a live chip around the text", async () => {
    const fetchMock = vi.fn().mockImplementation((input: unknown) => {
      if (String(input).includes("/tasks")) {
        return Promise.resolve(jsonResponse({ data: [TASK] }));
      }
      return Promise.reject(new Error("unexpected fetch"));
    });
    vi.stubGlobal("fetch", fetchMock);
    const { container } = renderBody("**bold** Hello @[task:t1] bye");

    const chip = await screen.findByRole("link", { name: "Buy keyboard" });
    expect(chip.getAttribute("href")).toBe("/tasks?id=t1");
    expect(chip.getAttribute("data-mention")).toBe("true");
    expect(container.textContent?.trim()).toBe("bold Hello Buy keyboard bye");
  });

  it("mutes a mention whose id no longer exists, as plain raw text", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ data: [] }));
    vi.stubGlobal("fetch", fetchMock);
    const { container } = renderBody("Bye @[task:ghost] now");

    const muted = await screen.findByText("@[task:ghost]");
    expect(muted.className).toContain("text-base-content/40");
    expect(container.querySelector("a[data-mention]")).toBeNull();
    expect(container.textContent?.trim()).toBe("Bye @[task:ghost] now");
  });

  it("leaves malformed tokens as plain text and never fetches", () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    const { container } = renderBody("See @[bogus:x] and @[task:] done");

    expect(container.textContent?.trim()).toBe(
      "See @[bogus:x] and @[task:] done",
    );
    expect(container.querySelector("a[data-mention]")).toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("keeps the token as plain text while the titles are still loading", () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(() => new Promise(() => {})),
    );
    const { container } = renderBody("Hi @[task:t1]");

    expect(container.textContent).toContain("@[task:t1]");
    expect(container.querySelector("a[data-mention]")).toBeNull();
  });

  it("navigates to the module on chip click and ignores plain-text clicks", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(jsonResponse({ data: [TASK] })),
    );
    const { container } = renderWithProbe("Hi @[task:t1]");

    const chip = await screen.findByRole("link", { name: "Buy keyboard" });
    fireEvent.click(container.querySelector("p") as HTMLElement);
    expect(screen.getByTestId("loc").textContent).toBe("/notes");

    fireEvent.click(chip);
    expect(screen.getByTestId("loc").textContent).toBe("/tasks?id=t1");
  });

  it("rebuilds the chips when the body itself changes", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(jsonResponse({ data: [TASK] })),
    );
    const { container, rerender } = renderBody("Hi @[task:t1]");
    await screen.findByRole("link", { name: "Buy keyboard" });

    rerender(
      <MemoryRouter initialEntries={["/notes"]}>
        <NoteBody body="Hi @[task:t1] and more" />
      </MemoryRouter>,
    );
    await waitFor(() =>
      expect(container.textContent?.trim()).toBe("Hi Buy keyboard and more"),
    );
    expect(container.querySelectorAll("a[data-mention]")).toHaveLength(1);
  });
});
