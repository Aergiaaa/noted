interface LinkItem {
  id: string;
  title: string;
}

interface LinkPickerProps {
  label: string;
  items: LinkItem[];
  // null = unlinked (the None option); a stored id missing from items
  // still renders so a loading list never shows a blank select.
  value: string | null;
  onChange: (id: string | null) => void;
  loading?: boolean;
}

// Shared link select across TaskForm / EventForm / TransactionForm (F9):
// existing titles only, "" clears per API.md PATCH. No free-text ID entry.
export default function LinkPicker({
  label,
  items,
  value,
  onChange,
  loading = false,
}: LinkPickerProps) {
  const selectValue = value ?? "";
  const unknownStored =
    value !== null && !items.some((item) => item.id === value);

  return (
    <label className="block">
      <span className="label-text text-xs uppercase text-base-content/60">
        {label}
      </span>
      <select
        className="select select-bordered select-sm mt-1 w-full"
        aria-label={label}
        value={selectValue}
        disabled={loading}
        onChange={(e) =>
          onChange(e.target.value === "" ? null : e.target.value)
        }
      >
        {loading ? (
          <option value="">Loading…</option>
        ) : (
          <option value="">{unknownStored ? "Link…" : "None"}</option>
        )}
        {unknownStored && <option value={value}>{value}</option>}
        {items.map((item) => (
          <option key={item.id} value={item.id}>
            {item.title}
          </option>
        ))}
      </select>
    </label>
  );
}
