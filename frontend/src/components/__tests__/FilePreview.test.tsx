import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'

// Mock heavy dependencies
vi.mock('shiki', () => ({
  createHighlighter: vi.fn().mockResolvedValue({
    getLoadedLanguages: () => ['go', 'typescript'],
    loadLanguage: vi.fn(),
    codeToHtml: (_code: string) => '<pre><code>highlighted</code></pre>',
  }),
}))

vi.mock('react-markdown', () => ({
  default: ({ children }: { children: string }) => <div data-testid="markdown">{children}</div>,
}))

vi.mock('remark-gfm', () => ({
  default: () => {},
}))

vi.mock('../../api', async (importOriginal) => {
  const actual = await importOriginal() as Record<string, unknown>
  return {
    ...actual,
    getRawFileUrl: (_cid: string, _path: string) => '/mock-raw-url',
  }
})

import FilePreview from '../FilePreview'

const defaultProps = { containerId: 'c1', filePath: null as string | null }

describe('FilePreview', () => {
  it('shows empty state when no file selected', () => {
    render(<FilePreview content={null} fileName={null} {...defaultProps} />)
    expect(screen.getByText('Select a file to preview')).toBeDefined()
  })

  it('shows loading state', () => {
    render(<FilePreview content={null} fileName="test.go" {...defaultProps} loading={true} />)
    expect(screen.getByText('Loading file...')).toBeDefined()
  })

  it('shows binary file message', () => {
    render(<FilePreview content={{ type: 'binary' }} fileName="app.exe" {...defaultProps} />)
    expect(screen.getByText('Cannot preview this file type')).toBeDefined()
  })

  it('shows truncation banner', () => {
    render(
      <FilePreview
        content={{ type: 'text', content: 'some code', truncated: true, language: 'go' }}
        fileName="big.go"
        {...defaultProps}
      />
    )
    expect(screen.getByText(/File truncated/)).toBeDefined()
  })

  it('renders code content', () => {
    render(
      <FilePreview
        content={{ type: 'text', content: 'package main', language: 'go' }}
        fileName="main.go"
        {...defaultProps}
      />
    )
    // Before Shiki loads, shows raw content
    expect(screen.getByText('package main')).toBeDefined()
  })

  it('renders image preview', () => {
    const { container } = render(
      <FilePreview
        content={{ type: 'image', content: 'base64data', mimeType: 'image/png' }}
        fileName="photo.png"
        {...defaultProps}
      />
    )
    const img = container.querySelector('img')
    expect(img).not.toBeNull()
    expect(img?.src).toContain('data:image/png;base64,base64data')
  })
})
