import { useState, useCallback, useRef } from 'react'
import { getCWD, getFileContent } from '../api'
import type { FileContent } from '../api'

interface UseFileBrowserResult {
  rootPath: string
  selectedFile: string | null
  fileContent: FileContent | null
  loading: boolean
  error: string | null
  initBrowser: (containerId: string) => Promise<void>
  selectFile: (containerId: string, path: string) => Promise<void>
  reset: () => void
}

export function useFileBrowser(): UseFileBrowserResult {
  const [rootPath, setRootPath] = useState('')
  const [selectedFile, setSelectedFile] = useState<string | null>(null)
  const [fileContent, setFileContent] = useState<FileContent | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const cache = useRef<Map<string, FileContent>>(new Map())

  const initBrowser = useCallback(async (containerId: string) => {
    try {
      const cwd = await getCWD(containerId)
      setRootPath(cwd)
      setSelectedFile(null)
      setFileContent(null)
      setError(null)
      cache.current.clear()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to get CWD')
    }
  }, [])

  const selectFile = useCallback(async (containerId: string, path: string) => {
    setSelectedFile(path)
    setError(null)

    const cacheKey = `${containerId}:${path}`
    const cached = cache.current.get(cacheKey)
    if (cached) {
      setFileContent(cached)
      return
    }

    setLoading(true)
    try {
      const content = await getFileContent(containerId, path)
      cache.current.set(cacheKey, content)
      setFileContent(content)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load file')
      setFileContent(null)
    }
    setLoading(false)
  }, [])

  const reset = useCallback(() => {
    setRootPath('')
    setSelectedFile(null)
    setFileContent(null)
    setError(null)
    cache.current.clear()
  }, [])

  return {
    rootPath,
    selectedFile,
    fileContent,
    loading,
    error,
    initBrowser,
    selectFile,
    reset,
  }
}
