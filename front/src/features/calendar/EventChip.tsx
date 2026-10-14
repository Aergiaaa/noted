import type { Event } from "../../lib/api";
import { formatTime } from "../../lib/format";

interface EventChipProps {
  event: Event;
  onOpen: (event: Event) => void;
}

export default function EventChip({ event, onOpen }: EventChipProps) {
  return (
    <button
      type="button"
      className="block w-full truncate rounded bg-base-300 px-1 text-left text-xs hover:bg-base-100"
      onClick={(e) => {
        // The cell behind this chip means "create" — stop the click here.
        e.stopPropagation();
        onOpen(event);
      }}
    >
      <span className="tabular-nums text-base-content/70">
        {formatTime(event.starts_at)}
      </span>{" "}
      {event.title}
    </button>
  );
}
