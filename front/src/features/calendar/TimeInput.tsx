import { useState, type KeyboardEvent } from "react";
import { createPortal } from "react-dom";

const MENU_W = 64;
const MENU_H = 176;
const MENU_GAP = 4;
const VIEWPORT_MARGIN = 8;

interface TimeInputProps {
  // aria-label only; the visible group label is rendered by the form.
  label: string;
  // Committed value, zero-padded ("00".."23" / "00".."59").
  value: string;
  max: number;
  onChange: (value: string) => void;
}

// Hour/minute field: focus opens a small scrollable menu of every valid
// value (portal — the modal card would clip it), and free typing always
// works. Typing keeps a local draft so partial input never reaches the
// composed datetime; blur or Enter commits a valid number (zero-padded)
// and anything else reverts to the last committed value.
export default function TimeInput({
  label,
  value,
  max,
  onChange,
}: TimeInputProps) {
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState<string | null>(null);
  const [pos, setPos] = useState<{ top: number; left: number } | null>(null);
  const shown = draft ?? value;

  const commit = (raw: string) => {
    const digits = raw.replace(/\D/g, "").slice(0, 2);
    const n = Number(digits);
    if (digits !== "" && n <= max) {
      onChange(String(n).padStart(2, "0"));
    }
    setDraft(null);
  };

  // currentTarget is the rect source — no ref, no null-guard branch.
  const openMenu = (event: React.FocusEvent<HTMLInputElement>) => {
    setPos(
      placeMenu(
        event.currentTarget.getBoundingClientRect(),
        window.innerWidth,
        window.innerHeight,
      ),
    );
    setOpen(true);
  };

  return (
    <div className="relative">
      <input
        className="input input-bordered input-sm w-16 text-center tabular-nums"
        aria-label={label}
        aria-expanded={open}
        inputMode="numeric"
        autoComplete="off"
        value={shown}
        onChange={(e) =>
          setDraft(e.target.value.replace(/\D/g, "").slice(0, 2))
        }
        onFocus={openMenu}
        onBlur={() => {
          commit(shown);
          setOpen(false);
        }}
        onKeyDown={(e: KeyboardEvent<HTMLInputElement>) => {
          if (e.key === "Enter") {
            commit(shown);
            setOpen(false);
          }
          if (e.key === "Escape" && open) {
            e.stopPropagation();
            setDraft(null);
            setOpen(false);
          }
        }}
      />
      {open &&
        pos !== null &&
        createPortal(
          <>
            <div
              data-testid="time-menu-backdrop"
              className="fixed inset-0 z-1199"
              aria-hidden="true"
              onMouseDown={() => setOpen(false)}
            />
            <ul
              data-testid="time-menu"
              className="fixed z-1200 max-h-44 w-16 overflow-auto rounded-md border border-base-300 bg-base-200 p-1 shadow"
              style={{ top: pos.top, left: pos.left }}
              // Keep the input focused: a blur would close the menu before
              // the option's click lands.
              onMouseDown={(e) => e.preventDefault()}
            >
              {Array.from({ length: max + 1 }, (_, i) => {
                const option = String(i).padStart(2, "0");
                return (
                  <li key={option}>
                    <button
                      type="button"
                      className={`block w-full rounded px-1 py-0.5 text-center text-xs tabular-nums hover:bg-base-300 ${
                        option === shown ? "bg-primary/20" : ""
                      }`}
                      onClick={() => {
                        onChange(option);
                        setDraft(null);
                        setOpen(false);
                      }}
                    >
                      {option}
                    </button>
                  </li>
                );
              })}
            </ul>
          </>,
          document.body,
        )}
    </div>
  );
}

// Below the input, flipping above when the viewport is short, clamped on
// both axes so the menu never hangs off screen.
function placeMenu(
  rect: DOMRect,
  viewportWidth: number,
  viewportHeight: number,
): { top: number; left: number } {
  let top = rect.bottom + MENU_GAP;
  if (top + MENU_H > viewportHeight - VIEWPORT_MARGIN) {
    top = rect.top - MENU_GAP - MENU_H;
  }
  top = Math.max(VIEWPORT_MARGIN, top);

  const left = rect.right - MENU_W;
  return {
    top,
    left: Math.min(
      Math.max(VIEWPORT_MARGIN, left),
      viewportWidth - VIEWPORT_MARGIN - MENU_W,
    ),
  };
}
