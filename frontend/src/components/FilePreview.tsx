import { useState, useEffect, useRef } from 'react'
import { FileQuestion, AlertTriangle } from 'lucide-react'
import type { FileContent } from '../api'

// Lazy-loaded Shiki highlighter (singleton)
let highlighterPromise: Promise<import('shiki').Highlighter> | null = null
function getHighlighter() {
  if (!highlighterPromise) {
    highlighterPromise = import('shiki').then(({ createHighlighter }) =>
      createHighlighter({
        themes: ['github-dark'],
        langs: [
          'go', 'javascript', 'typescript', 'tsx', 'jsx', 'python', 'rust',
          'java', 'c', 'cpp', 'bash', 'sql', 'html', 'css', 'json', 'yaml',
          'toml', 'markdown', 'dockerfile', 'makefile', 'xml',
        ],
      })
    )
  }
  return highlighterPromise
}

interface FilePreviewProps {
  content: FileContent | null
  fileName: string | null
  loading?: boolean
}

function CodePreview({ content, language }: { content: string; language?: string }) {
  const [html, setHtml] = useState<string>('')
  const [shikiReady, setShikiReady] = useState(false)

  useEffect(() => {
    let cancelled = false
    getHighlighter().then(async (highlighter) => {
      if (cancelled) return
      const loadedLangs = highlighter.getLoadedLanguages()
      let lang = language || 'text'
      if (!loadedLangs.includes(lang as never)) {
        try {
          await highlighter.loadLanguage(lang as never)
        } catch {
          lang = 'text'
        }
      }
      if (cancelled) return
      const result = highlighter.codeToHtml(content, {
        lang,
        theme: 'github-dark',
      })
      setHtml(result)
      setShikiReady(true)
    })
    return () => { cancelled = true }
  }, [content, language])

  if (!shikiReady) {
    return (
      <pre className="p-4 text-[12px] text-gray-400 font-mono whitespace-pre-wrap break-all">
        {content}
      </pre>
    )
  }

  return (
    <div
      className="p-4 text-[12px] overflow-auto [&_pre]:!bg-transparent [&_code]:!bg-transparent"
      dangerouslySetInnerHTML={{ __html: html }}
    />
  )
}

function MarkdownPreview({ content }: { content: string }) {
  const [ReactMarkdown, setReactMarkdown] = useState<typeof import('react-markdown').default | null>(null)
  const [remarkGfm, setRemarkGfm] = useState<typeof import('remark-gfm').default | null>(null)

  useEffect(() => {
    let cancelled = false
    Promise.all([
      import('react-markdown'),
      import('remark-gfm'),
    ]).then(([md, gfm]) => {
      if (cancelled) return
      setReactMarkdown(() => md.default)
      setRemarkGfm(() => gfm.default)
    })
    return () => { cancelled = true }
  }, [])

  if (!ReactMarkdown || !remarkGfm) {
    return <div className="p-4 text-[12px] text-gray-400">Loading markdown renderer...</div>
  }

  return (
    <div className="p-6 prose prose-invert prose-sm max-w-none
      prose-headings:text-gray-200 prose-p:text-gray-300 prose-a:text-blue-400
      prose-strong:text-gray-200 prose-code:text-pink-400 prose-code:bg-gray-800 prose-code:px-1 prose-code:rounded
      prose-pre:bg-gray-900 prose-pre:border prose-pre:border-gray-800
      prose-table:border-collapse
      prose-th:border prose-th:border-gray-700 prose-th:px-3 prose-th:py-1.5 prose-th:bg-gray-800
      prose-td:border prose-td:border-gray-700 prose-td:px-3 prose-td:py-1.5
      prose-img:rounded-lg prose-img:max-w-full
      prose-li:text-gray-300
      [&_input[type=checkbox]]:mr-2 [&_input[type=checkbox]]:accent-blue-500
    ">
      <ReactMarkdown remarkPlugins={[remarkGfm]}>{content}</ReactMarkdown>
    </div>
  )
}

function ImagePreview({ content, mimeType }: { content: string; mimeType?: string }) {
  return (
    <div className="flex items-center justify-center p-8 h-full">
      <img
        src={`data:${mimeType || 'image/png'};base64,${content}`}
        alt="Preview"
        className="max-w-full max-h-full object-contain rounded-lg"
      />
    </div>
  )
}

function base64ToBlob(base64: string, mimeType: string): string {
  const bytes = atob(base64)
  const arr = new Uint8Array(bytes.length)
  for (let i = 0; i < bytes.length; i++) {
    arr[i] = bytes.charCodeAt(i)
  }
  const blob = new Blob([arr], { type: mimeType })
  return URL.createObjectURL(blob)
}

function PdfPreview({ content }: { content: string }) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [PdfComponents, setPdfComponents] = useState<{
    Document: typeof import('react-pdf').Document
    Page: typeof import('react-pdf').Page
  } | null>(null)
  const [numPages, setNumPages] = useState(0)
  const [containerWidth, setContainerWidth] = useState(600)
  const [blobUrl, setBlobUrl] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    import('react-pdf').then(async (mod) => {
      if (cancelled) return
      // Import worker via Vite ?url
      const workerModule = await import('pdfjs-dist/build/pdf.worker.min.mjs?url')
      mod.pdfjs.GlobalWorkerOptions.workerSrc = workerModule.default
      setPdfComponents({ Document: mod.Document, Page: mod.Page })
    })
    return () => { cancelled = true }
  }, [])

  // Convert base64 to blob URL for better compatibility
  useEffect(() => {
    const url = base64ToBlob(content, 'application/pdf')
    setBlobUrl(url)
    return () => URL.revokeObjectURL(url)
  }, [content])

  useEffect(() => {
    if (!containerRef.current) return
    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        setContainerWidth(entry.contentRect.width - 32)
      }
    })
    observer.observe(containerRef.current)
    return () => observer.disconnect()
  }, [])

  if (!PdfComponents || !blobUrl) {
    return <div className="p-4 text-[12px] text-gray-400">Loading PDF viewer...</div>
  }

  const { Document, Page } = PdfComponents

  return (
    <div className="flex flex-col h-full">
      {numPages > 0 && (
        <div className="flex items-center justify-center py-1 border-b border-gray-800 bg-[#0c0c0e]">
          <span className="text-[10px] text-gray-500">{numPages} pages</span>
        </div>
      )}
      <div ref={containerRef} className="flex-1 overflow-auto">
        <div className="flex flex-col items-center gap-2 p-4">
          <Document
            file={blobUrl}
            onLoadSuccess={({ numPages: n }) => setNumPages(n)}
            loading={<div className="text-[12px] text-gray-400">Loading PDF...</div>}
            error={<div className="text-[12px] text-red-400">Failed to load PDF</div>}
          >
            {Array.from({ length: numPages }, (_, i) => (
              <Page
                key={i + 1}
                pageNumber={i + 1}
                width={Math.min(containerWidth, 900)}
                renderTextLayer={true}
                renderAnnotationLayer={true}
                className="shadow-lg mb-2"
              />
            ))}
          </Document>
        </div>
      </div>
    </div>
  )
}

export default function FilePreview({ content, fileName, loading }: FilePreviewProps) {
  if (loading) {
    return (
      <div className="flex items-center justify-center h-full text-[12px] text-gray-500">
        Loading file...
      </div>
    )
  }

  if (!content || !fileName) {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-2 text-gray-600">
        <FileQuestion size={32} />
        <span className="text-[12px]">Select a file to preview</span>
      </div>
    )
  }

  return (
    <div className="h-full overflow-auto bg-[#111114]">
      {content.truncated && (
        <div className="flex items-center gap-1.5 px-3 py-1.5 bg-yellow-900/30 border-b border-yellow-800/50 text-[11px] text-yellow-400">
          <AlertTriangle size={12} />
          File truncated — showing last portion
        </div>
      )}

      {content.type === 'text' && (
        <CodePreview content={content.content || ''} language={content.language} />
      )}
      {content.type === 'markdown' && (
        <MarkdownPreview content={content.content || ''} />
      )}
      {content.type === 'image' && (
        <ImagePreview content={content.content || ''} mimeType={content.mimeType} />
      )}
      {content.type === 'pdf' && (
        <PdfPreview content={content.content || ''} />
      )}
      {content.type === 'binary' && (
        <div className="flex flex-col items-center justify-center h-full gap-2 text-gray-500">
          <FileQuestion size={32} />
          <span className="text-[12px]">Cannot preview this file type</span>
        </div>
      )}
    </div>
  )
}
