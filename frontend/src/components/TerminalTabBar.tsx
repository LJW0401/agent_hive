import { Plus, X } from 'lucide-react'
import type { TerminalInfo } from '../api'

interface TerminalTabBarProps {
  terminals: TerminalInfo[]
  activeId: string
  onSelect: (id: string) => void
  onCreate: () => void
  onClose: (id: string) => void
  maxTerminals?: number
}

export default function TerminalTabBar({
  terminals,
  activeId,
  onSelect,
  onCreate,
  onClose,
  maxTerminals = 5,
}: TerminalTabBarProps) {
  return (
    <div className="flex items-center gap-0.5 px-1 py-0.5 bg-[#0c0c0e] border-b border-gray-800 overflow-x-auto shrink-0">
      {terminals.map((t) => (
        <button
          key={t.id}
          onClick={() => onSelect(t.id)}
          className={`flex items-center gap-1 shrink-0 px-2 py-0.5 text-[11px] rounded transition-colors select-none ${
            t.id === activeId
              ? 'bg-gray-700 text-gray-200'
              : 'text-gray-500 hover:text-gray-300 hover:bg-gray-800'
          }`}
        >
          <span>{t.name}</span>
          {!t.isDefault && (
            <span
              role="button"
              onClick={(e) => {
                e.stopPropagation()
                onClose(t.id)
              }}
              className="text-gray-600 hover:text-red-400 p-0.5 -mr-1"
            >
              <X size={10} />
            </span>
          )}
        </button>
      ))}
      {terminals.length < maxTerminals && (
        <button
          onClick={onCreate}
          className="shrink-0 p-0.5 text-gray-600 hover:text-gray-300 hover:bg-gray-800 rounded transition-colors"
          title="New terminal"
        >
          <Plus size={12} />
        </button>
      )}
    </div>
  )
}
