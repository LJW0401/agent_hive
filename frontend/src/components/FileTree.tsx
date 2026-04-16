import { useState, useCallback } from 'react'
import { ChevronRight, ChevronDown, Folder, FolderOpen, File, FileText, FileCode, Image, FileType2, Search } from 'lucide-react'
import { listFiles } from '../api'
import type { FileEntry } from '../api'

interface FileTreeProps {
  containerId: string
  rootPath: string
  selectedPath: string | null
  onSelect: (path: string) => void
}

interface TreeNode extends FileEntry {
  path: string
}

function getFileIcon(name: string, type: string) {
  if (type === 'dir') return null // handled by Folder/FolderOpen
  const ext = name.split('.').pop()?.toLowerCase()
  switch (ext) {
    case 'png': case 'jpg': case 'jpeg': case 'gif': case 'svg': case 'webp':
      return <Image size={14} className="text-purple-400 shrink-0" />
    case 'md': case 'markdown':
      return <FileText size={14} className="text-blue-400 shrink-0" />
    case 'go': case 'ts': case 'tsx': case 'js': case 'jsx': case 'py': case 'rs': case 'java':
      return <FileCode size={14} className="text-green-400 shrink-0" />
    case 'pdf':
      return <FileType2 size={14} className="text-red-400 shrink-0" />
    default:
      return <File size={14} className="text-gray-500 shrink-0" />
  }
}

function TreeItem({
  node,
  depth,
  containerId,
  selectedPath,
  onSelect,
  searchQuery,
}: {
  node: TreeNode
  depth: number
  containerId: string
  selectedPath: string | null
  onSelect: (path: string) => void
  searchQuery: string
}) {
  const [expanded, setExpanded] = useState(false)
  const [children, setChildren] = useState<TreeNode[]>([])
  const [loading, setLoading] = useState(false)
  const [loaded, setLoaded] = useState(false)

  const toggleExpand = useCallback(async () => {
    if (node.type !== 'dir') return

    if (expanded) {
      setExpanded(false)
      return
    }

    if (!loaded) {
      setLoading(true)
      try {
        const entries = await listFiles(containerId, node.path)
        setChildren(entries.map(e => ({
          ...e,
          path: node.path === '.' ? e.name : `${node.path}/${e.name}`,
        })))
        setLoaded(true)
      } catch (e) {
        console.error('Failed to load directory:', e)
      }
      setLoading(false)
    }
    setExpanded(true)
  }, [expanded, loaded, node, containerId])

  const handleClick = () => {
    if (node.type === 'dir') {
      toggleExpand()
    } else {
      onSelect(node.path)
    }
  }

  const isSelected = node.path === selectedPath
  const filteredChildren = searchQuery
    ? children.filter(c => matchesSearch(c, searchQuery, children))
    : children

  const highlightName = (name: string) => {
    if (!searchQuery) return name
    const idx = name.toLowerCase().indexOf(searchQuery.toLowerCase())
    if (idx === -1) return name
    return (
      <>
        {name.slice(0, idx)}
        <span className="bg-yellow-500/30 text-yellow-200">{name.slice(idx, idx + searchQuery.length)}</span>
        {name.slice(idx + searchQuery.length)}
      </>
    )
  }

  return (
    <div>
      <div
        className={`flex items-center gap-1 py-[2px] px-1 cursor-pointer select-none text-[12px] rounded transition-colors ${
          isSelected
            ? 'bg-blue-600/30 text-blue-200'
            : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800/50'
        }`}
        style={{ paddingLeft: `${depth * 16 + 4}px` }}
        onClick={handleClick}
      >
        {node.type === 'dir' ? (
          <>
            {loading ? (
              <div className="w-3 h-3 border border-gray-500 border-t-transparent rounded-full animate-spin shrink-0" />
            ) : expanded ? (
              <ChevronDown size={12} className="text-gray-500 shrink-0" />
            ) : (
              <ChevronRight size={12} className="text-gray-500 shrink-0" />
            )}
            {expanded ? (
              <FolderOpen size={14} className="text-yellow-500 shrink-0" />
            ) : (
              <Folder size={14} className="text-yellow-600 shrink-0" />
            )}
          </>
        ) : (
          <>
            <span className="w-3 shrink-0" />
            {getFileIcon(node.name, node.type)}
          </>
        )}
        <span className="truncate">{highlightName(node.name)}</span>
      </div>

      {expanded && filteredChildren.length > 0 && (
        <div className="relative">
          <div
            className="absolute top-0 bottom-0 w-px bg-gray-800"
            style={{ left: `${depth * 16 + 11}px` }}
          />
          {filteredChildren.map(child => (
            <TreeItem
              key={child.path}
              node={child}
              depth={depth + 1}
              containerId={containerId}
              selectedPath={selectedPath}
              onSelect={onSelect}
              searchQuery={searchQuery}
            />
          ))}
        </div>
      )}
    </div>
  )
}

function matchesSearch(node: TreeNode, query: string, _siblings: TreeNode[]): boolean {
  if (node.name.toLowerCase().includes(query.toLowerCase())) return true
  if (node.type === 'dir') return true // keep dirs to preserve hierarchy
  return false
}

export default function FileTree({ containerId, rootPath, selectedPath, onSelect }: FileTreeProps) {
  const [rootChildren, setRootChildren] = useState<TreeNode[]>([])
  const [loaded, setLoaded] = useState(false)
  const [loading, setLoading] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')

  const loadRoot = useCallback(async () => {
    if (loaded) return
    setLoading(true)
    try {
      const entries = await listFiles(containerId, rootPath)
      setRootChildren(entries.map(e => ({
        ...e,
        path: e.name,
      })))
      setLoaded(true)
    } catch (e) {
      console.error('Failed to load root:', e)
    }
    setLoading(false)
  }, [containerId, rootPath, loaded])

  // Auto-load on first render
  if (!loaded && !loading) {
    loadRoot()
  }

  const filteredChildren = searchQuery
    ? rootChildren.filter(c => matchesSearch(c, searchQuery, rootChildren))
    : rootChildren

  return (
    <div className="h-full flex flex-col bg-[#0f0f12] overflow-hidden">
      <div className="flex items-center gap-1 px-2 py-1.5 border-b border-gray-800">
        <Search size={12} className="text-gray-500 shrink-0" />
        <input
          type="text"
          value={searchQuery}
          onChange={e => setSearchQuery(e.target.value)}
          placeholder="Filter files..."
          className="flex-1 bg-transparent text-[11px] text-gray-300 placeholder-gray-600 outline-none"
        />
        {searchQuery && (
          <button onClick={() => setSearchQuery('')} className="text-gray-500 hover:text-gray-300 text-[10px]">
            clear
          </button>
        )}
      </div>

      <div className="flex-1 overflow-y-auto py-1">
        {loading && !loaded && (
          <div className="text-center text-[11px] text-gray-500 py-4">Loading...</div>
        )}
        {filteredChildren.map(node => (
          <TreeItem
            key={node.path}
            node={node}
            depth={0}
            containerId={containerId}
            selectedPath={selectedPath}
            onSelect={onSelect}
            searchQuery={searchQuery}
          />
        ))}
      </div>
    </div>
  )
}
