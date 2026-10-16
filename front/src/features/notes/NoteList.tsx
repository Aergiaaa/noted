import type { Note } from "../../lib/api";
import NoteRow from "./NoteRow";

interface NoteListProps {
  notes: Note[];
  onOpen: (note: Note) => void;
  onTogglePin: (note: Note) => void;
}

export default function NoteList({
  notes,
  onOpen,
  onTogglePin,
}: NoteListProps) {
  return (
    <ul className="space-y-3">
      {notes.map((note) => (
        <NoteRow
          key={note.id}
          note={note}
          onOpen={onOpen}
          onTogglePin={onTogglePin}
        />
      ))}
    </ul>
  );
}
