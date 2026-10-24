import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import LinkChip from "./LinkChip";

afterEach(cleanup);

function renderChip(props: { title?: string | null } = {}) {
  render(
    <MemoryRouter initialEntries={["/tasks"]}>
      <LinkChip kind="event" id="e9" {...props} />
    </MemoryRouter>,
  );
}

describe("LinkChip", () => {
  it("deep-links the generic kind label to its module", () => {
    renderChip();
    const chip = screen.getByRole("link", { name: "event" });
    expect(chip.getAttribute("href")).toBe("/calendar?event=e9");
  });

  it("prefers the resolved title over the generic label", () => {
    renderChip({ title: "Dentist" });
    const chip = screen.getByRole("link", { name: "Dentist" });
    expect(chip.getAttribute("href")).toBe("/calendar?event=e9");
  });

  it("shows no unlink control without an onUnlink handler", () => {
    renderChip();
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("confirms before unlinking and passes the link field", () => {
    const onUnlink = vi.fn();
    render(
      <MemoryRouter>
        <LinkChip kind="task" id="t1" onUnlink={onUnlink} />
      </MemoryRouter>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Unlink task" }));
    expect(onUnlink).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Keep" }));
    expect(onUnlink).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Unlink task" })).toBeDefined();

    fireEvent.click(screen.getByRole("button", { name: "Unlink task" }));
    fireEvent.click(screen.getByRole("button", { name: "Unlink" }));
    expect(onUnlink).toHaveBeenCalledWith("task_id");
    // The confirm resets so a failed unlink leaves the chip usable.
    expect(screen.getByRole("button", { name: "Unlink task" })).toBeDefined();
  });
});
