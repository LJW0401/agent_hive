import { useState, useRef, useEffect } from 'react'
import { X, Pencil, Check, ArrowLeft, ArrowRight } from 'lucide-react'
import Terminal from './Terminal'
import TodoList from './TodoList'
import type { Container } from '../api'

interface ProjectContainerProps {
  container: Container
  onClose: (id: string) => void
  onRename: (id: string, name: string) => void
  currentPage: number
  totalPages: number
  onMoveToPage: (containerId: string, page: number) => void
}

export default function ProjectContainer({ container, onClose, onRename, currentPage, totalPages, onMoveToPage }: ProjectContainerProps) {
  const [editing, setEditing] = useState(false)
  const [name, setName] = useState(container.name)
  const inputRef = useRef<HTMLInputElement>(null)

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

  return (
    <div className="flex flex-col h-full rounded-lg border border-gray-800 bg-[#111114] overflow-hidden">
      {/* Header */}
      <div className="flex items-center h-9 px-3 shrink-0 border-b border-gray-800 bg-[#0c0c0e]">
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
              className="flex-1 min-w-0 bg-transparent text-xs text-gray-200 outline-none border-b border-gray-600 py-0.5"
              maxLength={50}
            />
            <button type="submit" className="text-gray-500 hover:text-gray-300 p-0.5">
              <Check size={12} />
            </button>
          </form>
        ) : (
          <div className="flex items-center gap-1.5 flex-1 min-w-0">
            <span className="text-xs text-gray-300 truncate">{container.name}</span>
            <button
              onClick={() => setEditing(true)}
              className="text-gray-600 hover:text-gray-400 p-0.5 shrink-0"
            >
              <Pencil size={11} />
            </button>
          </div>
        )}
        <div className="flex items-center gap-0.5 ml-2 shrink-0">
          {currentPage > 0 && (
            <button
              onClick={() => onMoveToPage(container.id, currentPage - 1)}
              className="text-gray-700 hover:text-gray-400 p-0.5"
              title="Move to previous page"
            >
              <ArrowLeft size={11} />
            </button>
          )}
          {totalPages > 1 && currentPage < totalPages - 1 && (
            <button
              onClick={() => onMoveToPage(container.id, currentPage + 1)}
              className="text-gray-700 hover:text-gray-400 p-0.5"
              title="Move to next page"
            >
              <ArrowRight size={11} />
            </button>
          )}
          <button
            onClick={() => onClose(container.id)}
            className="text-gray-600 hover:text-red-400 p-0.5"
          >
            <X size={14} />
          </button>
        </div>
      </div>

      {/* Body: todo list + terminal */}
      <div className="flex flex-1 min-h-0">
        {/* Left: Todo area */}
        <div className="w-48 shrink-0 border-r border-gray-800 hidden lg:flex flex-col">
          <TodoList containerID={container.id} />
        </div>
        {/* Right: Terminal */}
        <div className="flex-1 min-w-0 min-h-0">
          <Terminal containerId={container.id} />
        </div>
      </div>
    </div>
  )
}
