import { useEffect, useState, useCallback } from 'react'
import ProjectContainer from './components/ProjectContainer'
import NewProjectSlot from './components/NewProjectSlot'
import {
  listContainers,
  createContainer,
  deleteContainer,
  renameContainer,
  type Container,
} from './api'

const GRID_SIZE = 4 // 2×2

export default function App() {
  const [containers, setContainers] = useState<Container[]>([])

  useEffect(() => {
    listContainers().then(setContainers)
  }, [])

  const handleCreate = useCallback(async () => {
    const c = await createContainer('New Project')
    setContainers((prev) => [...prev, c])
  }, [])

  const handleClose = useCallback(async (id: string) => {
    await deleteContainer(id)
    setContainers((prev) => prev.filter((c) => c.id !== id))
  }, [])

  const handleRename = useCallback(async (id: string, name: string) => {
    await renameContainer(id, name)
    setContainers((prev) =>
      prev.map((c) => (c.id === id ? { ...c, name } : c))
    )
  }, [])

  // Fill remaining slots with "new project" buttons
  const emptySlots = Math.max(0, GRID_SIZE - containers.length)

  return (
    <div className="flex flex-col h-screen bg-[#0a0a0b]">
      <header className="flex items-center px-4 h-11 border-b border-gray-800 shrink-0">
        <h1 className="text-sm font-semibold text-gray-200 tracking-wide">
          Agent Hive
        </h1>
        <span className="ml-3 text-[10px] text-gray-600">
          {containers.length} project{containers.length !== 1 ? 's' : ''}
        </span>
      </header>

      <main className="flex-1 min-h-0 p-2">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-2 h-full"
             style={{ gridTemplateRows: 'repeat(2, minmax(0, 1fr))' }}>
          {containers.slice(0, GRID_SIZE).map((c) => (
            <ProjectContainer
              key={c.id}
              container={c}
              onClose={handleClose}
              onRename={handleRename}
            />
          ))}
          {emptySlots > 0 &&
            Array.from({ length: emptySlots }).map((_, i) => (
              <NewProjectSlot key={`empty-${i}`} onClick={handleCreate} />
            ))}
        </div>
      </main>
    </div>
  )
}
