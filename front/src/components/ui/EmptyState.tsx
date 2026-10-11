interface EmptyStateProps {
  message: string
  action?: { label: string; onClick: () => void }
}

export default function EmptyState({ message, action }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center gap-3 py-10 text-center">
      <p className="text-sm text-base-content/70">{message}</p>
      {action !== undefined && (
        <button className="btn btn-primary btn-sm" onClick={action.onClick}>
          {action.label}
        </button>
      )}
    </div>
  )
}
