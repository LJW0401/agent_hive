import { useState } from 'react'
import PasteModal from './PasteModal'

interface ShortcutBarProps {
  onSend: (data: string) => void
}

const shortcuts = [
  { label: '^C', value: '\x03' },
  { label: '^L', value: '\x0c' },
  { label: 'Tab', value: '\t' },
  { label: 'Esc', value: '\x1b' },
  { label: '↑', value: '\x1b[A' },
  { label: '↓', value: '\x1b[B' },
  { label: '←', value: '\x1b[D' },
  { label: '→', value: '\x1b[C' },
  { label: 'Enter', value: '\r' },
]

export default function ShortcutBar({ onSend }: ShortcutBarProps) {
  const [pasteOpen, setPasteOpen] = useState(false)

  return (
    <>
      <div
        className="flex items-center gap-1 px-1.5 py-1 bg-[#0c0c0e] border-t border-gray-800 overflow-x-auto shrink-0"
        style={{ touchAction: 'pan-x' }}
        onTouchStart={(e) => e.stopPropagation()}
        onTouchMove={(e) => e.stopPropagation()}
      >
        {shortcuts.map((s) => (
          <button
            key={s.label}
            onTouchStart={(e) => e.stopPropagation()}
            onClick={() => onSend(s.value)}
            className="shrink-0 px-2.5 py-1 text-[11px] text-gray-400 bg-gray-800 hover:bg-gray-700 active:bg-gray-600 rounded transition-colors select-none"
          >
            {s.label}
          </button>
        ))}
        <button
          onTouchStart={(e) => e.stopPropagation()}
          onClick={() => setPasteOpen(true)}
          className="shrink-0 px-2.5 py-1 text-[11px] text-gray-400 bg-gray-800 hover:bg-gray-700 active:bg-gray-600 rounded transition-colors select-none"
        >
          Paste
        </button>
      </div>

      {pasteOpen && (
        <PasteModal
          onConfirm={(text) => {
            onSend(text)
            setPasteOpen(false)
          }}
          onClose={() => setPasteOpen(false)}
        />
      )}
    </>
  )
}
