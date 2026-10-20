import {
  useCallback,
  useEffect,
  useRef,
  type KeyboardEvent as ReactKeyboardEvent,
  type MouseEvent,
  type ReactNode,
} from "react";

interface ModalProps {
  onClose: () => void;
  children: ReactNode;
}

// Shared daisyUI modal shell with a real focus trap (UI.md: focus trap,
// Esc, backdrop close). Focus enters the card on mount, Tab/Shift+Tab cycle
// within it, and the opener regains focus on unmount. Escape is a
// window-level listener so it works no matter where focus sits — including
// React portals inside the card — and a separate effect keeps the callback
// fresh in a ref so the focus effect never re-runs (which would steal focus
// on every parent render).
export default function Modal({ onClose, children }: ModalProps) {
  const onCloseRef = useRef(onClose);
  const openerRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    onCloseRef.current = onClose;
  }, [onClose]);

  const setBox = useCallback((box: HTMLDivElement | null) => {
    if (box === null) return;
    openerRef.current = document.activeElement as HTMLElement | null;
    box.focus();
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onCloseRef.current();
    };
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("keydown", onKey);
      const opener = openerRef.current;
      if (opener !== null) opener.focus();
    };
  }, []);

  const trapTab = (e: ReactKeyboardEvent<HTMLDivElement>) => {
    if (e.key !== "Tab") return;
    const box = e.currentTarget;
    const focusables = Array.from(
      box.querySelectorAll<HTMLElement>(FOCUSABLES),
    );
    if (focusables.length === 0) return;
    const first = focusables[0];
    const last = focusables[focusables.length - 1];
    const inside = focusables.includes(document.activeElement as HTMLElement);
    if (e.shiftKey) {
      if (document.activeElement === first || !inside) {
        e.preventDefault();
        last.focus();
      }
    } else if (document.activeElement === last || !inside) {
      e.preventDefault();
      first.focus();
    }
  };

  const stop = (e: MouseEvent) => e.stopPropagation();

  return (
    <div className="modal modal-open" onClick={onClose}>
      <div
        ref={setBox}
        role="dialog"
        aria-modal="true"
        tabIndex={-1}
        className="modal-box outline-none"
        onClick={stop}
        onKeyDown={trapTab}
      >
        {children}
      </div>
    </div>
  );
}

const FOCUSABLES =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';
