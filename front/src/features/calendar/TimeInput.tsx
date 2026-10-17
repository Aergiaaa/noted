import { useId, useState, type KeyboardEvent } from "react";

interface TimeInputProps {
  // aria-label only; the visible group label is rendered by the form.
  label: string;
  // Committed value, zero-padded ("00".."23" / "00".."59").
  value: string;
  max: number;
  onChange: (value: string) => void;
}

// Hour/minute field: a datalist dropdown of every valid value, plus free
// typing. Typing keeps a local draft so partial input never reaches the
// composed datetime; blur or Enter commits a valid number (zero-padded)
// and anything else reverts to the last committed value.
export default function TimeInput({
  label,
  value,
  max,
  onChange,
}: TimeInputProps) {
  const listId = useId();
  const [draft, setDraft] = useState<string | null>(null);
  const shown = draft ?? value;

  const commit = (raw: string) => {
    const digits = raw.replace(/\D/g, "").slice(0, 2);
    const n = Number(digits);
    if (digits !== "" && n <= max) {
      onChange(String(n).padStart(2, "0"));
    }
    setDraft(null);
  };

  return (
    <>
      <input
        className="input input-bordered input-sm w-16 text-center tabular-nums"
        aria-label={label}
        list={listId}
        inputMode="numeric"
        autoComplete="off"
        value={shown}
        onChange={(e) =>
          setDraft(e.target.value.replace(/\D/g, "").slice(0, 2))
        }
        onBlur={() => commit(shown)}
        onKeyDown={(e: KeyboardEvent<HTMLInputElement>) => {
          if (e.key === "Enter") commit(shown);
        }}
      />
      <datalist id={listId}>
        {Array.from({ length: max + 1 }, (_, i) => {
          const option = String(i).padStart(2, "0");
          return <option key={option} value={option} />;
        })}
      </datalist>
    </>
  );
}
