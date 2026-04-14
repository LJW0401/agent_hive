import { useEffect, useState, useCallback, useRef } from 'react'
import { Swiper, SwiperSlide } from 'swiper/react'
import type { Swiper as SwiperType } from 'swiper'
import 'swiper/css'
import LoginPage from './components/LoginPage'
import MobileProjectView from './components/MobileProjectView'
import NewProjectSlot from './components/NewProjectSlot'
import {
  listContainers,
  createContainer,
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
  const [todoRefresh, setTodoRefresh] = useState<Record<string, number>>({})
  const swiperRef = useRef<SwiperType | null>(null)
  const pendingSlideIdRef = useRef<string | null>(null)

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
    for (const c of containers) {
      if (!sorted.find((s) => s.id === c.id)) {
        sorted.push(c)
      }
    }
    return sorted
  })()

  // Apply pending slide after data updates (find by container ID)
  useEffect(() => {
    if (pendingSlideIdRef.current && swiperRef.current) {
      const idx = sortedContainers.findIndex((c) => c.id === pendingSlideIdRef.current)
      if (idx >= 0) {
        pendingSlideIdRef.current = null
        setTimeout(() => swiperRef.current?.slideTo(idx, 0), 50)
      }
    }
  }, [sortedContainers])

  const handleLogin = useCallback((token: string) => {
    setAuthToken(token)
    setAuthState('ready')
    loadData()
    connectNotifyWS()
  }, [loadData, connectNotifyWS])

  const handleCreate = useCallback(async () => {
    try {
      const c = await createContainer('New Project')
      pendingSlideIdRef.current = c.id
      loadData()
    } catch (e) {
      alert('Failed to create project')
      console.error(e)
    }
  }, [loadData])

  const handleClose = useCallback(async (id: string) => {
    const currentSlide = swiperRef.current?.activeIndex ?? 0
    await deleteContainer(id)
    const newLayout = mobileLayout
      .filter((e) => e.containerId !== id)
      .map((e, i) => ({ ...e, sortOrder: i }))
    setMobileLayout(newLayout)
    await updateMobileLayout(newLayout)
    loadData()
    // Stay at previous project or clamp
    setTimeout(() => {
      const target = Math.min(currentSlide, newLayout.length - 1)
      swiperRef.current?.slideTo(Math.max(0, target))
    }, 100)
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

  const handleMoveLeft = useCallback(async (index: number) => {
    if (index <= 0) return
    const newLayout = [...mobileLayout].sort((a, b) => a.sortOrder - b.sortOrder)
    // Swap with previous
    const temp = newLayout[index]
    newLayout[index] = newLayout[index - 1]
    newLayout[index - 1] = temp
    const reindexed = newLayout.map((e, i) => ({ ...e, sortOrder: i }))
    setMobileLayout(reindexed)
    await updateMobileLayout(reindexed)
    loadData()
    setTimeout(() => swiperRef.current?.slideTo(index - 1), 100)
  }, [mobileLayout, loadData])

  const handleMoveRight = useCallback(async (index: number) => {
    const sorted = [...mobileLayout].sort((a, b) => a.sortOrder - b.sortOrder)
    if (index >= sorted.length - 1) return
    // Swap with next
    const temp = sorted[index]
    sorted[index] = sorted[index + 1]
    sorted[index + 1] = temp
    const reindexed = sorted.map((e, i) => ({ ...e, sortOrder: i }))
    setMobileLayout(reindexed)
    await updateMobileLayout(reindexed)
    loadData()
    setTimeout(() => swiperRef.current?.slideTo(index + 1), 100)
  }, [mobileLayout, loadData])

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
      <Swiper
        onSwiper={(s) => { swiperRef.current = s }}
        spaceBetween={0}
        slidesPerView={1}
        touchAngle={45}
        className="flex-1 w-full"
      >
        {sortedContainers.map((container, idx) => (
          <SwiperSlide key={container.id} className="!flex flex-col h-full">
            <MobileProjectView
              container={container}
              onClose={handleClose}
              onRename={handleRename}
              onStatusChange={handleStatusChange}
              todoRefreshKey={todoRefresh[container.id] ?? 0}
              index={idx}
              total={sortedContainers.length}
              onMoveLeft={() => handleMoveLeft(idx)}
              onMoveRight={() => handleMoveRight(idx)}
            />
          </SwiperSlide>
        ))}
        <SwiperSlide className="!flex flex-col h-full p-4">
          <NewProjectSlot onClick={handleCreate} />
        </SwiperSlide>
      </Swiper>
    </div>
  )
}
