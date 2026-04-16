import { useState, useEffect, useRef, useCallback } from 'react'
import { listTerminals, createTerminal, deleteTerminal, hasProcess } from '../api'
import type { TerminalInfo } from '../api'
import type { TerminalHandle } from '../components/Terminal'

interface UseTerminalTabsResult {
  terminals: TerminalInfo[]
  activeTerminalId: string
  setActiveTerminalId: (id: string) => void
  confirmClose: string | null
  terminalRefs: React.MutableRefObject<Map<string, TerminalHandle>>
  handleCreateTerminal: () => Promise<void>
  handleCloseTerminal: (tid: string) => Promise<void>
  doCloseTerminal: (tid: string) => Promise<void>
  cancelClose: () => void
  sendToActive: (data: string) => void
}

export function useTerminalTabs(containerID: string, terminalRefreshKey?: number): UseTerminalTabsResult {
  const [terminals, setTerminals] = useState<TerminalInfo[]>([])
  const [activeTerminalId, setActiveTerminalId] = useState<string>('')
  const [confirmClose, setConfirmClose] = useState<string | null>(null)
  const terminalRefs = useRef<Map<string, TerminalHandle>>(new Map())

  useEffect(() => {
    listTerminals(containerID).then((terms) => {
      setTerminals(terms)
      if (terms.length > 0 && !activeTerminalId) {
        setActiveTerminalId(terms.find(t => t.isDefault)?.id ?? terms[0].id)
      }
    })
  }, [containerID, terminalRefreshKey])

  const handleCreateTerminal = useCallback(async () => {
    try {
      const term = await createTerminal(containerID)
      setTerminals(prev => [...prev, term])
      setActiveTerminalId(term.id)
    } catch (e) {
      console.error('create terminal failed:', e)
    }
  }, [containerID])

  const handleCloseTerminal = useCallback(async (tid: string) => {
    try {
      const hasProc = await hasProcess(containerID, tid)
      if (hasProc) {
        setConfirmClose(tid)
        return
      }
      await doClose(tid)
    } catch {
      setConfirmClose(tid)
    }
  }, [containerID])

  const doClose = useCallback(async (tid: string) => {
    try {
      await deleteTerminal(containerID, tid)
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
  }, [containerID, activeTerminalId])

  const cancelClose = useCallback(() => setConfirmClose(null), [])

  const sendToActive = useCallback((data: string) => {
    const handle = terminalRefs.current.get(activeTerminalId)
    handle?.sendData(data)
  }, [activeTerminalId])

  return {
    terminals,
    activeTerminalId,
    setActiveTerminalId,
    confirmClose,
    terminalRefs,
    handleCreateTerminal,
    handleCloseTerminal,
    doCloseTerminal: doClose,
    cancelClose,
    sendToActive,
  }
}
