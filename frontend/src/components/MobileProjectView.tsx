import { useState, useRef, useEffect } from 'react'
import { X, Pencil, Check, ChevronLeft, ChevronRight } from 'lucide-react'
import Terminal from './Terminal'
import TodoList from './TodoList'
import type { Container } from '../api'

interface MobileProjectViewProps {
  container: Container
  onClose: (id: string) => void
  onRename: (id: string, name: string) => void
  onStatusChange: (id: string, connected: boolean) => void
  todoRefreshKey?: number
  index: number
  total: number
  onMoveLeft?: () => void
  onMoveRight?: () => void
}

export default function MobileProjectView({
  container,
  onClose,
  onRename,
  onStatusChange,
  todoRefreshKey,
  index,
  total,
  onMoveLeft,
  onMoveRight,
}: MobileProjectViewProps) {
  const [editing, setEditing] = useState(false)
  const [name, setName] = useState(container.name)
  const inputRef = useRef<HTMLInputElement>(null)
  const [splitRatio, setSplitRatio] = useState(0.5)
  const splitContainerRef = useRef<HTMLDivElement>(null)
  const draggingRef = useRef(false)

  useEffect(() => {
    setName(container.name)
  }, [container.name])

  useEffect(() => {
    if (editing) {
      inputRef.current?.focus()
      inputRef.current?.select()
    }
  }, [editing])

  const commitRename = () => {
    const trimmed = name.trim()
    if (trimmed && trimmed !== container.name) {
      onRename(container.id, trimmed)
    } else {
      setName(container.name)
    }
    setEditing(false)
  }

  // Split pane touch handlers
  const MIN_PX = 30

  const handleTouchStart = (e: React.TouchEvent) => {
    e.stopPropagation()
    draggingRef.current = true
  }

  const handleTouchMove = (e: React.TouchEvent) => {
    if (!draggingRef.current || !splitContainerRef.current) return
    e.stopPropagation()
    const rect = splitContainerRef.current.getBoundingClientRect()
    const y = e.touches[0].clientY - rect.top
    const total = rect.height
    const minRatio = MIN_PX / total
    const maxRatio = 1 - MIN_PX / total
    setSplitRatio(Math.min(maxRatio, Math.max(minRatio, y / total)))
  }

  const handleTouchEnd = () => {
    draggingRef.current = false
  }

  return (
    <div className="flex flex-col flex-1 min-h-0">
      {/* Header */}
      <div className="flex items-center h-10 px-3 shrink-0 border-b border-gray-800 bg-[#0c0c0e]">
        {editing ? (
          <form
            className="flex items-center gap-1 flex-1 min-w-0"
            onSubmit={(e) => { e.preventDefault(); commitRename() }}
          >
            <input
              ref={inputRef}
              value={name}
              onChange={(e) => setName(e.target.value)}
              onBlur={commitRename}
              className="flex-1 min-w-0 bg-transparent text-sm text-gray-200 outline-none border-b border-gray-600 py-0.5"
              maxLength={50}
            />
            <button type="submit" className="text-gray-500 hover:text-gray-300 p-1">
              <Check size={14} />
            </button>
          </form>
        ) : (
          <div className="flex items-center gap-1.5 flex-1 min-w-0">
            <span className={`w-2 h-2 rounded-full shrink-0 ${container.connected ? 'bg-emerald-500' : 'bg-gray-600'}`} />
            <span className="text-sm text-gray-300 truncate">{container.name}</span>
            <button
              onClick={() => setEditing(true)}
              className="text-gray-600 hover:text-gray-400 p-1 shrink-0"
            >
              <Pencil size={12} />
            </button>
          </div>
        )}
        <div className="flex items-center gap-1 ml-2 shrink-0">
          <button
            onClick={onMoveLeft}
            disabled={!onMoveLeft || index === 0}
            className="text-gray-600 hover:text-gray-400 disabled:text-gray-800 disabled:cursor-not-allowed p-1"
          >
            <ChevronLeft size={16} />
          </button>
          <span className="text-[10px] text-gray-600 min-w-[2rem] text-center">
            {total > 0 ? `${index + 1}/${total}` : ''}
          </span>
          <button
            onClick={onMoveRight}
            disabled={!onMoveRight || index >= total - 1}
            className="text-gray-600 hover:text-gray-400 disabled:text-gray-800 disabled:cursor-not-allowed p-1"
          >
            <ChevronRight size={16} />
          </button>
          <button
            onClick={() => onClose(container.id)}
            className="text-gray-600 hover:text-red-400 p-1"
          >
            <X size={16} />
          </button>
        </div>
      </div>

      {/* Split pane: terminal (top) + todo (bottom) */}
      <div
        ref={splitContainerRef}
        className="flex flex-col flex-1 min-h-0"
        onTouchMove={handleTouchMove}
        onTouchEnd={handleTouchEnd}
      >
        {/* Terminal */}
        <div style={{ height: `${splitRatio * 100}%` }} className="min-h-0 overflow-hidden">
          <Terminal
            containerId={container.id}
            connected={container.connected}
            onReconnected={() => onStatusChange(container.id, true)}
          />
        </div>

        {/* Drag handle */}
        <div
          className="h-2 shrink-0 flex items-center justify-center cursor-row-resize bg-[#0c0c0e] border-y border-gray-800 touch-none"
          onTouchStart={handleTouchStart}
        >
          <div className="w-8 h-0.5 rounded-full bg-gray-600" />
        </div>

        {/* Todo list */}
        <div style={{ height: `${(1 - splitRatio) * 100}%` }} className="min-h-0 overflow-hidden overscroll-contain">
          <TodoList containerID={container.id} refreshKey={todoRefreshKey} />
        </div>
      </div>
    </div>
  )
}
