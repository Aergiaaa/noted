import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import TimeInput from "./TimeInput";

afterEach(() => {
  cleanup();
  setViewport(1024, 768);
});

function setViewport(width: number, height: number) {
  Object.defineProperty(window, "innerWidth", { value: width });
  Object.defineProperty(window, "innerHeight", { value: height });
}

function stubRect(input: HTMLElement, rect: Partial<DOMRect>) {
  input.getBoundingClientRect = () =>
    ({
      top: 0,
      left: 0,
      right: 0,
      bottom: 0,
      width: 0,
      height: 0,
      x: 0,
      y: 0,
      toJSON: () => ({}),
      ...rect,
    }) as DOMRect;
}

function renderHour(value = "14", onChange = vi.fn()) {
  render(
    <TimeInput
      label="Starts hour"
      value={value}
      max={23}
      onChange={onChange}
    />,
  );
  return onChange;
}

function menu(): HTMLElement | null {
  return document.querySelector('[data-testid="time-menu"]');
}

function option(name: string): HTMLElement {
  const items = Array.from(
    document.querySelectorAll<HTMLButtonElement>(
      '[data-testid="time-menu"] button',
    ),
  );
  const found = items.find((b) => b.textContent === name);
  if (!found) throw new Error(`option ${name} not in menu`);
  return found;
}

describe("TimeInput", () => {
  it("shows the padded value with no native datalist arrow", () => {
    renderHour("09");
    const input = screen.getByLabelText("Starts hour") as HTMLInputElement;
    expect(input.value).toBe("09");
    expect(input.getAttribute("list")).toBeNull();
    expect(document.querySelector("datalist")).toBeNull();
  });

  it("opens a menu of every value on focus", () => {
    renderHour("14");
    const input = screen.getByLabelText("Starts hour");
    fireEvent.focus(input);

    expect(input.getAttribute("aria-expanded")).toBe("true");
    expect(menu()?.parentElement).toBe(document.body);
    const options = document.querySelectorAll(
      '[data-testid="time-menu"] button',
    );
    expect(options).toHaveLength(24);
    expect(options[0].textContent).toBe("00");
    expect(options[23].textContent).toBe("23");
  });

  it("commits a clicked option and closes the menu", () => {
    const onChange = renderHour("14");
    fireEvent.focus(screen.getByLabelText("Starts hour"));

    // mousedown first, like a real click: the menu swallows it so the
    // input never blurs before the option's click lands.
    fireEvent.mouseDown(option("07"));
    fireEvent.click(option("07"));
    expect(onChange).toHaveBeenCalledWith("07");
    expect(menu()).toBeNull();
    expect(
      (screen.getByLabelText("Starts hour") as HTMLInputElement).value,
    ).toBe("14");
  });

  it("anchors the menu below the input", () => {
    setViewport(1024, 768);
    renderHour("14");
    const input = screen.getByLabelText("Starts hour");
    stubRect(input, { top: 100, left: 300, right: 364, bottom: 136 });

    fireEvent.focus(input);
    expect(menu()?.style.top).toBe("140px");
    expect(menu()?.style.left).toBe("300px");
  });

  it("flips the menu above when the viewport is short, clamped to the edge", () => {
    setViewport(1024, 200);
    renderHour("14");
    const input = screen.getByLabelText("Starts hour");
    stubRect(input, { top: 100, left: 300, right: 364, bottom: 136 });

    fireEvent.focus(input);
    // 136 + 4 + 176 > 200 - 8, so above: 100 - 4 - 176 < 8 -> 8.
    expect(menu()?.style.top).toBe("8px");
  });

  it("clamps the menu left and right inside the viewport", () => {
    setViewport(320, 768);
    renderHour("14");
    const input = screen.getByLabelText("Starts hour");
    stubRect(input, { top: 100, left: 300, right: 364, bottom: 136 });

    fireEvent.focus(input);
    // right-clamped: 320 - 8 - 64 = 248.
    expect(menu()?.style.left).toBe("248px");

    stubRect(input, { top: 100, left: 0, right: 64, bottom: 136 });
    fireEvent.blur(input);
    fireEvent.focus(input);
    // left-clamped to the margin.
    expect(menu()?.style.left).toBe("8px");
  });

  it("closes the menu on an outside mousedown without committing", () => {
    const onChange = renderHour("14");
    const input = screen.getByLabelText("Starts hour");
    fireEvent.focus(input);

    fireEvent.mouseDown(
      document.querySelector('[data-testid="time-menu-backdrop"]') as Element,
    );
    expect(menu()).toBeNull();
    expect(onChange).not.toHaveBeenCalled();
  });

  it("commits a typed valid value, padded, on blur", () => {
    const onChange = renderHour("14");
    const input = screen.getByLabelText("Starts hour");

    fireEvent.change(input, { target: { value: "9" } });
    expect(onChange).not.toHaveBeenCalled();

    fireEvent.blur(input);
    expect(onChange).toHaveBeenCalledWith("09");
  });

  it("strips non-digits while typing", () => {
    const onChange = renderHour("14");
    const input = screen.getByLabelText("Starts hour");

    fireEvent.change(input, { target: { value: "1a2b" } });
    expect((input as HTMLInputElement).value).toBe("12");
    fireEvent.blur(input);
    expect(onChange).toHaveBeenCalledWith("12");
  });

  it("reverts on blur when the typed value is out of range", () => {
    const onChange = renderHour("14");
    const input = screen.getByLabelText("Starts hour");

    fireEvent.change(input, { target: { value: "99" } });
    fireEvent.blur(input);
    expect(onChange).not.toHaveBeenCalled();
    expect((input as HTMLInputElement).value).toBe("14");
  });

  it("reverts on blur when the field was emptied", () => {
    const onChange = renderHour("14");
    const input = screen.getByLabelText("Starts hour");

    fireEvent.change(input, { target: { value: "" } });
    fireEvent.blur(input);
    expect(onChange).not.toHaveBeenCalled();
    expect((input as HTMLInputElement).value).toBe("14");
  });

  it("commits on Enter and closes the menu", () => {
    const onChange = renderHour("14");
    const input = screen.getByLabelText("Starts hour");
    fireEvent.focus(input);

    fireEvent.change(input, { target: { value: "7" } });
    fireEvent.keyDown(input, { key: "Enter" });
    expect(onChange).toHaveBeenCalledWith("07");
    expect(menu()).toBeNull();
  });

  it("closes the menu and reverts the draft on Escape without committing", () => {
    const onChange = renderHour("14");
    const input = screen.getByLabelText("Starts hour");
    fireEvent.focus(input);

    fireEvent.change(input, { target: { value: "7" } });
    fireEvent.keyDown(input, { key: "Escape" });
    expect(menu()).toBeNull();
    expect(onChange).not.toHaveBeenCalled();
    expect((input as HTMLInputElement).value).toBe("14");

    fireEvent.keyDown(input, { key: "Escape" });
    expect(onChange).not.toHaveBeenCalled();
  });

  it("ignores other keys", () => {
    const onChange = renderHour("14");
    const input = screen.getByLabelText("Starts hour");

    fireEvent.change(input, { target: { value: "7" } });
    fireEvent.keyDown(input, { key: "a" });
    expect(onChange).not.toHaveBeenCalled();
    fireEvent.blur(input);
    expect(onChange).toHaveBeenCalledWith("07");
  });

  it("enforces the minute max", () => {
    const onChange = vi.fn();
    render(
      <TimeInput
        label="Starts minute"
        value="30"
        max={59}
        onChange={onChange}
      />,
    );
    const input = screen.getByLabelText("Starts minute");

    fireEvent.change(input, { target: { value: "75" } });
    fireEvent.blur(input);
    expect(onChange).not.toHaveBeenCalled();
    expect((input as HTMLInputElement).value).toBe("30");
  });
});
