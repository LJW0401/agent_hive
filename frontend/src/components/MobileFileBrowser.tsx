import { useState, useCallback } from 'react'
import { ArrowLeft } from 'lucide-react'
import FileTree from './FileTree'
import FilePreview from './FilePreview'
import { useFileBrowser } from '../hooks/useFileBrowser'

interface MobileFileBrowserProps {
  containerId: string
}

export default function MobileFileBrowser({ containerId }: MobileFileBrowserProps) {
  const [view, setView] = useState<'tree' | 'preview'>('tree')
  const fileBrowser = useFileBrowser()

  const handleSelect = useCallback(async (path: string) => {
    await fileBrowser.selectFile(containerId, path)
    setView('preview')
  }, [containerId, fileBrowser])

  const handleBack = useCallback(() => {
    setView('tree')
  }, [])

  const fileName = fileBrowser.selectedFile?.split('/').pop() || null

  return (
    <div className="h-full relative">
      {/* File tree — always mounted, hidden when previewing */}
      <div className={`h-full ${view === 'preview' ? 'hidden' : ''}`}>
        <FileTree
          containerId={containerId}
          rootPath="."
          selectedPath={fileBrowser.selectedFile}
          onSelect={handleSelect}
        />
      </div>

      {/* File preview */}
      {view === 'preview' && (
        <div className="flex flex-col h-full absolute inset-0">
          <div className="flex items-center gap-2 h-9 px-3 shrink-0 border-b border-gray-800 bg-[#0c0c0e]">
            <button
              onClick={handleBack}
              className="text-gray-400 hover:text-gray-200 p-0.5"
            >
              <ArrowLeft size={16} />
            </button>
            <span className="text-[12px] text-gray-300 truncate">{fileName}</span>
          </div>
          <div className="flex-1 min-h-0 overflow-auto">
            <FilePreview
              content={fileBrowser.fileContent}
              fileName={fileName}
              filePath={fileBrowser.selectedFile}
              containerId={containerId}
              loading={fileBrowser.loading}
            />
          </div>
        </div>
      )}
    </div>
  )
}
