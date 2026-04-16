interface ConfirmDialogProps {
  open: boolean
  title: string
  message: string
  onConfirm: () => void
  onCancel: () => void
}

export default function ConfirmDialog({ open, title, message, onConfirm, onCancel }: ConfirmDialogProps) {
  if (!open) return null

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60"
      onClick={onCancel}
    >
      <div
        className="bg-[#1a1a1e] border border-gray-700 rounded-lg p-4 w-[90vw] max-w-sm"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="text-sm font-medium text-gray-200 mb-2">{title}</div>
        <div className="text-xs text-gray-400 mb-4">{message}</div>
        <div className="flex justify-end gap-2">
          <button
            onClick={onCancel}
            className="px-3 py-1.5 text-xs text-gray-400 hover:text-gray-200 bg-gray-800 hover:bg-gray-700 rounded transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={onConfirm}
            className="px-3 py-1.5 text-xs text-gray-200 bg-red-700 hover:bg-red-600 rounded transition-colors"
          >
            Confirm
          </button>
        </div>
      </div>
    </div>
  )
}
