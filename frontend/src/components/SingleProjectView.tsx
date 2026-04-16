import { useState, useRef, useEffect, useCallback } from 'react'
import { ChevronLeft, ChevronRight, Pencil, Check, X } from 'lucide-react'
import Terminal, { type TerminalHandle } from './Terminal'
import TerminalTabBar from './TerminalTabBar'
import ConfirmDialog from './ConfirmDialog'
import TodoList from './TodoList'
import { listTerminals, createTerminal, deleteTerminal, hasProcess } from '../api'
import type { Container, TerminalInfo } from '../api'

interface SingleProjectViewProps {
  container: Container
  onClose: (id: string) => void
  onRename: (id: string, name: string) => void
  onStatusChange: (id: string, connected: boolean) => void
  todoRefreshKey?: number
  terminalRefreshKey?: number
  canGoLeft: boolean
  canGoRight: boolean
  onNavigateLeft: () => void
  onNavigateRight: () => void
}

const MAX_TODO_RATIO = 2 / 3
const DEFAULT_TODO_RATIO = 0.25

export default function SingleProjectView({
  container,
  onClose,
  onRename,
  onStatusChange,
  todoRefreshKey,
  terminalRefreshKey,
  canGoLeft,
  canGoRight,
  onNavigateLeft,
  onNavigateRight,
}: SingleProjectViewProps) {
  const [editing, setEditing] = useState(false)
  const [name, setName] = useState(container.name)
  const [todoRatio, setTodoRatio] = useState(DEFAULT_TODO_RATIO)
  const inputRef = useRef<HTMLInputElement>(null)
  const bodyRef = useRef<HTMLDivElement>(null)
  const draggingRef = useRef(false)

  // Terminal tabs state
  const [terminals, setTerminals] = useState<TerminalInfo[]>([])
  const [activeTerminalId, setActiveTerminalId] = useState<string>('')
  const [confirmClose, setConfirmClose] = useState<string | null>(null)
  const terminalRefs = useRef<Map<string, TerminalHandle>>(new Map())

  useEffect(() => {
    setName(container.name)
  }, [container.name])

  useEffect(() => {
    listTerminals(container.id).then((terms) => {
      setTerminals(terms)
      if (terms.length > 0 && !activeTerminalId) {
        setActiveTerminalId(terms.find(t => t.isDefault)?.id ?? terms[0].id)
      }
    })
  }, [container.id, terminalRefreshKey])

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

  const handleSplitMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault()
    draggingRef.current = true

    const onMouseMove = (ev: MouseEvent) => {
      if (!draggingRef.current || !bodyRef.current) return
      const rect = bodyRef.current.getBoundingClientRect()
      const x = ev.clientX - rect.left
      const ratio = x / rect.width
      setTodoRatio(Math.min(MAX_TODO_RATIO, Math.max(0, ratio)))
    }

    const onMouseUp = () => {
      draggingRef.current = false
      document.removeEventListener('mousemove', onMouseMove)
      document.removeEventListener('mouseup', onMouseUp)
    }

    document.addEventListener('mousemove', onMouseMove)
    document.addEventListener('mouseup', onMouseUp)
  }, [])

  const handleCreateTerminal = async () => {
    try {
      const term = await createTerminal(container.id)
      setTerminals(prev => [...prev, term])
      setActiveTerminalId(term.id)
    } catch (e) {
      console.error('create terminal failed:', e)
    }
  }

  const handleCloseTerminal = async (tid: string) => {
    try {
      const hasProc = await hasProcess(container.id, tid)
      if (hasProc) {
        setConfirmClose(tid)
        return
      }
      await doCloseTerminal(tid)
    } catch {
      setConfirmClose(tid)
    }
  }

  const doCloseTerminal = async (tid: string) => {
    try {
      await deleteTerminal(container.id, tid)
      setTerminals(prev => {
        const remaining = prev.filter(t => t.id !== tid)
        if (activeTerminalId === tid && remaining.length > 0) {
          const oldIdx = prev.findIndex(t => t.id === tid)
          const newIdx = Math.min(oldIdx, remaining.length - 1)
          setActiveTerminalId(remaining[newIdx].id)
        }
        return remaining
      })
    } catch (e) {
      console.error('delete terminal failed:', e)
    }
    setConfirmClose(null)
  }

  const todoHidden = todoRatio < 0.02

  return (
    <div className="flex flex-col h-full">
      {/* Title bar with navigation */}
      <div className="flex items-center h-10 px-4 shrink-0 border-b border-gray-800 bg-[#0c0c0e]">
        <button
          onClick={onNavigateLeft}
          disabled={!canGoLeft}
          className="p-1 text-gray-500 hover:text-gray-300 disabled:text-gray-800 disabled:cursor-not-allowed"
        >
          <ChevronLeft size={18} />
        </button>

        <div className="flex-1 flex items-center justify-center gap-2 min-w-0">
          <span className={`w-2 h-2 rounded-full shrink-0 ${container.connected ? 'bg-emerald-500' : 'bg-gray-600'}`} />
          {editing ? (
            <form
              className="flex items-center gap-1 min-w-0"
              onSubmit={(e) => { e.preventDefault(); commitRename() }}
            >
              <input
                ref={inputRef}
                value={name}
                onChange={(e) => setName(e.target.value)}
                onBlur={commitRename}
                className="min-w-0 bg-transparent text-sm text-gray-200 outline-none border-b border-gray-600 py-0.5 text-center"
                maxLength={50}
              />
              <button type="submit" className="text-gray-500 hover:text-gray-300 p-0.5">
                <Check size={14} />
              </button>
            </form>
          ) : (
            <>
              <span className="text-sm text-gray-300 truncate">{container.name}</span>
              <button
                onClick={() => setEditing(true)}
                className="text-gray-600 hover:text-gray-400 p-0.5 shrink-0"
              >
                <Pencil size={12} />
              </button>
            </>
          )}
        </div>

        <button
          onClick={onNavigateRight}
          disabled={!canGoRight}
          className="p-1 text-gray-500 hover:text-gray-300 disabled:text-gray-800 disabled:cursor-not-allowed"
        >
          <ChevronRight size={18} />
        </button>
        <button
          onClick={() => onClose(container.id)}
          className="ml-2 text-gray-600 hover:text-red-400 p-0.5"
        >
          <X size={16} />
        </button>
      </div>

      {/* Body: todo + splitter + terminal tabs + terminal */}
      <div ref={bodyRef} className="flex flex-1 min-h-0">
        {!todoHidden && (
          <div
            className="shrink-0 border-r border-gray-800 flex flex-col overflow-hidden"
            style={{ width: `${todoRatio * 100}%` }}
          >
            <TodoList containerID={container.id} refreshKey={todoRefreshKey} />
          </div>
        )}
        <div
          className="w-1 shrink-0 flex items-center justify-center cursor-col-resize hover:bg-gray-700/50 active:bg-gray-600/50 transition-colors"
          onMouseDown={handleSplitMouseDown}
        >
          <div className="w-0.5 h-8 rounded-full bg-gray-700" />
        </div>
        <div className="flex-1 min-w-0 min-h-0 flex flex-col">
          {terminals.length > 0 && (
            <TerminalTabBar
              terminals={terminals}
              activeId={activeTerminalId}
              onSelect={setActiveTerminalId}
              onCreate={handleCreateTerminal}
              onClose={handleCloseTerminal}
            />
          )}
          <div className="flex-1 min-h-0 relative">
            {terminals.map((t) => (
              <div
                key={t.id}
                className="absolute inset-0"
                style={{ display: t.id === activeTerminalId ? 'block' : 'none' }}
              >
                <Terminal
                  ref={(handle) => {
                    if (handle) terminalRefs.current.set(t.id, handle)
                    else terminalRefs.current.delete(t.id)
                  }}
                  containerId={container.id}
                  terminalId={t.id}
                  connected={t.connected}
                  active={t.id === activeTerminalId}
                  isDefault={t.isDefault}
                  onReconnected={() => onStatusChange(container.id, true)}
                />
              </div>
            ))}
          </div>
        </div>
      </div>

      <ConfirmDialog
        open={confirmClose !== null}
        title="Close terminal?"
        message="This terminal has a running process. Are you sure you want to close it?"
        onConfirm={() => confirmClose && doCloseTerminal(confirmClose)}
        onCancel={() => setConfirmClose(null)}
      />
    </div>
  )
}
