import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import PasteModal from '../PasteModal'

describe('PasteModal', () => {
  it('renders modal with textarea and buttons', () => {
    const onConfirm = vi.fn()
    const onClose = vi.fn()
    render(<PasteModal onConfirm={onConfirm} onClose={onClose} />)

    expect(screen.getByPlaceholderText('Paste text here...')).toBeTruthy()
    expect(screen.getByText('Send')).toBeTruthy()
    expect(screen.getByText('Cancel')).toBeTruthy()
  })

  it('calls onConfirm with text when Send clicked', () => {
    const onConfirm = vi.fn()
    const onClose = vi.fn()
    render(<PasteModal onConfirm={onConfirm} onClose={onClose} />)

    const textarea = screen.getByPlaceholderText('Paste text here...')
    fireEvent.change(textarea, { target: { value: 'echo hello' } })
    fireEvent.click(screen.getByText('Send'))

    expect(onConfirm).toHaveBeenCalledWith('echo hello')
  })

  it('calls onClose when Cancel clicked', () => {
    const onConfirm = vi.fn()
    const onClose = vi.fn()
    render(<PasteModal onConfirm={onConfirm} onClose={onClose} />)

    fireEvent.click(screen.getByText('Cancel'))
    expect(onClose).toHaveBeenCalled()
    expect(onConfirm).not.toHaveBeenCalled()
  })

  it('calls onClose when empty text and Send clicked', () => {
    const onConfirm = vi.fn()
    const onClose = vi.fn()
    render(<PasteModal onConfirm={onConfirm} onClose={onClose} />)

    fireEvent.click(screen.getByText('Send'))
    expect(onClose).toHaveBeenCalled()
    expect(onConfirm).not.toHaveBeenCalled()
  })

  it('calls onClose when clicking backdrop', () => {
    const onConfirm = vi.fn()
    const onClose = vi.fn()
    render(<PasteModal onConfirm={onConfirm} onClose={onClose} />)

    // Click the backdrop (outermost div)
    const backdrop = screen.getByText('Paste content').parentElement!.parentElement!
    fireEvent.click(backdrop)
    expect(onClose).toHaveBeenCalled()
  })
})
