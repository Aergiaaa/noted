import "cally";
import { useState } from "react";
import { formatDateTime } from "../../lib/format";

const HOURS = Array.from({ length: 24 }, (_, h) => String(h).padStart(2, "0"));
const MINUTES = Array.from({ length: 60 }, (_, m) =>
  String(m).padStart(2, "0"),
);

interface DateTimeFieldProps {
  label: string;
  // Naive local datetime, YYYY-MM-DDTHH:MM (datetime-local shape).
  value: string;
  onChange: (value: string) => void;
}

// Date-time picker for the event modal: Cally date (built-in header kept
// for month navigation) plus hour/minute selects in a daisyUI dropdown.
// A full-screen backdrop gives outside-click close; clicking the trigger
// while open toggles it shut the same way.
export default function DateTimeField({
  label,
  value,
  onChange,
}: DateTimeFieldProps) {
  const [open, setOpen] = useState(false);
  const date = value.slice(0, 10);
  const time = value.slice(11);

  const pickDate = (nextDate: string) => onChange(`${nextDate}T${time}`);
  const pickHour = (nextHour: string) =>
    onChange(`${date}T${nextHour}:${time.slice(3)}`);
  const pickMinute = (nextMinute: string) =>
    onChange(`${date}T${time.slice(0, 2)}:${nextMinute}`);

  return (
    <div className="block">
      <span className="label-text text-xs uppercase text-base-content/60">
        {label}
      </span>
      <div className="dropdown mt-1 w-full">
        <button
          type="button"
          className="input input-bordered input-sm w-full text-left font-normal"
          aria-label={label}
          aria-expanded={open}
          onClick={() => setOpen(!open)}
        >
          {formatDateTime(value)}
        </button>

        {open && (
          <div
            data-testid="picker-backdrop"
            className="fixed inset-0 z-10"
            aria-hidden="true"
            onClick={() => setOpen(false)}
          />
        )}

        {open && (
          <div className="dropdown-content z-20 mt-1 w-full rounded-md border border-base-300 bg-base-200 p-2">
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
        )}
      </div>
    </div>
  );
}
