import { useState } from "react";
import { Link } from "react-router";
import { mentionRoute, type MentionType } from "../../lib/mentions";

export type LinkField = `${MentionType}_id`;

interface LinkChipProps {
  kind: MentionType;
  id: string;
  title?: string | null;
  // Unlink affordance (chip ×) — only where no edit form exists
  // (FEATURES F9: TransactionRow). Forms unlink by clearing the picker.
  onUnlink?: (field: LinkField) => void;
}

// One FK link as a badge deep-linked to its module. Known title wins;
// otherwise the generic kind label renders — never the raw id (F9).
export default function LinkChip({ kind, id, title, onUnlink }: LinkChipProps) {
  const [confirming, setConfirming] = useState(false);
  const field: LinkField = `${kind}_id`;
  const label = title ?? kind;

  return (
    <span className="inline-flex items-center gap-1">
      <Link className="badge badge-info badge-sm" to={mentionRoute(kind, id)}>
        {label}
      </Link>
      {onUnlink !== undefined &&
        (confirming ? (
          <>
            <button
              type="button"
              className="btn btn-error btn-xs"
              onClick={() => {
                setConfirming(false);
                onUnlink(field);
              }}
            >
              Unlink
            </button>
            <button
              type="button"
              className="btn btn-ghost btn-xs"
              onClick={() => setConfirming(false)}
            >
              Keep
            </button>
          </>
        ) : (
          <button
            type="button"
            className="btn btn-ghost btn-xs"
            aria-label={`Unlink ${kind}`}
            onClick={() => setConfirming(true)}
          >
            ×
          </button>
        ))}
    </span>
  );
}
