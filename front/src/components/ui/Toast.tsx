import { useEffect } from "react";

export interface ToastData {
  message: string;
  kind: "success" | "error";
  action?: { label: string; onAction: () => void };
}

interface ToastProps {
  toast: ToastData;
  onDismiss: () => void;
}

// UI.md notifications: bottom-right, auto-dismiss 4s, success/error only.
export default function Toast({ toast, onDismiss }: ToastProps) {
  useEffect(() => {
    const timer = setTimeout(onDismiss, 4000);
    return () => clearTimeout(timer);
  }, [toast, onDismiss]);

  const alertClass = toast.kind === "success" ? "alert-success" : "alert-error";

  return (
    <div className="toast toast-end z-50" role="status">
      <div className={`alert py-2 shadow-lg ${alertClass}`}>
        <span className="text-sm">{toast.message}</span>
        {toast.action !== undefined && (
          <button
            type="button"
            className="btn btn-ghost btn-xs"
            onClick={toast.action.onAction}
          >
            {toast.action.label}
          </button>
        )}
        <button
          type="button"
          aria-label="Dismiss"
          className="btn btn-ghost btn-xs"
          onClick={onDismiss}
        >
          ×
        </button>
      </div>
    </div>
  );
}
