import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render } from "@testing-library/react";
import NoteBody from "./NoteBody";

afterEach(() => {
  cleanup();
});

describe("NoteBody", () => {
  it("renders markdown headings and emphasis", () => {
    const { container } = render(
      <NoteBody body={"# Title\n\nSome **bold** text."} />,
    );
    expect(container.querySelector("h1")?.textContent).toBe("Title");
    expect(container.querySelector("strong")?.textContent).toBe("bold");
  });

  it("sanitizes scripts, event handlers and javascript: links", () => {
    const { container } = render(
      <NoteBody
        body={
          "<script>window.__pwned = 1</script>\n\n" +
          '<img src="x" onerror="window.__pwned = 2">\n\n' +
          "[click](javascript:window.__pwned=3)"
        }
      />,
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
    const { container } = render(<NoteBody body="" />);
    expect(container.firstElementChild).not.toBeNull();
    expect(container.textContent).toBe("");
  });

  it("keeps its base classes without an extra class", () => {
    const { container } = render(<NoteBody body="hi" />);
    expect(container.firstElementChild?.className).toContain("text-sm");
    expect(container.firstElementChild?.className).not.toContain("undefined");
  });

  it("appends an extra class when one is passed", () => {
    const { container } = render(
      <NoteBody body="hi" className="line-clamp-3" />,
    );
    expect(container.firstElementChild?.className).toContain("line-clamp-3");
  });
});
