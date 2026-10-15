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
