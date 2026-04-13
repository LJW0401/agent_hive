export interface Container {
  id: string
  name: string
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
