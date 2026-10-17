import "cally";
import { useState } from "react";
import { createPortal } from "react-dom";
import { formatDate, fromDayKey } from "../../lib/format";

// Fixed placement for the picker panel. The modal card clips overflow
// (daisyUI modal-box is overflow-y:auto), so the panel is portaled to body
// and anchored right of the trigger, past the card's padding. It flips to
// the left when the viewport is too narrow and clamps vertically.
const PICKER_WIDTH = 288;
const PICKER_HEIGHT = 280;
const PICKER_GAP = 12;
const CARD_PADDING = 24;
const VIEWPORT_MARGIN = 8;

interface DatePickerProps {
  // aria-label only; the visible group label is rendered by the form.
  label: string;
  // Naive local datetime, YYYY-MM-DDTHH:MM — the date part is the picker,
  // the time part rides along untouched so callers keep one value.
  value: string;
  onChange: (value: string) => void;
}

// Date picker for the event modal: Cally date (built-in header kept for
// month navigation), spawned as a fixed panel outside the card. A portal
// backdrop gives outside-click close; clicking the trigger while open
// toggles it shut the same way. Hour and minute are separate TimeInput
// fields beside the trigger, not part of this panel.
export default function DatePicker({
  label,
  value,
  onChange,
}: DatePickerProps) {
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<{ top: number; left: number } | null>(null);
  const date = value.slice(0, 10);
  const time = value.slice(11);

  const pickDate = (nextDate: string) => onChange(`${nextDate}T${time}`);

  // The trigger is the click target, so currentTarget is the rect source —
  // no ref, no null-guard branch.
  const toggle = (event: React.MouseEvent<HTMLButtonElement>) => {
    if (!open) {
      setPos(
        placePicker(
          event.currentTarget.getBoundingClientRect(),
          window.innerWidth,
          window.innerHeight,
        ),
      );
    }
    setOpen(!open);
  };

  return (
    <button
      type="button"
      className="input input-bordered input-sm w-full text-left font-normal"
      aria-label={label}
      aria-expanded={open}
      onClick={toggle}
    >
      {formatDate(fromDayKey(date))}
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
              onClick={(e) => e.stopPropagation()}
            >
              <calendar-date
                className="cally"
                value={date}
                firstDayOfWeek={1}
                onchange={(e) => pickDate((e.target as HTMLInputElement).value)}
              >
                <calendar-month />
              </calendar-date>
            </div>
          </>,
          document.body,
        )}
    </button>
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
