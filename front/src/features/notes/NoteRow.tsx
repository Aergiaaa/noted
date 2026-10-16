import type { Note } from "../../lib/api";
import NoteBody from "./NoteBody";

interface NoteRowProps {
  note: Note;
  onOpen: (note: Note) => void;
  onTogglePin: (note: Note) => void;
}

export default function NoteRow({ note, onOpen, onTogglePin }: NoteRowProps) {
  return (
    <li
      className="card bg-base-200 border border-base-300 p-4"
      data-testid="note-row"
    >
      <div className="flex items-start justify-between gap-2">
        <button
          type="button"
          className="text-left text-base font-semibold"
          onClick={() => onOpen(note)}
        >
          {note.title}
        </button>
        <button
          type="button"
          className={`btn btn-ghost btn-sm ${note.pinned ? "btn-active" : ""}`}
          aria-pressed={note.pinned}
          onClick={() => onTogglePin(note)}
        >
          {note.pinned ? "Unpin" : "Pin"}
        </button>
      </div>
      <div className="mt-2 max-h-24 overflow-hidden text-base-content/70">
        <NoteBody body={note.body_md} />
      </div>
    </li>
  );
}
