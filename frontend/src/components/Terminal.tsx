import { useEffect, useRef, useState } from 'react'
import { Terminal as XTerm } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { RotateCw } from 'lucide-react'
import { reopenContainer, getAuthToken } from '../api'
import '@xterm/xterm/css/xterm.css'

interface TerminalProps {
  containerId: string
  connected: boolean
  readOnly?: boolean
  onReconnected: () => void
  onReadOnly?: () => void
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

export default function Terminal({ containerId, connected, readOnly, onReconnected, onReadOnly }: TerminalProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<XTerm | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const fitAddonRef = useRef<FitAddon | null>(null)
  const readOnlyRef = useRef(readOnly)
  readOnlyRef.current = readOnly
  const [disconnected, setDisconnected] = useState(!connected)
  const [reopening, setReopening] = useState(false)
  // Use a key to force full remount of the terminal when reopening
  const [mountKey, setMountKey] = useState(0)

  useEffect(() => {
    if (!containerRef.current) return

    const term = new XTerm({
      cursorBlink: true,
      fontSize: 13,
      fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', Menlo, Monaco, 'Courier New', monospace",
      theme: THEME,
    })

    const fitAddon = new FitAddon()
    term.loadAddon(fitAddon)
    term.open(containerRef.current)
    fitAddon.fit()
    termRef.current = term
    fitAddonRef.current = fitAddon

    // Connect WebSocket
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const tokenParam = getAuthToken() ? `&token=${getAuthToken()}` : ''
    const wsUrl = `${protocol}//${window.location.host}/ws/terminal?id=${containerId}${tokenParam}`
    const ws = new WebSocket(wsUrl)
    wsRef.current = ws
    ws.binaryType = 'arraybuffer'

    ws.onopen = () => {
      ws.send(JSON.stringify({ type: 'resize', rows: term.rows, cols: term.cols }))
    }

    ws.onmessage = (event) => {
      if (typeof event.data === 'string') {
        try {
          const msg = JSON.parse(event.data)
          if (msg.type === 'status' && msg.connected === false) {
            setDisconnected(true)
            return
          }
          if (msg.type === 'preempted') {
            term.write('\r\n\x1b[33m[Session preempted - read only]\x1b[0m\r\n')
            onReadOnly?.()
            return
          }
          if (msg.type === 'readonly') {
            term.write('\x1b[33m[Read-only mode]\x1b[0m\r\n')
            onReadOnly?.()
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

    // Keyboard input → WebSocket
    const onDataDisposable = term.onData((data) => {
      if (readOnlyRef.current) return
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(new TextEncoder().encode(data))
      }
    })

    // Resize handling
    const handleResize = () => {
      fitAddon.fit()
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'resize', rows: term.rows, cols: term.cols }))
      }
    }
    const resizeObserver = new ResizeObserver(handleResize)
    resizeObserver.observe(containerRef.current)

    return () => {
      resizeObserver.disconnect()
      onDataDisposable.dispose()
      ws.close()
      term.dispose()
      termRef.current = null
      wsRef.current = null
      fitAddonRef.current = null
    }
  }, [containerId, mountKey])

  const handleReopen = async () => {
    setReopening(true)
    try {
      await reopenContainer(containerId)
      setDisconnected(false)
      onReconnected()
      // Force full terminal remount
      setMountKey((k) => k + 1)
    } catch (e) {
      console.error('reopen failed:', e)
    } finally {
      setReopening(false)
    }
  }

  return (
    <div className="w-full h-full flex flex-col">
      <div ref={containerRef} className="flex-1 min-h-0" key={mountKey} />
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
}
