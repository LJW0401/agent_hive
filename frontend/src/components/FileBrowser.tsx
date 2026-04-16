import { useState, useRef, useCallback } from 'react'
import FileTree from './FileTree'
import FilePreview from './FilePreview'
import type { FileContent } from '../api'

interface FileBrowserProps {
  containerId: string
  selectedFile: string | null
  fileContent: FileContent | null
  loading: boolean
  onSelectFile: (path: string) => void
}

const MIN_TREE_RATIO = 0.15
const MAX_TREE_RATIO = 0.6
const DEFAULT_TREE_RATIO = 0.3

export default function FileBrowser({
  containerId,
  selectedFile,
  fileContent,
  loading,
  onSelectFile,
}: FileBrowserProps) {
  const [treeRatio, setTreeRatio] = useState(DEFAULT_TREE_RATIO)
  const dragging = useRef(false)
  const containerRef = useRef<HTMLDivElement>(null)

  const onMouseDown = useCallback(() => {
    dragging.current = true
    const onMouseMove = (e: MouseEvent) => {
      if (!dragging.current || !containerRef.current) return
      const rect = containerRef.current.getBoundingClientRect()
      const ratio = (e.clientX - rect.left) / rect.width
      setTreeRatio(Math.max(MIN_TREE_RATIO, Math.min(MAX_TREE_RATIO, ratio)))
    }
    const onMouseUp = () => {
      dragging.current = false
      window.removeEventListener('mousemove', onMouseMove)
      window.removeEventListener('mouseup', onMouseUp)
    }
    window.addEventListener('mousemove', onMouseMove)
    window.addEventListener('mouseup', onMouseUp)
  }, [])

  const fileName = selectedFile ? selectedFile.split('/').pop() || null : null

  return (
    <div ref={containerRef} className="flex h-full w-full">
      <div style={{ width: `${treeRatio * 100}%` }} className="shrink-0 overflow-hidden border-r border-gray-800">
        <FileTree
          containerId={containerId}
          rootPath="."
          selectedPath={selectedFile}
          onSelect={onSelectFile}
        />
      </div>

      <div
        className="w-1 shrink-0 cursor-col-resize bg-gray-800 hover:bg-blue-600/50 transition-colors"
        onMouseDown={onMouseDown}
      />

      <div className="flex-1 min-w-0 overflow-hidden">
        <FilePreview content={fileContent} fileName={fileName} filePath={selectedFile} containerId={containerId} loading={loading} />
      </div>
    </div>
  )
}
