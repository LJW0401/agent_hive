import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'

// Mock the api module
vi.mock('../../api', () => ({
  listTodos: vi.fn().mockResolvedValue([
    { id: 1, content: 'Short todo', done: false, position: 0 },
    { id: 2, content: 'A very long todo item that should wrap to multiple lines instead of being truncated with an ellipsis', done: false, position: 1 },
    { id: 3, content: 'Done item', done: true, position: 2 },
  ]),
  createTodo: vi.fn(),
  updateTodo: vi.fn(),
  deleteTodo: vi.fn(),
  reorderTodos: vi.fn(),
}))

import TodoList from '../TodoList'

describe('TodoList text wrapping', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('should not use truncate class on todo items', async () => {
    render(<TodoList containerID="test-1" />)

    const longText = await screen.findByText(/A very long todo item/)
    expect(longText.className).not.toContain('truncate')
    expect(longText.className).toContain('break-all')
  })

  it('should use items-start alignment for multi-line support', async () => {
    render(<TodoList containerID="test-1" />)

    await screen.findByText('Short todo')
    // The parent container of todo items should use items-start
    const todoItems = document.querySelectorAll('.flex.items-start')
    expect(todoItems.length).toBeGreaterThan(0)
  })

  // Smoke: delete button is visible by default on mobile viewports (no hover).
  it('delete button is visible by default and reverts to hover on md+ breakpoint', async () => {
    render(<TodoList containerID="test-1" />)

    await screen.findByText('Short todo')
    // Each todo row renders a delete (Trash2) button; className should make it
    // visible on mobile (opacity-100) while retaining the desktop hover-only
    // behaviour (md:opacity-0 + md:group-hover:opacity-100).
    const deleteButtons = Array.from(
      document.querySelectorAll('button.text-gray-700.hover\\:text-red-400')
    )
    expect(deleteButtons.length).toBeGreaterThan(0)

    for (const btn of deleteButtons) {
      const cls = btn.className
      // non-empty: edge — mobile visibility
      expect(cls).toContain('opacity-100')
      // non-empty: edge — desktop fallback preserved
      expect(cls).toContain('md:opacity-0')
      expect(cls).toContain('md:group-hover:opacity-100')
    }
  })
})
