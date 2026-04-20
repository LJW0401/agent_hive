import { isMobile } from './utils/device'
import MobileApp from './MobileApp'
import { useEffect, useState, useCallback, useMemo, useRef } from 'react'
import { detectEdgeZone, EDGE_DWELL_MS, EDGE_THRESHOLD_PX, type EdgeZone } from './utils/dragEdge'
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
import { ChevronLeft, ChevronRight, LayoutGrid, Maximize2 } from 'lucide-react'
import ProjectContainer from './components/ProjectContainer'
import SingleProjectView from './components/SingleProjectView'
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
  setAuthToken,
  getAuthToken,
  type Container,
  type LayoutEntry,
} from './api'

const PAGE_SIZE = 4
const LAYOUT_KEY = 'agent_hive_layout_mode'

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
  if (isMobile()) return <MobileApp />
  return <DesktopApp />
}

function DesktopApp() {
  const [authState, setAuthState] = useState<'loading' | 'login' | 'ready'>('loading')
  const [containers, setContainers] = useState<Container[]>([])
  const [layout, setLayout] = useState<LayoutEntry[]>([])
  const [currentPage, setCurrentPage] = useState(0)
  const [activeId, setActiveId] = useState<string | null>(null)
  const [direction, setDirection] = useState(0)
  const [edgeZone, setEdgeZone] = useState<EdgeZone>(null)
  const edgeZoneRef = useRef<EdgeZone>(null)
  const edgeTimerRef = useRef<number | null>(null)
  const [todoRefresh, setTodoRefresh] = useState<Record<string, number>>({})
  const [terminalRefresh, setTerminalRefresh] = useState<Record<string, number>>({})

  const [layoutMode, setLayoutMode] = useState<'multi' | 'single'>(() => {
    const saved = localStorage.getItem(LAYOUT_KEY)
    return saved === 'single' ? 'single' : 'multi'
  })
  const [focusedContainerId, setFocusedContainerId] = useState<string | null>(null)
  const [singleDirection, setSingleDirection] = useState(0)

  const toggleLayout = useCallback(() => {
    setLayoutMode(prev => {
      const next = prev === 'multi' ? 'single' : 'multi'
      localStorage.setItem(LAYOUT_KEY, next)
      return next
    })
  }, [])

  // For single mode: determine which container to show
  const singleContainerId = useMemo(() => {
    if (focusedContainerId && containers.find(c => c.id === focusedContainerId)) {
      return focusedContainerId
    }
    return containers[0]?.id ?? null
  }, [focusedContainerId, containers])

  // Sorted container list for single-mode navigation
  const sortedContainerIds = useMemo(() => {
    const sorted = [...layout].sort((a, b) => a.page - b.page || a.position - b.position)
    return sorted.map(e => e.containerId).filter(id => containers.some(c => c.id === id))
  }, [layout, containers])

  const navigateSingle = useCallback((delta: number) => {
    if (!singleContainerId) return
    const idx = sortedContainerIds.indexOf(singleContainerId)
    const next = idx + delta
    if (next >= 0 && next < sortedContainerIds.length) {
      setSingleDirection(delta)
      setFocusedContainerId(sortedContainerIds[next])
    }
  }, [singleContainerId, sortedContainerIds])

  const connectNotifyWS = useCallback(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const tokenParam = getAuthToken() ? `?token=${getAuthToken()}` : ''
    const ws = new WebSocket(`${protocol}//${window.location.host}/ws/notify${tokenParam}`)
    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data)
        if (msg.type === 'todos-updated' && msg.containerId) {
          setTodoRefresh((prev) => ({
            ...prev,
            [msg.containerId]: (prev[msg.containerId] ?? 0) + 1,
          }))
        }
        if (msg.type === 'terminals-changed' && msg.containerId) {
          setTerminalRefresh((prev) => ({
            ...prev,
            [msg.containerId]: (prev[msg.containerId] ?? 0) + 1,
          }))
        }
        if (msg.type === 'containers-changed') {
          loadData()
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
    checkAuth().then((auth) => {
      if (!auth.enabled) {
        setAuthState('ready')
        loadData()
        connectNotifyWS()
        return
      }
      if (!auth.valid) {
        setAuthState('login')
        return
      }
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

  // Keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Don't capture when typing in inputs
      const tag = (e.target as HTMLElement).tagName
      if (tag === 'INPUT' || tag === 'TEXTAREA') return

      const mod = e.ctrlKey || e.metaKey

      // Ctrl/Cmd + N → new container
      if (mod && e.key === 'n') {
        e.preventDefault()
        handleCreate()
      }
      // Ctrl/Cmd + ArrowLeft/Right
      if (mod && e.key === 'ArrowLeft') {
        e.preventDefault()
        if (layoutMode === 'single') {
          navigateSingle(-1)
        } else {
          if (currentPage > 0) goToPage(currentPage - 1)
        }
      }
      if (mod && e.key === 'ArrowRight') {
        e.preventDefault()
        if (layoutMode === 'single') {
          navigateSingle(1)
        } else {
          if (currentPage < totalPages - 1) goToPage(currentPage + 1)
        }
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [currentPage, totalPages, handleCreate, goToPage, layoutMode, navigateSingle])

  // Reset edge-dwell state whenever we leave a dwell window (drag ends, zone change, page jump, etc.)
  const clearEdgeDwell = useCallback(() => {
    if (edgeTimerRef.current !== null) {
      window.clearTimeout(edgeTimerRef.current)
      edgeTimerRef.current = null
    }
    edgeZoneRef.current = null
    setEdgeZone(null)
  }, [])

  // Drag-to-page-flip: while an item is being dragged, watch the pointer X.
  // Entering an edge zone shows a marker and starts a dwell timer; after
  // EDGE_DWELL_MS we move the active container to the neighbouring page and
  // navigate to it. The ongoing drag is intentionally cancelled by the page
  // remount (see learnings: "change of rendering topology is outside S scope").
  useEffect(() => {
    if (!activeId) {
      clearEdgeDwell()
      return
    }
    const handlePointerMove = (e: PointerEvent) => {
      const rawZone = detectEdgeZone(e.clientX, window.innerWidth, EDGE_THRESHOLD_PX)
      // Suppress the marker when there is no page to flip to in that direction.
      const canLeft = currentPage > 0
      const canRight = currentPage < totalPages - 1
      const zone: EdgeZone =
        rawZone === 'left' ? (canLeft ? 'left' : null)
        : rawZone === 'right' ? (canRight ? 'right' : null)
        : null

      if (zone === edgeZoneRef.current) return
      edgeZoneRef.current = zone
      setEdgeZone(zone)
      if (edgeTimerRef.current !== null) {
        window.clearTimeout(edgeTimerRef.current)
        edgeTimerRef.current = null
      }
      if (zone !== null) {
        const target = zone === 'left' ? currentPage - 1 : currentPage + 1
        const containerId = activeId
        edgeTimerRef.current = window.setTimeout(() => {
          moveToPage(containerId, target)
          goToPage(target)
          clearEdgeDwell()
        }, EDGE_DWELL_MS)
      }
    }
    window.addEventListener('pointermove', handlePointerMove)
    return () => {
      window.removeEventListener('pointermove', handlePointerMove)
      clearEdgeDwell()
    }
  }, [activeId, currentPage, totalPages, moveToPage, goToPage, clearEdgeDwell])

  const handleLogin = useCallback((token: string) => {
    setAuthToken(token)
    setAuthState('ready')
    loadData()
    connectNotifyWS()
  }, [loadData, connectNotifyWS])

  if (authState === 'loading') {
    return (
      <div className="flex flex-col items-center justify-center h-screen bg-[#0a0a0b] gap-3">
        <div className="w-6 h-6 border-2 border-gray-700 border-t-gray-400 rounded-full animate-spin" />
        <span className="text-gray-500 text-sm">Connecting...</span>
      </div>
    )
  }

  if (authState === 'login') {
    return <LoginPage onLogin={handleLogin} />
  }

  return (
    <div className="flex flex-col h-screen bg-[#0a0a0b]">
      <header className="flex items-center px-4 h-11 border-b border-gray-800 shrink-0">
        <h1 className="text-sm font-semibold text-gray-200 tracking-wide">
          Agent Hive
        </h1>
        <span className="ml-3 text-[10px] text-gray-600">
          {containers.length} project{containers.length !== 1 ? 's' : ''}
        </span>

        <div className="ml-auto flex items-center gap-1">
          <button
            onClick={toggleLayout}
            className="p-1 text-gray-500 hover:text-gray-300 mr-2"
            title={layoutMode === 'multi' ? 'Switch to single project' : 'Switch to grid'}
          >
            {layoutMode === 'multi' ? <Maximize2 size={15} /> : <LayoutGrid size={15} />}
          </button>

          {layoutMode === 'multi' && (
            <>
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
            </>
          )}
        </div>
      </header>

      <main className="flex-1 min-h-0 overflow-hidden relative" onClick={(e) => {
        // Track focused container for single-mode switching
        const el = (e.target as HTMLElement).closest('[data-container-id]')
        if (el) setFocusedContainerId(el.getAttribute('data-container-id'))
      }}>
        {layoutMode === 'single' && singleContainerId ? (
          <AnimatePresence initial={false} mode="wait" custom={singleDirection}>
            {(() => {
              const c = containers.find(c => c.id === singleContainerId)
              if (!c) return null
              const idx = sortedContainerIds.indexOf(singleContainerId)
              return (
                <motion.div
                  key={singleContainerId}
                  custom={singleDirection}
                  variants={{
                    enter: (d: number) => ({ x: d > 0 ? '50%' : '-50%', opacity: 0 }),
                    center: { x: 0, opacity: 1 },
                    exit: (d: number) => ({ x: d > 0 ? '-50%' : '50%', opacity: 0 }),
                  }}
                  initial="enter"
                  animate="center"
                  exit="exit"
                  transition={{ duration: 0.2, ease: 'easeInOut' }}
                  className="absolute inset-0"
                >
                  <SingleProjectView
                    container={c}
                    onClose={handleClose}
                    onRename={handleRename}
                    onStatusChange={handleStatusChange}
                    todoRefreshKey={todoRefresh[c.id] ?? 0}
                    terminalRefreshKey={terminalRefresh[c.id] ?? 0}
                    canGoLeft={idx > 0}
                    canGoRight={idx < sortedContainerIds.length - 1}
                    onNavigateLeft={() => navigateSingle(-1)}
                    onNavigateRight={() => navigateSingle(1)}
                  />
                </motion.div>
              )
            })()}
          </AnimatePresence>
        ) : (
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
                variants={{
                  enter: (d: number) => ({ x: d > 0 ? '50%' : '-50%', opacity: 0 }),
                  center: { x: 0, opacity: 1 },
                  exit: (d: number) => ({ x: d > 0 ? '-50%' : '50%', opacity: 0 }),
                }}
                initial="enter"
                animate="center"
                exit="exit"
                transition={{ duration: 0.2, ease: 'easeInOut' }}
                className="absolute inset-0 p-2"
              >
                <SortableContext items={sortableIds} strategy={rectSortingStrategy}>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-2 h-full grid-rows-2">
                    {pageSlots.map((container, idx) =>
                      container ? (
                        <SortableGridItem key={container.id} id={container.id}>
                          {(dragHandleProps) => (
                            <div data-container-id={container.id} className="h-full">
                              <ProjectContainer
                                container={container}
                                onClose={handleClose}
                                onRename={handleRename}
                                onStatusChange={handleStatusChange}
                                todoRefreshKey={todoRefresh[container.id] ?? 0}
                                terminalRefreshKey={terminalRefresh[container.id] ?? 0}
                                currentPage={currentPage}
                                totalPages={totalPages}
                                onMoveToPage={moveToPage}
                                dragHandleProps={dragHandleProps}
                              />
                            </div>
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

            <AnimatePresence>
              {edgeZone === 'left' && (
                <motion.div
                  key="edge-left"
                  data-edge-marker="left"
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  exit={{ opacity: 0 }}
                  transition={{ duration: 0.12 }}
                  className="pointer-events-none absolute inset-y-0 left-0 z-40 flex items-center justify-start"
                  style={{ width: EDGE_THRESHOLD_PX }}
                >
                  <div className="w-full h-full bg-gradient-to-r from-sky-500/40 via-sky-500/15 to-transparent animate-pulse" />
                </motion.div>
              )}
              {edgeZone === 'right' && (
                <motion.div
                  key="edge-right"
                  data-edge-marker="right"
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  exit={{ opacity: 0 }}
                  transition={{ duration: 0.12 }}
                  className="pointer-events-none absolute inset-y-0 right-0 z-40 flex items-center justify-end"
                  style={{ width: EDGE_THRESHOLD_PX }}
                >
                  <div className="w-full h-full bg-gradient-to-l from-sky-500/40 via-sky-500/15 to-transparent animate-pulse" />
                </motion.div>
              )}
            </AnimatePresence>
          </DndContext>
        )}
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
