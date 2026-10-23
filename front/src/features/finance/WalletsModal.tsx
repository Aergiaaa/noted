import { useState, type SubmitEvent } from "react";
import { api, errorLines, type Wallet } from "../../lib/api";
import Modal from "../../components/ui/Modal";

const CREATE_FAILED = "Couldn't save the wallet.";
const RENAME_FAILED = "Couldn't rename the wallet.";
const ARCHIVE_FAILED = "Couldn't change the wallet.";
const NAME_MAX_CHAR = 50;

interface WalletsModalProps {
  wallets: Wallet[];
  onClose: () => void;
  onChanged: () => void;
}

// Create / rename / archive the money pots. Wallets archive, never
// delete (DOMAIN.md) — history and balances keep resolving.
export default function WalletsModal({
  wallets,
  onClose,
  onChanged,
}: WalletsModalProps) {
  const [newName, setNewName] = useState("");
  const [renaming, setRenaming] = useState<{ id: string; name: string } | null>(
    null,
  );
  const [error, setError] = useState<string | null>(null);

  const createWallet = async (submitEvent: SubmitEvent) => {
    submitEvent.preventDefault();
    setError(null);
    try {
      await api("/finance/wallets", {
        method: "POST",
        body: { name: newName },
      });
      setNewName("");
      onChanged();
    } catch (err: unknown) {
      setError(errorLines(err, CREATE_FAILED)[0]);
    }
  };

  const renameWallet = async (id: string, name: string) => {
    setError(null);
    try {
      await api(`/finance/wallets/${id}`, {
        method: "PATCH",
        body: { name },
      });
      setRenaming(null);
      onChanged();
    } catch (err: unknown) {
      setError(errorLines(err, RENAME_FAILED)[0]);
    }
  };

  const setArchived = async (id: string, archived: boolean) => {
    setError(null);
    try {
      await api(`/finance/wallets/${id}`, {
        method: "PATCH",
        body: { archived },
      });
      onChanged();
    } catch (err: unknown) {
      setError(errorLines(err, ARCHIVE_FAILED)[0]);
    }
  };

  return (
    <Modal onClose={onClose}>
      <div className="flex items-center justify-between">
        <h2 className="text-base font-semibold">Manage wallets</h2>
        <button
          type="button"
          className="btn btn-ghost btn-xs"
          onClick={onClose}
        >
          Close
        </button>
      </div>

      <form className="mt-3 flex gap-2" onSubmit={createWallet} noValidate>
        <input
          className="input input-bordered input-sm w-full"
          aria-label="New wallet name"
          maxLength={NAME_MAX_CHAR}
          value={newName}
          onChange={(e) => setNewName(e.target.value)}
        />
        <button type="submit" className="btn btn-primary btn-sm">
          Add
        </button>
      </form>

      <ul className="mt-3 space-y-2">
        {wallets.map((wallet) => {
          const isRenaming = renaming?.id === wallet.id;
          return (
            <li
              key={wallet.id}
              className="flex items-center justify-between gap-2 rounded-md border border-base-300 p-2"
            >
              <span className="flex items-center gap-2 text-sm">
                {wallet.name}
                {wallet.archived_at !== null && (
                  <span className="badge badge-ghost badge-sm">archived</span>
                )}
              </span>

              {isRenaming && renaming !== null ? (
                <span className="flex items-center gap-1">
                  <input
                    className="input input-bordered input-xs w-32"
                    aria-label={`Rename ${wallet.name}`}
                    maxLength={NAME_MAX_CHAR}
                    value={renaming.name}
                    onChange={(e) =>
                      setRenaming({ id: wallet.id, name: e.target.value })
                    }
                  />
                  <button
                    type="button"
                    className="btn btn-primary btn-xs"
                    onClick={() => void renameWallet(wallet.id, renaming.name)}
                  >
                    Save
                  </button>
                  <button
                    type="button"
                    className="btn btn-ghost btn-xs"
                    onClick={() => setRenaming(null)}
                  >
                    Cancel
                  </button>
                </span>
              ) : (
                <span className="flex items-center gap-1">
                  <button
                    type="button"
                    className="btn btn-ghost btn-xs"
                    onClick={() =>
                      setRenaming({ id: wallet.id, name: wallet.name })
                    }
                  >
                    Rename
                  </button>
                  <button
                    type="button"
                    className="btn btn-ghost btn-xs"
                    onClick={() =>
                      void setArchived(wallet.id, wallet.archived_at === null)
                    }
                  >
                    {wallet.archived_at === null ? "Archive" : "Unarchive"}
                  </button>
                </span>
              )}
            </li>
          );
        })}
      </ul>

      {error !== null && (
        <p className="mt-3 text-xs text-error" role="alert">
          {error}
        </p>
      )}
    </Modal>
  );
}
