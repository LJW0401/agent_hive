import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import ShortcutBar from '../ShortcutBar'

describe('ShortcutBar', () => {
  it('renders all shortcut buttons', () => {
    const onSend = vi.fn()
    render(<ShortcutBar onSend={onSend} />)

    expect(screen.getByText('^C')).toBeTruthy()
    expect(screen.getByText('^L')).toBeTruthy()
    expect(screen.getByText('Tab')).toBeTruthy()
    expect(screen.getByText('Esc')).toBeTruthy()
    expect(screen.getByText('↑')).toBeTruthy()
    expect(screen.getByText('↓')).toBeTruthy()
    expect(screen.getByText('←')).toBeTruthy()
    expect(screen.getByText('→')).toBeTruthy()
    expect(screen.getByText('Paste')).toBeTruthy()
  })

  it('sends correct control character for ^C', () => {
    const onSend = vi.fn()
    render(<ShortcutBar onSend={onSend} />)
    fireEvent.click(screen.getByText('^C'))
    expect(onSend).toHaveBeenCalledWith('\x03')
  })

  it('sends correct control character for ^L', () => {
    const onSend = vi.fn()
    render(<ShortcutBar onSend={onSend} />)
    fireEvent.click(screen.getByText('^L'))
    expect(onSend).toHaveBeenCalledWith('\x0c')
  })

  it('sends correct character for Tab', () => {
    const onSend = vi.fn()
    render(<ShortcutBar onSend={onSend} />)
    fireEvent.click(screen.getByText('Tab'))
    expect(onSend).toHaveBeenCalledWith('\t')
  })

  it('sends correct character for Esc', () => {
    const onSend = vi.fn()
    render(<ShortcutBar onSend={onSend} />)
    fireEvent.click(screen.getByText('Esc'))
    expect(onSend).toHaveBeenCalledWith('\x1b')
  })

  it('sends correct escape sequence for arrow keys', () => {
    const onSend = vi.fn()
    render(<ShortcutBar onSend={onSend} />)

    fireEvent.click(screen.getByText('↑'))
    expect(onSend).toHaveBeenCalledWith('\x1b[A')

    fireEvent.click(screen.getByText('↓'))
    expect(onSend).toHaveBeenCalledWith('\x1b[B')

    fireEvent.click(screen.getByText('←'))
    expect(onSend).toHaveBeenCalledWith('\x1b[D')

    fireEvent.click(screen.getByText('→'))
    expect(onSend).toHaveBeenCalledWith('\x1b[C')
  })
})
