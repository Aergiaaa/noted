import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import LinkPicker from "./LinkPicker";

afterEach(cleanup);

const ITEMS = [
  { id: "e9", title: "Dentist" },
  { id: "e10", title: "Standup" },
];

describe("LinkPicker", () => {
  it("lists every option with a leading None and the current value", () => {
    render(
      <LinkPicker label="Event" items={ITEMS} value="e10" onChange={vi.fn()} />,
    );

    const select = screen.getByLabelText("Event");
    expect((select as HTMLSelectElement).value).toBe("e10");
    const options = screen.getAllByRole("option");
    expect(options.map((o) => o.textContent)).toEqual([
      "None",
      "Dentist",
      "Standup",
    ]);
  });

  it("emits the picked id", () => {
    const onChange = vi.fn();
    render(
      <LinkPicker
        label="Event"
        items={ITEMS}
        value={null}
        onChange={onChange}
      />,
    );

    fireEvent.change(screen.getByLabelText("Event"), {
      target: { value: "e9" },
    });
    expect(onChange).toHaveBeenCalledWith("e9");
  });

  it("emits null when cleared back to None", () => {
    const onChange = vi.fn();
    render(
      <LinkPicker label="Note" items={ITEMS} value="e9" onChange={onChange} />,
    );

    fireEvent.change(screen.getByLabelText("Note"), {
      target: { value: "" },
    });
    expect(onChange).toHaveBeenCalledWith(null);
  });

  it("keeps a stored value whose option is not in the list yet (loading)", () => {
    render(
      <LinkPicker
        label="Task"
        items={[]}
        value="t_missing"
        onChange={vi.fn()}
        loading
      />,
    );

    const select = screen.getByLabelText("Task") as HTMLSelectElement;
    expect(select.value).toBe("t_missing");
    expect(screen.getByText("Loading…")).toBeDefined();
  });

  it("shows the loading placeholder while items have not landed", () => {
    render(
      <LinkPicker
        label="Task"
        items={[]}
        value={null}
        onChange={vi.fn()}
        loading
      />,
    );

    const options = screen.getAllByRole("option");
    expect(options[0]?.textContent).toBe("Loading…");
    expect((screen.getByLabelText("Task") as HTMLSelectElement).disabled).toBe(
      true,
    );
  });
});
