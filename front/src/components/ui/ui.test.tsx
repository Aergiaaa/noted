import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
} from "@testing-library/react";
import EmptyState from "./EmptyState";
import ErrorState from "./ErrorState";
import Modal from "./Modal";
import Toast from "./Toast";

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("Toast", () => {
  it("renders the message and runs the action on click", () => {
    const onAction = vi.fn();
    render(
      <Toast
        toast={{
          message: "Event deleted",
          kind: "success",
          action: { label: "Undo", onAction },
        }}
        onDismiss={vi.fn()}
      />,
    );

    expect(screen.getByText("Event deleted")).toBeDefined();
    fireEvent.click(screen.getByRole("button", { name: "Undo" }));
    expect(onAction).toHaveBeenCalledOnce();
  });

  it("auto-dismisses after 4 seconds", () => {
    vi.useFakeTimers();
    const onDismiss = vi.fn();
    render(
      <Toast
        toast={{ message: "Event deleted", kind: "success" }}
        onDismiss={onDismiss}
      />,
    );

    act(() => {
      vi.advanceTimersByTime(3999);
    });
    expect(onDismiss).not.toHaveBeenCalled();
    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(onDismiss).toHaveBeenCalledOnce();
  });

  it("dismisses on the close button", () => {
    const onDismiss = vi.fn();
    render(
      <Toast
        toast={{ message: "Event deleted", kind: "success" }}
        onDismiss={onDismiss}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Dismiss" }));
    expect(onDismiss).toHaveBeenCalledOnce();
  });

  it("renders the error variant without an action", () => {
    render(
      <Toast
        toast={{ message: "Couldn't restore the event.", kind: "error" }}
        onDismiss={vi.fn()}
      />,
    );

    expect(screen.getByText("Couldn't restore the event.")).toBeDefined();
    expect(screen.queryByRole("button", { name: "Undo" })).toBeNull();
  });
});

describe("EmptyState", () => {
  it("renders the line and a call to action", () => {
    const onCreate = vi.fn();
    render(
      <EmptyState
        message="No notes yet"
        action={{ label: "New note", onClick: onCreate }}
      />,
    );

    expect(screen.getByText("No notes yet")).toBeDefined();
    fireEvent.click(screen.getByRole("button", { name: "New note" }));
    expect(onCreate).toHaveBeenCalledOnce();
  });

  it("omits the button when there is no action", () => {
    render(<EmptyState message="Nothing here" />);
    expect(screen.getByText("Nothing here")).toBeDefined();
    expect(screen.queryByRole("button")).toBeNull();
  });
});

describe("ErrorState", () => {
  it("renders the message and retries on click", () => {
    const retry = vi.fn();
    render(<ErrorState message="Could not load tasks" onRetry={retry} />);

    expect(screen.getByText("Could not load tasks")).toBeDefined();
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(retry).toHaveBeenCalledOnce();
  });
});

describe("Modal", () => {
  it("renders its children", () => {
    render(
      <Modal onClose={vi.fn()}>
        <h2>Trash</h2>
      </Modal>,
    );
    expect(screen.getByRole("heading", { name: "Trash" })).toBeDefined();
  });

  it("focuses the card on mount, keeps the latest onClose, and restores focus on unmount", () => {
    function Harness() {
      const [current, setCurrent] = useState("old");
      return (
        <div>
          <button type="button">opener</button>
          <button type="button" onClick={() => setCurrent("new")}>
            swap
          </button>
          <Modal onClose={() => {}}>
            <span>{current}</span>
          </Modal>
        </div>
      );
    }
    const { unmount } = render(<Harness />);
    const box = screen.getByRole("dialog");
    expect(box).toHaveProperty("tabIndex", -1);
    expect(box.getAttribute("aria-modal")).toBe("true");
    expect(document.activeElement).toBe(box);

    // The effect keeps a ref, so a rerender with a new inline onClose
    // doesn't re-run the effect (which would steal focus again).
    act(() => {
      screen.getByRole("button", { name: "swap" }).click();
    });
    expect(document.activeElement).toBe(box);

    unmount();
    const opener = screen.queryByRole("button", { name: "opener" });
    expect(opener === null || document.activeElement !== opener).toBe(true);
  });

  it("calls the latest onClose on Escape after a rerender", () => {
    const first = vi.fn();
    const second = vi.fn();
    function Harness() {
      const [current, setCurrent] = useState("first");
      return (
        <div>
          <button type="button" onClick={() => setCurrent("second")}>
            swap
          </button>
          <Modal onClose={current === "first" ? first : second}>
            <span>{current}</span>
          </Modal>
        </div>
      );
    }
    render(<Harness />);
    act(() => {
      screen.getByRole("button", { name: "swap" }).click();
    });
    fireEvent.keyDown(window, { key: "Escape" });
    expect(first).not.toHaveBeenCalled();
    expect(second).toHaveBeenCalledOnce();
  });

  it("focuses the opener on unmount when the opener is still mounted", () => {
    function Harness() {
      const [open, setOpen] = useState(false);
      return (
        <div>
          <button type="button" onClick={() => setOpen(true)}>
            open
          </button>
          {open && (
            <Modal onClose={() => setOpen(false)}>
              <p>card</p>
            </Modal>
          )}
        </div>
      );
    }
    render(<Harness />);
    const opener = screen.getByRole("button", { name: "open" });
    opener.focus();
    act(() => {
      opener.click();
    });
    expect(document.activeElement).toBe(screen.getByRole("dialog"));
    act(() => {
      fireEvent.keyDown(window, { key: "Escape" });
    });
    expect(document.activeElement).toBe(opener);
  });

  it("does not try to refocus a null opener on unmount", () => {
    let box: HTMLElement | null = null;
    const { unmount } = render(
      <Modal onClose={vi.fn()}>
        <p>card</p>
      </Modal>,
    );
    box = screen.getByRole("dialog");
    box.remove();
    unmount();
    expect(box).not.toBeNull();
  });

  it("closes on a backdrop click but not on the card", () => {
    const onClose = vi.fn();
    const { container } = render(
      <Modal onClose={onClose}>
        <p>card</p>
      </Modal>,
    );

    fireEvent.click(screen.getByText("card"));
    expect(onClose).not.toHaveBeenCalled();

    fireEvent.click(container.querySelector(".modal") as HTMLElement);
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("closes on Escape but not on other window keys", () => {
    const onClose = vi.fn();
    render(
      <Modal onClose={onClose}>
        <p>card</p>
      </Modal>,
    );

    fireEvent.keyDown(window, { key: "Enter" });
    expect(onClose).not.toHaveBeenCalled();

    fireEvent.keyDown(window, { key: "Escape" });
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("cycles focus forward from the last button to the first", () => {
    render(
      <Modal onClose={vi.fn()}>
        <button type="button">first</button>
        <button type="button">last</button>
      </Modal>,
    );
    const box = screen.getByRole("dialog");
    const first = screen.getByRole("button", { name: "first" });
    const last = screen.getByRole("button", { name: "last" });

    last.focus();
    fireEvent.keyDown(box, { key: "Tab" });
    expect(document.activeElement).toBe(first);
  });

  it("cycles focus backward from the first button to the last", () => {
    render(
      <Modal onClose={vi.fn()}>
        <button type="button">first</button>
        <button type="button">last</button>
      </Modal>,
    );
    const box = screen.getByRole("dialog");
    const first = screen.getByRole("button", { name: "first" });
    const last = screen.getByRole("button", { name: "last" });

    first.focus();
    fireEvent.keyDown(box, { key: "Tab", shiftKey: true });
    expect(document.activeElement).toBe(last);
  });

  it("leaves focus alone for a middle Tab and non-Tab keys", () => {
    render(
      <Modal onClose={vi.fn()}>
        <button type="button">first</button>
        <button type="button">last</button>
      </Modal>,
    );
    const box = screen.getByRole("dialog");
    const first = screen.getByRole("button", { name: "first" });

    first.focus();
    fireEvent.keyDown(box, { key: "Escape" });
    expect(document.activeElement).toBe(first);

    fireEvent.keyDown(box, { key: "Enter" });
    expect(document.activeElement).toBe(first);
  });

  it("pulls focus in from outside the card in both directions", () => {
    render(
      <div>
        <button type="button">outside</button>
        <Modal onClose={vi.fn()}>
          <button type="button">first</button>
          <button type="button">last</button>
        </Modal>
      </div>,
    );
    const box = screen.getByRole("dialog");
    const first = screen.getByRole("button", { name: "first" });
    const last = screen.getByRole("button", { name: "last" });
    const outside = screen.getByRole("button", { name: "outside" });

    outside.focus();
    fireEvent.keyDown(box, { key: "Tab" });
    expect(document.activeElement).toBe(first);

    outside.focus();
    fireEvent.keyDown(box, { key: "Tab", shiftKey: true });
    expect(document.activeElement).toBe(last);
  });

  it("leaves a middle button focused in both directions", () => {
    render(
      <Modal onClose={vi.fn()}>
        <button type="button">first</button>
        <button type="button">middle</button>
        <button type="button">last</button>
      </Modal>,
    );
    const box = screen.getByRole("dialog");
    const middle = screen.getByRole("button", { name: "middle" });

    middle.focus();
    fireEvent.keyDown(box, { key: "Tab" });
    expect(document.activeElement).toBe(middle);

    fireEvent.keyDown(box, { key: "Tab", shiftKey: true });
    expect(document.activeElement).toBe(middle);
  });

  it("ignores Tab when the card has nothing focusable", () => {
    render(
      <Modal onClose={vi.fn()}>
        <p>plain text</p>
      </Modal>,
    );
    const box = screen.getByRole("dialog");

    // No crash, and focus stays where the mount put it.
    fireEvent.keyDown(box, { key: "Tab" });
    expect(document.activeElement).toBe(box);
  });
});
