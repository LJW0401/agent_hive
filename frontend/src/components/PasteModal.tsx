import { useState, useRef, useEffect } from 'react'

interface PasteModalProps {
  onConfirm: (text: string) => void
  onClose: () => void
}

export default function PasteModal({ onConfirm, onClose }: PasteModalProps) {
  const [text, setText] = useState('')
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  useEffect(() => {
    textareaRef.current?.focus()
  }, [])

  const handleConfirm = () => {
    const trimmed = text.trim()
    if (trimmed) {
      onConfirm(trimmed)
    } else {
      onClose()
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60"
      onClick={onClose}
    >
      <div
        className="bg-[#1a1a1e] border border-gray-700 rounded-lg p-4 w-[90vw] max-w-sm"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="text-sm text-gray-300 mb-2">Paste content</div>
        <textarea
          ref={textareaRef}
          value={text}
          onChange={(e) => setText(e.target.value)}
          placeholder="Paste text here..."
          className="w-full h-24 bg-[#111114] text-gray-300 text-sm border border-gray-700 rounded p-2 outline-none resize-none placeholder-gray-600"
        />
        <div className="flex justify-end gap-2 mt-3">
          <button
            onClick={onClose}
            className="px-3 py-1.5 text-xs text-gray-400 hover:text-gray-200 bg-gray-800 hover:bg-gray-700 rounded transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={handleConfirm}
            className="px-3 py-1.5 text-xs text-gray-200 bg-emerald-700 hover:bg-emerald-600 rounded transition-colors"
          >
            Send
          </button>
        </div>
      </div>
    </div>
  )
}
