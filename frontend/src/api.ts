export interface Container {
  id: string
  name: string
  connected: boolean
  createdAt: string
}

export async function listContainers(): Promise<Container[]> {
  const res = await fetch('/api/containers')
  const data = await res.json()
  return data ?? []
}

export async function createContainer(name: string): Promise<Container> {
  const res = await fetch('/api/containers', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  })
  return res.json()
}

export async function deleteContainer(id: string): Promise<void> {
  await fetch(`/api/containers/${id}`, { method: 'DELETE' })
}

export async function renameContainer(id: string, name: string): Promise<void> {
  await fetch(`/api/containers/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  })
}

export async function reopenContainer(id: string): Promise<void> {
  await fetch(`/api/containers/${id}/reopen`, { method: 'POST' })
}

// --- Layout API ---

export interface LayoutEntry {
  containerId: string
  page: number
  position: number
}

export async function getLayout(): Promise<LayoutEntry[]> {
  const res = await fetch('/api/layout')
  return res.json()
}

export async function updateLayout(entries: LayoutEntry[]): Promise<void> {
  await fetch('/api/layout', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(entries),
  })
}

// --- Todo API ---

export interface Todo {
  id: number
  container: string
  content: string
  done: boolean
  sortOrder: number
  createdAt: string
}

export async function listTodos(containerID: string): Promise<Todo[]> {
  const res = await fetch(`/api/todos/${containerID}`)
  return res.json()
}

export async function createTodo(containerID: string, content: string): Promise<Todo> {
  const res = await fetch(`/api/todos/${containerID}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ content }),
  })
  return res.json()
}

export async function updateTodo(containerID: string, todoID: number, content: string, done: boolean): Promise<void> {
  await fetch(`/api/todos/${containerID}/${todoID}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ content, done }),
  })
}

export async function deleteTodo(containerID: string, todoID: number): Promise<void> {
  await fetch(`/api/todos/${containerID}/${todoID}`, { method: 'DELETE' })
}

export async function reorderTodos(containerID: string, ids: number[]): Promise<void> {
  await fetch(`/api/todos/${containerID}/reorder`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ids }),
  })
}
