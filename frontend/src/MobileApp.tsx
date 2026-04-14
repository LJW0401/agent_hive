import { useEffect, useState, useCallback } from 'react'
import LoginPage from './components/LoginPage'
import MobileProjectView from './components/MobileProjectView'
import {
  listContainers,
  getMobileLayout,
  checkAuth,
  setAuthToken,
  getAuthToken,
  deleteContainer,
  renameContainer,
  updateMobileLayout,
  type Container,
  type MobileLayoutEntry,
} from './api'

export default function MobileApp() {
  const [authState, setAuthState] = useState<'loading' | 'login' | 'ready'>('loading')
  const [containers, setContainers] = useState<Container[]>([])
  const [mobileLayout, setMobileLayout] = useState<MobileLayoutEntry[]>([])
  const [currentIndex, setCurrentIndex] = useState(0)
  const [todoRefresh, setTodoRefresh] = useState<Record<string, number>>({})

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
    Promise.all([listContainers(), getMobileLayout()]).then(([cs, ml]) => {
      setContainers(cs)
      setMobileLayout(ml)
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

  // Build sorted container list from mobile layout
  const sortedContainers = (() => {
    const map = new Map(containers.map((c) => [c.id, c]))
    const sorted: Container[] = []
    for (const entry of [...mobileLayout].sort((a, b) => a.sortOrder - b.sortOrder)) {
      const c = map.get(entry.containerId)
      if (c) sorted.push(c)
    }
    // Append containers not in mobile layout (e.g. created before mobile layout existed)
    for (const c of containers) {
      if (!sorted.find((s) => s.id === c.id)) {
        sorted.push(c)
      }
    }
    return sorted
  })()

  // Clamp currentIndex
  useEffect(() => {
    if (sortedContainers.length === 0) {
      setCurrentIndex(0)
    } else if (currentIndex >= sortedContainers.length) {
      setCurrentIndex(sortedContainers.length - 1)
    }
  }, [sortedContainers.length, currentIndex])

  const handleLogin = useCallback((token: string) => {
    setAuthToken(token)
    setAuthState('ready')
    loadData()
    connectNotifyWS()
  }, [loadData, connectNotifyWS])

  const handleClose = useCallback(async (id: string) => {
    await deleteContainer(id)
    // Remove from mobile layout
    const newLayout = mobileLayout
      .filter((e) => e.containerId !== id)
      .map((e, i) => ({ ...e, sortOrder: i }))
    setMobileLayout(newLayout)
    await updateMobileLayout(newLayout)
    loadData()
  }, [mobileLayout, loadData])

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

  const currentContainer = sortedContainers[currentIndex] ?? null

  return (
    <div className="flex flex-col h-screen bg-[#0a0a0b]">
      {currentContainer ? (
        <MobileProjectView
          container={currentContainer}
          onClose={handleClose}
          onRename={handleRename}
          onStatusChange={handleStatusChange}
          todoRefreshKey={todoRefresh[currentContainer.id] ?? 0}
          index={currentIndex}
          total={sortedContainers.length}
        />
      ) : (
        <div className="flex items-center justify-center flex-1 text-gray-500">
          No projects
        </div>
      )}
    </div>
  )
}
