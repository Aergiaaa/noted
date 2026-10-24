import { useLayoutEffect, useRef, useState } from "react";
import { insertMention, type MentionType } from "../../lib/mentions";
import { useLinkSources } from "./useLinkSources";

const MENTION_SOURCES = ["task", "event", "note"] as const;

interface MentionInsertProps {
  text: string;
  onTextChange: (next: string) => void;
  textareaRef: { current: HTMLTextAreaElement | null };
}

// F9 mention insert: a toggle under the prose field that reveals a picker
// of existing titles; picking splices @[kind:id] at the caret (mentions.ts
// grammar) and puts the caret right behind the token. Catalogs fetch only
// once the picker opens — forms without a pick pay nothing.
export default function MentionInsert({
  text,
  onTextChange,
  textareaRef,
}: MentionInsertProps) {
  const [open, setOpen] = useState(false);
  const pendingCaret = useRef<number | null>(null);

  const insertAtCaret = (type: MentionType, id: string) => {
    const textarea = textareaRef.current as HTMLTextAreaElement;
    const next = insertMention(textarea.value, 0, type, id, {
      start: textarea.selectionStart,
      end: textarea.selectionEnd,
    });
    pendingCaret.current = next.cursor;
    onTextChange(next.text);
  };

  useLayoutEffect(() => {
    const caret = pendingCaret.current;
    if (caret === null) return;
    pendingCaret.current = null;
    const textarea = textareaRef.current as HTMLTextAreaElement;
    textarea.focus();
    textarea.setSelectionRange(caret, caret);
  }, [text, textareaRef]);

  return (
    <div className="mt-1 flex items-center gap-2">
      <button
        type="button"
        className="btn btn-ghost btn-xs"
        aria-pressed={open}
        onClick={() => setOpen((value) => !value)}
      >
        Mention
      </button>
      {open && <MentionPicker onPick={insertAtCaret} />}
    </div>
  );
}

// Owns the catalog fetch (mounted only while the picker is open, so the
// hook's eager fetch stays behind the toggle).
function MentionPicker({
  onPick,
}: {
  onPick: (type: MentionType, id: string) => void;
}) {
  const sources = useLinkSources(MENTION_SOURCES);
  const groups: {
    kind: MentionType;
    items: { id: string; title: string }[];
  }[] = [
    { kind: "task", items: sources.tasks },
    { kind: "event", items: sources.events },
    { kind: "note", items: sources.notes },
  ];

  return (
    <select
      className="select select-bordered select-sm min-w-0 flex-1"
      aria-label="Insert mention"
      value=""
      disabled={sources.loading}
      onChange={(event) => {
        const [type, id] = event.target.value.split(":");
        onPick(type as MentionType, id);
      }}
    >
      {sources.loading ? (
        <option value="">Loading…</option>
      ) : (
        <option value="">Mention…</option>
      )}
      {groups.map((group) => (
        <optgroup key={group.kind} label={`${group.kind}s`}>
          {group.items.map((item) => (
            <option key={item.id} value={`${group.kind}:${item.id}`}>
              {item.title}
            </option>
          ))}
        </optgroup>
      ))}
    </select>
  );
}
