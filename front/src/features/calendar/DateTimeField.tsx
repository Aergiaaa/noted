import "cally";
import { useRef, useState } from "react";
import { createPortal } from "react-dom";
import { formatDateTime } from "../../lib/format";

const HOURS = Array.from({ length: 24 }, (_, h) => String(h).padStart(2, "0"));
const MINUTES = Array.from({ length: 60 }, (_, m) =>
  String(m).padStart(2, "0"),
);

// Fixed placement for the picker panel. The modal card clips overflow
// (daisyUI modal-box is overflow-y:auto), so the panel is portaled to body
// and anchored right of the trigger, past the card's padding. It flips to
// the left when the viewport is too narrow and clamps vertically.
const PICKER_WIDTH = 288;
const PICKER_HEIGHT = 360;
const PICKER_GAP = 12;
const CARD_PADDING = 24;
const VIEWPORT_MARGIN = 8;

interface DateTimeFieldProps {
  label: string;
  // Naive local datetime, YYYY-MM-DDTHH:MM (datetime-local shape).
  value: string;
  onChange: (value: string) => void;
}

// Date-time picker for the event modal: Cally date (built-in header kept
// for month navigation) plus hour/minute selects, spawned as a fixed panel
// outside the card. A portal backdrop gives outside-click close; clicking
// the trigger while open toggles it shut the same way.
export default function DateTimeField({
  label,
  value,
  onChange,
}: DateTimeFieldProps) {
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<{ top: number; left: number } | null>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const date = value.slice(0, 10);
  const time = value.slice(11);

  const pickDate = (nextDate: string) => onChange(`${nextDate}T${time}`);
  const pickHour = (nextHour: string) =>
    onChange(`${date}T${nextHour}:${time.slice(3)}`);
  const pickMinute = (nextMinute: string) =>
    onChange(`${date}T${time.slice(0, 2)}:${nextMinute}`);

  const toggle = () => {
    if (!open) {
      const rect = triggerRef.current?.getBoundingClientRect();
      if (rect !== undefined) {
        setPos(placePicker(rect, window.innerWidth, window.innerHeight));
      }
    }
    setOpen(!open);
  };

  return (
    <div className="block">
      <span className="label-text text-xs uppercase text-base-content/60">
        {label}
      </span>
      <button
        ref={triggerRef}
        type="button"
        className="input input-bordered input-sm mt-1 w-full text-left font-normal"
        aria-label={label}
        aria-expanded={open}
        onClick={toggle}
      >
        {formatDateTime(value)}
      </button>

      {open &&
        pos !== null &&
        createPortal(
          <>
            <div
              data-testid="picker-backdrop"
              className="fixed inset-0 z-[1000]"
              aria-hidden="true"
              onClick={() => setOpen(false)}
            />
            <div
              data-testid="picker-panel"
              className="fixed z-[1001] w-72 rounded-md border border-base-300 bg-base-200 p-2"
              style={{ top: pos.top, left: pos.left }}
            >
              <calendar-date
                className="cally"
                value={date}
                firstDayOfWeek={1}
                onchange={(e) => pickDate((e.target as HTMLInputElement).value)}
              >
                <calendar-month />
              </calendar-date>
              <div className="mt-2 flex gap-2">
                <select
                  className="select select-bordered select-sm w-full"
                  aria-label={`${label} hour`}
                  value={time.slice(0, 2)}
                  onChange={(e) => pickHour(e.target.value)}
                >
                  {HOURS.map((hour) => (
                    <option key={hour} value={hour}>
                      {hour}
                    </option>
                  ))}
                </select>
                <select
                  className="select select-bordered select-sm w-full"
                  aria-label={`${label} minute`}
                  value={time.slice(3)}
                  onChange={(e) => pickMinute(e.target.value)}
                >
                  {MINUTES.map((minute) => (
                    <option key={minute} value={minute}>
                      {minute}
                    </option>
                  ))}
                </select>
              </div>
            </div>
          </>,
          document.body,
        )}
    </div>
  );
}

// Top-right anchor with left-flip and vertical clamp (see constants).
function placePicker(
  rect: DOMRect,
  viewportWidth: number,
  viewportHeight: number,
): { top: number; left: number } {
  const top = Math.max(
    VIEWPORT_MARGIN,
    Math.min(rect.top, viewportHeight - PICKER_HEIGHT - VIEWPORT_MARGIN),
  );
  const right = rect.right + CARD_PADDING + PICKER_GAP;
  if (right + PICKER_WIDTH <= viewportWidth - VIEWPORT_MARGIN) {
    return { top, left: right };
  }
  const left = rect.left - CARD_PADDING - PICKER_GAP - PICKER_WIDTH;
  return { top, left: Math.max(VIEWPORT_MARGIN, left) };
}
