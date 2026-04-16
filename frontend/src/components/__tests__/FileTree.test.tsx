import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('../../api', () => ({
  listFiles: vi.fn(),
}))

import FileTree from '../FileTree'
import { listFiles } from '../../api'

const mockListFiles = vi.mocked(listFiles)

beforeEach(() => {
  vi.clearAllMocks()
})

describe('FileTree', () => {
  it('renders root directory entries', async () => {
    mockListFiles.mockResolvedValueOnce([
      { name: 'src', type: 'dir', size: 0 },
      { name: 'README.md', type: 'file', size: 100 },
      { name: 'main.go', type: 'file', size: 200 },
    ])

    render(
      <FileTree
        containerId="c1"
        rootPath="."
        selectedPath={null}
        onSelect={vi.fn()}
      />
    )

    await waitFor(() => {
      expect(screen.getByText('src')).toBeDefined()
      expect(screen.getByText('README.md')).toBeDefined()
      expect(screen.getByText('main.go')).toBeDefined()
    })
  })

  it('calls onSelect when file clicked', async () => {
    mockListFiles.mockResolvedValueOnce([
      { name: 'main.go', type: 'file', size: 200 },
    ])

    const onSelect = vi.fn()
    render(
      <FileTree
        containerId="c1"
        rootPath="."
        selectedPath={null}
        onSelect={onSelect}
      />
    )

    await waitFor(() => {
      expect(screen.getByText('main.go')).toBeDefined()
    })

    fireEvent.click(screen.getByText('main.go'))
    expect(onSelect).toHaveBeenCalledWith('main.go')
  })

  it('expands directory on click', async () => {
    mockListFiles
      .mockResolvedValueOnce([
        { name: 'src', type: 'dir', size: 0 },
      ])
      .mockResolvedValueOnce([
        { name: 'index.ts', type: 'file', size: 50 },
      ])

    render(
      <FileTree
        containerId="c1"
        rootPath="."
        selectedPath={null}
        onSelect={vi.fn()}
      />
    )

    await waitFor(() => {
      expect(screen.getByText('src')).toBeDefined()
    })

    fireEvent.click(screen.getByText('src'))

    await waitFor(() => {
      expect(screen.getByText('index.ts')).toBeDefined()
    })
    expect(mockListFiles).toHaveBeenCalledTimes(2)
  })

  it('filters files by search query', async () => {
    mockListFiles.mockResolvedValueOnce([
      { name: 'src', type: 'dir', size: 0 },
      { name: 'README.md', type: 'file', size: 100 },
      { name: 'main.go', type: 'file', size: 200 },
    ])

    render(
      <FileTree
        containerId="c1"
        rootPath="."
        selectedPath={null}
        onSelect={vi.fn()}
      />
    )

    await waitFor(() => {
      expect(screen.getByText('main.go')).toBeDefined()
    })

    const searchInput = screen.getByPlaceholderText('Filter files...')
    fireEvent.change(searchInput, { target: { value: 'main' } })

    // main.go should be visible (text split by highlight spans), README.md should be filtered out
    expect(screen.getAllByText((_, el) => el?.textContent === 'main.go' && el?.tagName === 'SPAN').length).toBeGreaterThan(0)
    expect(screen.queryByText('README.md')).toBeNull()
  })
})
