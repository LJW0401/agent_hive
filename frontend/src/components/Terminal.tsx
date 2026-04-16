import { useEffect, useRef, useState, forwardRef, useImperativeHandle } from 'react'
import { Terminal as XTerm } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { RotateCw } from 'lucide-react'
import { reopenContainer, getAuthToken } from '../api'
import '@xterm/xterm/css/xterm.css'

interface TerminalProps {
  containerId: string
  terminalId?: string
  connected: boolean
  active?: boolean
  isDefault?: boolean
  onReconnected: () => void
}

export interface TerminalHandle {
  sendData: (data: string) => void
}

const THEME = {
  background: '#111114',
  foreground: '#e5e7eb',
  cursor: '#e5e7eb',
  selectionBackground: '#374151',
  black: '#1f2937',
  red: '#ef4444',
  green: '#22c55e',
  yellow: '#eab308',
  blue: '#3b82f6',
  magenta: '#a855f7',
  cyan: '#06b6d4',
  white: '#e5e7eb',
  brightBlack: '#6b7280',
  brightRed: '#f87171',
  brightGreen: '#4ade80',
  brightYellow: '#facc15',
  brightBlue: '#60a5fa',
  brightMagenta: '#c084fc',
  brightCyan: '#22d3ee',
  brightWhite: '#f9fafb',
}

const Terminal = forwardRef<TerminalHandle, TerminalProps>(function Terminal(
  { containerId, terminalId, connected, active = true, isDefault = true, onReconnected },
  ref,
) {
  const containerRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<XTerm | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const fitAddonRef = useRef<FitAddon | null>(null)
  const [disconnected, setDisconnected] = useState(!connected)
  const [reopening, setReopening] = useState(false)
  const [mountKey, setMountKey] = useState(0)

  // For non-default terminals, track whether we should render based on active state
  const shouldConnect = isDefault || active

  useImperativeHandle(ref, () => ({
    sendData: (data: string) => {
      const ws = wsRef.current
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(new TextEncoder().encode(data))
      }
    },
  }))

  useEffect(() => {
    if (!containerRef.current || !shouldConnect) return

    const term = new XTerm({
      cursorBlink: true,
      fontSize: 13,
      fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', Menlo, Monaco, 'Courier New', monospace",
      theme: THEME,
    })

    const fitAddon = new FitAddon()
    term.loadAddon(fitAddon)
    term.open(containerRef.current)
    termRef.current = term
    fitAddonRef.current = fitAddon

    let ws: WebSocket | null = null
    let onDataDisposable: { dispose: () => void } | null = null
    let cancelled = false

    const rafId = requestAnimationFrame(() => {
      if (cancelled) return
      fitAddon.fit()

      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const tokenParam = getAuthToken() ? `&token=${getAuthToken()}` : ''
      const tidParam = terminalId ? `&tid=${terminalId}` : ''
      const wsUrl = `${protocol}//${window.location.host}/ws/terminal?id=${containerId}${tidParam}${tokenParam}`
      ws = new WebSocket(wsUrl)
      wsRef.current = ws
      ws.binaryType = 'arraybuffer'

      ws.onopen = () => {
        term.write('\x1bc')
        ws!.send(JSON.stringify({ type: 'resize', rows: term.rows, cols: term.cols }))
      }

      ws.onmessage = (event) => {
        if (typeof event.data === 'string') {
          try {
            const msg = JSON.parse(event.data)
            if (msg.type === 'status' && msg.connected === false) {
              setDisconnected(true)
              return
            }
          } catch { /* not JSON */ }
          term.write(event.data)
        } else {
          term.write(new Uint8Array(event.data))
        }
      }

      ws.onclose = () => {}
      ws.onerror = () => {}

      onDataDisposable = term.onData((data) => {
        if (ws && ws.readyState === WebSocket.OPEN) {
          ws.send(new TextEncoder().encode(data))
        }
      })
    })

    const handleResize = () => {
      fitAddon.fit()
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'resize', rows: term.rows, cols: term.cols }))
      }
    }
    const resizeObserver = new ResizeObserver(handleResize)
    resizeObserver.observe(containerRef.current)

    // Mobile touch scroll
    let touchStartY = 0
    let touchAccum = 0
    const lineHeight = Math.ceil(term.options.fontSize! * 1.2)

    const onTouchStart = (e: TouchEvent) => {
      touchStartY = e.touches[0].clientY
      touchAccum = 0
    }
    const onTouchMove = (e: TouchEvent) => {
      const dy = touchStartY - e.touches[0].clientY
      touchStartY = e.touches[0].clientY
      touchAccum += dy
      const lines = Math.trunc(touchAccum / lineHeight)
      if (lines !== 0) {
        term.scrollLines(lines)
        touchAccum -= lines * lineHeight
      }
    }
    const el = containerRef.current
    el.addEventListener('touchstart', onTouchStart, { passive: true })
    el.addEventListener('touchmove', onTouchMove, { passive: true })

    return () => {
      cancelled = true
      cancelAnimationFrame(rafId)
      el.removeEventListener('touchstart', onTouchStart)
      el.removeEventListener('touchmove', onTouchMove)
      resizeObserver.disconnect()
      onDataDisposable?.dispose()
      ws?.close()
      term.dispose()
      termRef.current = null
      wsRef.current = null
      fitAddonRef.current = null
    }
  }, [containerId, terminalId, mountKey, shouldConnect])

  const handleReopen = async () => {
    setReopening(true)
    try {
      await reopenContainer(containerId)
      setDisconnected(false)
      onReconnected()
      setMountKey((k) => k + 1)
    } catch (e) {
      console.error('reopen failed:', e)
    } finally {
      setReopening(false)
    }
  }

  // Non-default inactive terminal: render nothing
  if (!shouldConnect) {
    return <div className="w-full h-full bg-[#111114]" />
  }

  return (
    <div className="w-full h-full flex flex-col">
      <div ref={containerRef} className="flex-1 min-h-0" key={`${mountKey}-${shouldConnect}`} />
      {disconnected && (
        <div className="flex items-center justify-center gap-2 py-2 bg-[#0c0c0e] border-t border-gray-800">
          <span className="text-[10px] text-gray-500">Terminal disconnected</span>
          <button
            onClick={handleReopen}
            disabled={reopening}
            className="flex items-center gap-1 px-2 py-0.5 text-[10px] text-gray-400 hover:text-gray-200 bg-gray-800 hover:bg-gray-700 rounded transition-colors disabled:opacity-50"
          >
            <RotateCw size={10} className={reopening ? 'animate-spin' : ''} />
            {reopening ? 'Opening...' : 'Reopen'}
          </button>
        </div>
      )}
    </div>
  )
})

export default Terminal
