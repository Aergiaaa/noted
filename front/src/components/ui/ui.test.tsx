import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import EmptyState from "./EmptyState";
import ErrorState from "./ErrorState";

afterEach(cleanup);

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
