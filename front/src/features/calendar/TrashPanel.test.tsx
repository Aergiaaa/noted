import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import TrashPanel from "./TrashPanel";
import type { Event } from "../../lib/api";

afterEach(cleanup);

const deleted: Event = {
  id: "e9",
  title: "Dentist",
  starts_at: "2026-10-14T09:00:00.000Z",
  ends_at: "2026-10-14T10:00:00.000Z",
};

function panel(overrides: Partial<Parameters<typeof TrashPanel>[0]> = {}) {
  return (
    <TrashPanel
      open
      events={[]}
      loading={false}
      error={null}
      onRestore={vi.fn()}
      onRetry={vi.fn()}
      onClose={vi.fn()}
      {...overrides}
    />
  );
}

describe("TrashPanel", () => {
  it("renders nothing when closed", () => {
    render(panel({ open: false, events: [deleted] }));
    expect(screen.queryByRole("heading", { name: "Trash" })).toBeNull();
  });

  it("lists deleted events and restores on click", () => {
    const onRestore = vi.fn();
    render(panel({ events: [deleted], onRestore }));

    expect(screen.getByText("Dentist")).toBeDefined();
    fireEvent.click(screen.getByRole("button", { name: "Restore" }));
    expect(onRestore).toHaveBeenCalledWith("e9");
  });

  it("shows skeletons while loading", () => {
    const { container } = render(panel({ loading: true, events: [deleted] }));
    expect(container.querySelectorAll(".skeleton")).toHaveLength(3);
    expect(screen.queryByText("Dentist")).toBeNull();
  });

  it("shows the error state with retry", () => {
    const onRetry = vi.fn();
    render(panel({ error: "Couldn't load events.", onRetry }));

    expect(screen.getByText("Couldn't load events.")).toBeDefined();
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it("shows the empty state when nothing was deleted", () => {
    render(panel());
    expect(screen.getByText("Nothing in the trash.")).toBeDefined();
  });

  it("closes on backdrop click but not on inner clicks", () => {
    const onClose = vi.fn();
    const { container } = render(panel({ events: [deleted], onClose }));

    fireEvent.click(screen.getByText("Dentist"));
    expect(onClose).not.toHaveBeenCalled();

    fireEvent.click(container.querySelector(".modal")!);
    expect(onClose).toHaveBeenCalledOnce();
  });
});
