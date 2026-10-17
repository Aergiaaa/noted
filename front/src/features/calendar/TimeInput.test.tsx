import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import TimeInput from "./TimeInput";

afterEach(() => cleanup());

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

describe("TimeInput", () => {
  it("shows the padded value", () => {
    renderHour("09");
    expect(
      (screen.getByLabelText("Starts hour") as HTMLInputElement).value,
    ).toBe("09");
  });

  it("offers a dropdown of every value up to max", () => {
    renderHour("14");
    const options = document.querySelectorAll("datalist option");
    expect(options).toHaveLength(24);
    expect(options[0].getAttribute("value")).toBe("00");
    expect(options[23].getAttribute("value")).toBe("23");
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

  it("commits on Enter", () => {
    const onChange = renderHour("14");
    const input = screen.getByLabelText("Starts hour");

    fireEvent.change(input, { target: { value: "7" } });
    fireEvent.keyDown(input, { key: "Enter" });
    expect(onChange).toHaveBeenCalledWith("07");
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
