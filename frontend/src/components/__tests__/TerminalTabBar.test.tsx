import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import TerminalTabBar from '../TerminalTabBar'

const defaultTerminal = { id: 't-1', name: 'Terminal 1', isDefault: true, connected: true }
const extraTerminal = { id: 't-2', name: 'Terminal 2', isDefault: false, connected: true }

describe('TerminalTabBar', () => {
  it('renders correct number of tabs', () => {
    render(
      <TerminalTabBar
        terminals={[defaultTerminal, extraTerminal]}
        activeId="t-1"
        onSelect={vi.fn()}
        onCreate={vi.fn()}
        onClose={vi.fn()}
      />
    )
    expect(screen.getByText('Terminal 1')).toBeDefined()
    expect(screen.getByText('Terminal 2')).toBeDefined()
  })

  it('calls onSelect when tab clicked', () => {
    const onSelect = vi.fn()
    render(
      <TerminalTabBar
        terminals={[defaultTerminal, extraTerminal]}
        activeId="t-1"
        onSelect={onSelect}
        onCreate={vi.fn()}
        onClose={vi.fn()}
      />
    )
    fireEvent.click(screen.getByText('Terminal 2'))
    expect(onSelect).toHaveBeenCalledWith('t-2')
  })

  it('does not show close button on default terminal', () => {
    const { container } = render(
      <TerminalTabBar
        terminals={[defaultTerminal]}
        activeId="t-1"
        onSelect={vi.fn()}
        onCreate={vi.fn()}
        onClose={vi.fn()}
      />
    )
    const closeButtons = container.querySelectorAll('[role="button"]')
    expect(closeButtons.length).toBe(0)
  })

  it('shows close button on extra terminal', () => {
    const { container } = render(
      <TerminalTabBar
        terminals={[defaultTerminal, extraTerminal]}
        activeId="t-1"
        onSelect={vi.fn()}
        onCreate={vi.fn()}
        onClose={vi.fn()}
      />
    )
    const closeButtons = container.querySelectorAll('[role="button"]')
    expect(closeButtons.length).toBe(1)
  })

  it('always shows + button', () => {
    const { container } = render(
      <TerminalTabBar
        terminals={[defaultTerminal]}
        activeId="t-1"
        onSelect={vi.fn()}
        onCreate={vi.fn()}
        onClose={vi.fn()}
      />
    )
    expect(container.querySelector('[title="New terminal"]')).not.toBeNull()
  })
})
