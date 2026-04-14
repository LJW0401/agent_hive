import { useEffect, useState, useCallback, useMemo } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import {
  DndContext,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
  DragOverlay,
  type DragStartEvent,
  PointerSensor as LibPointerSensor,
} from '@dnd-kit/core'
import {
  SortableContext,
  rectSortingStrategy,
  useSortable,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import ProjectContainer from './components/ProjectContainer'
import NewProjectSlot from './components/NewProjectSlot'
import LoginPage from './components/LoginPage'
import {
  listContainers,
  createContainer,
  deleteContainer,
  renameContainer,
  getLayout,
  updateLayout,
  checkAuth,
  claimSession,
  setAuthToken,
  getAuthToken,
  type Container,
  type LayoutEntry,
} from './api'

const PAGE_SIZE = 4

// Custom PointerSensor that only activates when the event originates from a [data-drag-handle] element
class DragHandlePointerSensor extends LibPointerSensor {
  static activators = [
    {
      eventName: 'onPointerDown' as const,
      handler: ({ nativeEvent }: { nativeEvent: PointerEvent }) => {
        let el = nativeEvent.target as Element | null
        while (el) {
          if (el.hasAttribute('data-drag-handle')) return true
          el = el.parentElement
        }
        return false
      },
    },
  ]
}

// Compact layout: repack all containers sequentially, filling each page before the next.
function compactLayout(entries: LayoutEntry[]): LayoutEntry[] {
  // Sort by original page then position to preserve relative order
  const sorted = [...entries].sort((a, b) => a.page - b.page || a.position - b.position)
  return sorted.map((e, i) => ({
    ...e,
    page: Math.floor(i / PAGE_SIZE),
    position: i % PAGE_SIZE,
  }))
}

export default function App() {
  const [authState, setAuthState] = useState<'loading' | 'login' | 'ready'>('loading')
  const [readOnly, setReadOnly] = useState(false)
  const [containers, setContainers] = useState<Container[]>([])
  const [layout, setLayout] = useState<LayoutEntry[]>([])
  const [currentPage, setCurrentPage] = useState(0)
  const [activeId, setActiveId] = useState<string | null>(null)
  const [direction, setDirection] = useState(0)

  // Counter that increments when todos change remotely, keyed by containerId
  const [todoRefresh, setTodoRefresh] = useState<Record<string, number>>({})

  const connectNotifyWS = useCallback(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const tokenParam = getAuthToken() ? `?token=${getAuthToken()}` : ''
    const ws = new WebSocket(`${protocol}//${window.location.host}/ws/notify${tokenParam}`)
    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data)
        if (msg.type === 'preempted') {
          setReadOnly(true)
        }
        if (msg.type === 'todos-updated' && msg.containerId) {
          setTodoRefresh((prev) => ({
            ...prev,
            [msg.containerId]: (prev[msg.containerId] ?? 0) + 1,
          }))
        }
      } catch { /* ignore */ }
    }
    ws.onclose = () => {
      setTimeout(connectNotifyWS, 3000)
    }
    return ws
  }, [])

  const loadData = useCallback(() => {
    Promise.all([listContainers(), getLayout()]).then(([cs, lo]) => {
      setContainers(cs)
      setLayout(lo)
    })
  }, [])

  useEffect(() => {
    checkAuth().then(async (auth) => {
      if (!auth.enabled) {
        // No auth configured — go straight in
        setAuthState('ready')
        loadData()
        return
      }
      if (!auth.valid) {
        // No valid token — show login
        setAuthState('login')
        return
      }
      // Valid token exists — claim active control (preempt others)
      await claimSession()
      setReadOnly(false)
      setAuthState('ready')
      loadData()
      connectNotifyWS()
    })
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  // Total pages: ceil(containers / PAGE_SIZE), plus 1 extra if exactly full. Min 1.
  const totalPages = useMemo(() => {
    const count = layout.length
    if (count === 0) return 1
    const needed = Math.ceil(count / PAGE_SIZE)
    // Add an extra empty page only when all needed pages are full
    return count % PAGE_SIZE === 0 ? needed + 1 : needed
  }, [layout])

  // Clamp currentPage when pages shrink
  useEffect(() => {
    if (currentPage >= totalPages) {
      setCurrentPage(Math.max(0, totalPages - 1))
    }
  }, [totalPages, currentPage])

  // Containers on the current page, sorted by position
  const pageSlots = useMemo(() => {
    const pageEntries = layout
      .filter((e) => e.page === currentPage)
      .sort((a, b) => a.position - b.position)

    const containerMap = new Map(containers.map((c) => [c.id, c]))
    const slots: (Container | null)[] = [null, null, null, null]

    for (const entry of pageEntries) {
      if (entry.position >= 0 && entry.position < PAGE_SIZE) {
        const c = containerMap.get(entry.containerId)
        if (c) slots[entry.position] = c
      }
    }
    return slots
  }, [containers, layout, currentPage])

  const sortableIds = useMemo(
    () => pageSlots.filter((c): c is Container => c !== null).map((c) => c.id),
    [pageSlots],
  )

  const sensors = useSensors(
    useSensor(DragHandlePointerSensor, { activationConstraint: { distance: 8 } }),
  )

  const handleCreate = useCallback(async () => {
    const c = await createContainer('New Project')
    setContainers((prev) => [...prev, c])
    const lo = await getLayout()
    setLayout(lo)
    const entry = lo.find((e) => e.containerId === c.id)
    if (entry) setCurrentPage(entry.page)
  }, [])

  const handleClose = useCallback(async (id: string) => {
    await deleteContainer(id)
    setContainers((prev) => prev.filter((c) => c.id !== id))
    // Remove from layout and compact
    const newLayout = compactLayout(
      layout.filter((e) => e.containerId !== id),
    )
    setLayout(newLayout)
    await updateLayout(newLayout)
  }, [layout])

  const handleRename = useCallback(async (id: string, name: string) => {
    await renameContainer(id, name)
    setContainers((prev) =>
      prev.map((c) => (c.id === id ? { ...c, name } : c)),
    )
  }, [])

  const handleStatusChange = useCallback((id: string, connected: boolean) => {
    setContainers((prev) =>
      prev.map((c) => (c.id === id ? { ...c, connected } : c)),
    )
  }, [])

  const handleDragStart = useCallback((event: DragStartEvent) => {
    setActiveId(event.active.id as string)
  }, [])

  const handleDragEnd = useCallback(
    async (event: DragEndEvent) => {
      setActiveId(null)
      const { active, over } = event
      if (!over || active.id === over.id) return

      const newLayout = [...layout]
      const activeEntry = newLayout.find(
        (e) => e.containerId === active.id && e.page === currentPage,
      )
      const overEntry = newLayout.find(
        (e) => e.containerId === over.id && e.page === currentPage,
      )

      if (activeEntry && overEntry) {
        const tempPos = activeEntry.position
        activeEntry.position = overEntry.position
        overEntry.position = tempPos
        setLayout(newLayout)
        await updateLayout(newLayout)
      }
    },
    [layout, currentPage],
  )

  const goToPage = useCallback(
    (page: number) => {
      setDirection(page > currentPage ? 1 : -1)
      setCurrentPage(page)
    },
    [currentPage],
  )

  const moveToPage = useCallback(
    async (containerId: string, targetPage: number) => {
      const newLayout = compactLayout(
        layout.map((e) => {
          if (e.containerId !== containerId) return e
          const taken = layout
            .filter((x) => x.page === targetPage && x.containerId !== containerId)
            .map((x) => x.position)
          let pos = 0
          while (taken.includes(pos) && pos < PAGE_SIZE) pos++
          return { ...e, page: targetPage, position: pos }
        }),
      )
      setLayout(newLayout)
      await updateLayout(newLayout)
    },
    [layout],
  )

  const activeContainer = activeId
    ? containers.find((c) => c.id === activeId) ?? null
    : null

  const handleLogin = useCallback((token: string) => {
    setAuthToken(token)
    setAuthState('ready')
    loadData()
    connectNotifyWS()
  }, [loadData, connectNotifyWS])

  if (authState === 'loading') {
    return <div className="flex items-center justify-center h-screen bg-[#0a0a0b] text-gray-500 text-sm">Loading...</div>
  }

  if (authState === 'login') {
    return <LoginPage onLogin={handleLogin} />
  }

  return (
    <div className="flex flex-col h-screen bg-[#0a0a0b]">
      {readOnly && (
        <div className="bg-yellow-900/30 border-b border-yellow-800 px-4 py-1 text-center text-[11px] text-yellow-400">
          Read-only mode — another device has control
        </div>
      )}
      <header className="flex items-center px-4 h-11 border-b border-gray-800 shrink-0">
        <h1 className="text-sm font-semibold text-gray-200 tracking-wide">
          Agent Hive
        </h1>
        <span className="ml-3 text-[10px] text-gray-600">
          {containers.length} project{containers.length !== 1 ? 's' : ''}
        </span>

        <div className="ml-auto flex items-center gap-1">
          <button
            onClick={() => goToPage(Math.max(0, currentPage - 1))}
            disabled={currentPage === 0}
            className="p-1 text-gray-500 hover:text-gray-300 disabled:text-gray-800 disabled:cursor-not-allowed"
          >
            <ChevronLeft size={16} />
          </button>

          {Array.from({ length: totalPages }).map((_, i) => (
            <button
              key={i}
              onClick={() => goToPage(i)}
              className={`w-2 h-2 rounded-full transition-colors ${
                i === currentPage
                  ? 'bg-gray-300'
                  : 'bg-gray-700 hover:bg-gray-500'
              }`}
            />
          ))}

          <button
            onClick={() => goToPage(currentPage + 1)}
            disabled={currentPage >= totalPages - 1}
            className="p-1 text-gray-500 hover:text-gray-300 disabled:text-gray-800 disabled:cursor-not-allowed"
          >
            <ChevronRight size={16} />
          </button>
        </div>
      </header>

      <main className="flex-1 min-h-0 overflow-hidden relative">
        <DndContext
          sensors={sensors}
          collisionDetection={closestCenter}
          onDragStart={handleDragStart}
          onDragEnd={handleDragEnd}
        >
          <AnimatePresence initial={false} mode="wait" custom={direction}>
            <motion.div
              key={currentPage}
              custom={direction}
              initial={(d: number) => ({ x: d > 0 ? '50%' : '-50%', opacity: 0 })}
              animate={{ x: 0, opacity: 1 }}
              exit={(d: number) => ({ x: d > 0 ? '-50%' : '50%', opacity: 0 })}
              transition={{ duration: 0.2, ease: 'easeInOut' }}
              className="absolute inset-0 p-2"
            >
              <SortableContext items={sortableIds} strategy={rectSortingStrategy}>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-2 h-full grid-rows-2">
                  {pageSlots.map((container, idx) =>
                    container ? (
                      <SortableGridItem key={container.id} id={container.id}>
                        {(dragHandleProps) => (
                          <ProjectContainer
                            container={container}
                            onClose={handleClose}
                            onRename={handleRename}
                            onStatusChange={handleStatusChange}
                            onReadOnly={() => setReadOnly(true)}
                            readOnly={readOnly}
                            todoRefreshKey={todoRefresh[container.id] ?? 0}
                            currentPage={currentPage}
                            totalPages={totalPages}
                            onMoveToPage={moveToPage}
                            dragHandleProps={dragHandleProps}
                          />
                        )}
                      </SortableGridItem>
                    ) : (
                      <NewProjectSlot key={`empty-${idx}`} onClick={handleCreate} />
                    ),
                  )}
                </div>
              </SortableContext>
            </motion.div>
          </AnimatePresence>

          <DragOverlay>
            {activeContainer ? (
              <div className="rounded-lg border border-gray-600 bg-[#111114] opacity-80 w-full h-48">
                <div className="flex items-center h-9 px-3 border-b border-gray-800 bg-[#0c0c0e] rounded-t-lg">
                  <span className="text-xs text-gray-300">{activeContainer.name}</span>
                </div>
              </div>
            ) : null}
          </DragOverlay>
        </DndContext>
      </main>
    </div>
  )
}

function SortableGridItem({
  id,
  children,
}: {
  id: string
  children: (dragHandleProps: Record<string, unknown>) => React.ReactNode
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } =
    useSortable({ id })

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.3 : 1,
  }

  return (
    <div ref={setNodeRef} style={{ ...style, width: '100%', height: '100%' }}>
      {children({ ...attributes, ...listeners })}
    </div>
  )
}
