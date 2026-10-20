import { useEffect, useState } from "react";
import { api, errorLines, type Note, type NotesResponse } from "../lib/api";
import NoteList from "../features/notes/NoteList";
import NoteForm from "../features/notes/NoteForm";
import ErrorState from "../components/ui/ErrorState";
import EmptyState from "../components/ui/EmptyState";

const LOAD_FAILED = "Couldn't load notes.";
const UPDATE_FAILED = "Couldn't update the note.";
const EMPTY_MESSAGE = "No notes yet.";

export default function NotesPage() {
  const [notes, setNotes] = useState<Note[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<Note | null>(null);

  useEffect(() => {
    let cancelled = false;
    api<NotesResponse>("/notes")
      .then((res) => {
        if (!cancelled) {
          setNotes(res.data);
          setError(null);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(errorLines(err, LOAD_FAILED)[0]);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [nonce]);

  const refetch = () => {
    setError(null);
    setNonce((n) => n + 1);
  };
  const retry = () => {
    setError(null);
    setLoading(true);
    setNonce((n) => n + 1);
  };
  const togglePin = async (note: Note) => {
    try {
      await api(`/notes/${note.id}`, {
        method: "PATCH",
        body: { pinned: !note.pinned },
      });
      refetch();
    } catch (err: unknown) {
      setError(errorLines(err, UPDATE_FAILED)[0]);
    }
  };
  const closeForm = () => {
    setCreating(false);
    setEditing(null);
  };

  if (error !== null) {
    return (
      <section>
        <h1 className="text-xl font-semibold">Notes</h1>
        <ErrorState message={error} onRetry={retry} />
      </section>
    );
  }

  if (loading) {
    return (
      <section>
        <h1 className="text-xl font-semibold">Notes</h1>
        <div className="mt-4 space-y-3">
          {Array.from({ length: 3 }, (_, i) => (
            <div key={i} className="skeleton h-20" />
          ))}
        </div>
      </section>
    );
  }

  return (
    <section>
      <header className="flex flex-wrap items-center justify-between gap-2">
        <h1 className="text-xl font-semibold">Notes</h1>
        <button
          type="button"
          className="btn btn-primary btn-sm"
          onClick={() => setCreating(true)}
        >
          New note
        </button>
      </header>

      <div className="mt-4">
        {notes.length === 0 ? (
          <EmptyState
            message={EMPTY_MESSAGE}
            action={{
              label: "Create your first note",
              onClick: () => setCreating(true),
            }}
          />
        ) : (
          <NoteList
            notes={notes}
            onOpen={setEditing}
            onTogglePin={(note) => void togglePin(note)}
          />
        )}
      </div>

      {(creating || editing !== null) && (
        <NoteForm
          note={creating ? null : editing}
          onClose={closeForm}
          onSaved={refetch}
          onDeleted={refetch}
        />
      )}
    </section>
  );
}
