// Auth token persisted in localStorage
const TOKEN_KEY = 'agent_hive_token'
let authToken = localStorage.getItem(TOKEN_KEY) ?? ''

export function setAuthToken(token: string) {
  authToken = token
  localStorage.setItem(TOKEN_KEY, token)
}

export function getAuthToken(): string {
  return authToken
}

function authHeaders(): Record<string, string> {
  const h: Record<string, string> = { 'Content-Type': 'application/json' }
  if (authToken) h['X-Auth-Token'] = authToken
  return h
}

function authQuery(): string {
  return authToken ? `token=${authToken}` : ''
}

// --- Auth API ---

export interface AuthCheck {
  enabled: boolean
  valid: boolean
}

export async function checkAuth(): Promise<AuthCheck> {
  const res = await fetch(`/api/auth/check?${authQuery()}`)
  return res.json()
}

// --- Container API ---

export interface Container {
  id: string
  name: string
  connected: boolean
  createdAt: string
}

export async function listContainers(): Promise<Container[]> {
  const res = await fetch('/api/containers', { headers: authHeaders() })
  const data = await res.json()
  return data ?? []
}

export async function createContainer(name: string): Promise<Container> {
  const res = await fetch('/api/containers', {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify({ name }),
  })
  return res.json()
}

export async function deleteContainer(id: string): Promise<void> {
  await fetch(`/api/containers/${id}`, { method: 'DELETE', headers: authHeaders() })
}

export async function renameContainer(id: string, name: string): Promise<void> {
  await fetch(`/api/containers/${id}`, {
    method: 'PATCH',
    headers: authHeaders(),
    body: JSON.stringify({ name }),
  })
}

export async function reopenContainer(id: string): Promise<void> {
  await fetch(`/api/containers/${id}/reopen`, { method: 'POST', headers: authHeaders() })
}

// --- Layout API ---

export interface LayoutEntry {
  containerId: string
  page: number
  position: number
}

export async function getLayout(): Promise<LayoutEntry[]> {
  const res = await fetch('/api/layout', { headers: authHeaders() })
  return res.json()
}

export async function updateLayout(entries: LayoutEntry[]): Promise<void> {
  await fetch('/api/layout', {
    method: 'PUT',
    headers: authHeaders(),
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
  const res = await fetch(`/api/todos/${containerID}`, { headers: authHeaders() })
  return res.json()
}

export async function createTodo(containerID: string, content: string): Promise<Todo> {
  const res = await fetch(`/api/todos/${containerID}`, {
    method: 'POST',
    headers: authHeaders(),
    body: JSON.stringify({ content }),
  })
  return res.json()
}

export async function updateTodo(containerID: string, todoID: number, content: string, done: boolean): Promise<void> {
  await fetch(`/api/todos/${containerID}/${todoID}`, {
    method: 'PATCH',
    headers: authHeaders(),
    body: JSON.stringify({ content, done }),
  })
}

export async function deleteTodo(containerID: string, todoID: number): Promise<void> {
  await fetch(`/api/todos/${containerID}/${todoID}`, { method: 'DELETE', headers: authHeaders() })
}

export async function reorderTodos(containerID: string, ids: number[]): Promise<void> {
  await fetch(`/api/todos/${containerID}/reorder`, {
    method: 'PUT',
    headers: authHeaders(),
    body: JSON.stringify({ ids }),
  })
}
