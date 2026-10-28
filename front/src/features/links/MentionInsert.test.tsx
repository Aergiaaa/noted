import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { useRef, useState } from "react";
import MentionInsert from "./MentionInsert";

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

function listFetcher() {
  const spy = vi.fn((input: unknown) => {
    const url = String(input);
    if (url.includes("/tasks")) {
      return Promise.resolve(jsonResponse({ data: [TASK] }));
    }
    if (url.includes("/events")) {
      return Promise.resolve(
        jsonResponse({
          data: [{ id: "e9", title: "Dentist", starts_at: "", ends_at: "" }],
        }),
      );
    }
    if (url.includes("/notes")) {
      return Promise.resolve(
        jsonResponse({
          data: [{ id: "n9", title: "Groceries", body_md: "", pinned: false }],
        }),
      );
    }
    return Promise.reject(new Error(`unexpected ${url}`));
  });
  vi.stubGlobal("fetch", spy);
  return spy;
}

function Harness() {
  const [text, setText] = useState("Hello  world");
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  return (
    <>
      <textarea
        aria-label="Text"
        ref={textareaRef}
        value={text}
        onChange={(event) => setText(event.target.value)}
      />
      <MentionInsert
        text={text}
        onTextChange={setText}
        textareaRef={textareaRef}
      />
    </>
  );
}

describe("MentionInsert", () => {
  it("fetches nothing until the Mention button is clicked", () => {
    const fetchMock = listFetcher();
    render(<Harness />);
    expect(screen.queryByLabelText("Insert mention")).toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("stays disabled with a Loading option while the catalogs fetch", () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(() => new Promise(() => {})),
    );
    render(<Harness />);
    fireEvent.click(screen.getByRole("button", { name: "Mention" }));

    const select = screen.getByLabelText("Insert mention") as HTMLSelectElement;
    expect(select.disabled).toBe(true);
    expect(screen.getByRole("option", { name: "Loading…" })).toBeDefined();
  });

  it("lists a placeholder then one titled option per entity", async () => {
    listFetcher();
    render(<Harness />);
    fireEvent.click(screen.getByRole("button", { name: "Mention" }));

    await screen.findByRole("option", { name: "Buy keyboard" });
    const options = screen.getAllByRole("option");
    expect(options.map((option) => option.textContent)).toEqual([
      "Mention…",
      "Buy keyboard",
      "Dentist",
      "Groceries",
    ]);
    expect(
      (screen.getByLabelText("Insert mention") as HTMLSelectElement).disabled,
    ).toBe(false);
  });

  it("inserts the picked token at the caret and restores the caret", async () => {
    listFetcher();
    render(<Harness />);
    const area = screen.getByLabelText("Text") as HTMLTextAreaElement;
    area.setSelectionRange(6, 6);

    fireEvent.click(screen.getByRole("button", { name: "Mention" }));
    await screen.findByRole("option", { name: "Buy keyboard" });
    fireEvent.change(screen.getByLabelText("Insert mention"), {
      target: { value: "task:t1" },
    });

    expect(area.value).toBe("Hello @[task:t1] world");
    expect(area.selectionStart).toBe(6 + "@[task:t1]".length);
    expect(document.activeElement).toBe(area);
  });

  it("closes the picker when the button is clicked again", async () => {
    listFetcher();
    render(<Harness />);
    fireEvent.click(screen.getByRole("button", { name: "Mention" }));
    await screen.findByRole("option", { name: "Buy keyboard" });

    fireEvent.click(screen.getByRole("button", { name: "Mention" }));
    expect(screen.queryByLabelText("Insert mention")).toBeNull();
  });

  it("resets to the placeholder so the same item can be picked twice", async () => {
    listFetcher();
    render(<Harness />);
    const area = screen.getByLabelText("Text") as HTMLTextAreaElement;
    area.setSelectionRange(area.value.length, area.value.length);

    fireEvent.click(screen.getByRole("button", { name: "Mention" }));
    await screen.findByRole("option", { name: "Buy keyboard" });
    const select = screen.getByLabelText("Insert mention") as HTMLSelectElement;

    fireEvent.change(select, { target: { value: "task:t1" } });
    expect(select.value).toBe("");
    fireEvent.change(select, { target: { value: "task:t1" } });

    expect(area.value).toBe("Hello  world@[task:t1]@[task:t1]");
  });
});
