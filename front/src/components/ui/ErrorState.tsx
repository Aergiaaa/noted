interface ErrorStateProps {
  message: string
  onRetry: () => void
}

export default function ErrorState({ message, onRetry }: ErrorStateProps) {
  return (
    <div className="flex flex-col items-center gap-3 py-10 text-center" role="alert">
      <p className="text-sm text-error">{message}</p>
      <button className="btn btn-sm" onClick={onRetry}>
        Retry
      </button>
    </div>
  )
}
