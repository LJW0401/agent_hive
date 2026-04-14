import { useState } from 'react'

interface LoginPageProps {
  onLogin: (token: string) => void
}

export default function LoginPage({ onLogin }: LoginPageProps) {
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError('')

    try {
      const res = await fetch('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password }),
      })

      if (!res.ok) {
        const text = await res.text()
        setError(text || 'Login failed')
        return
      }

      const data = await res.json()
      onLogin(data.token)
    } catch {
      setError('Connection error')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex items-center justify-center h-screen bg-[#0a0a0b]">
      <form
        onSubmit={handleSubmit}
        className="flex flex-col gap-4 w-72 p-6 rounded-lg border border-gray-800 bg-[#111114]"
      >
        <h1 className="text-lg font-semibold text-gray-200 text-center">Agent Hive</h1>
        <input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder="Password"
          autoFocus
          className="px-3 py-2 text-sm bg-[#0c0c0e] border border-gray-700 rounded text-gray-200 placeholder-gray-600 outline-none focus:border-gray-500"
        />
        {error && (
          <p className="text-xs text-red-400 text-center">{error}</p>
        )}
        <button
          type="submit"
          disabled={loading}
          className="px-3 py-2 text-sm bg-gray-800 hover:bg-gray-700 text-gray-200 rounded transition-colors disabled:opacity-50"
        >
          {loading ? 'Logging in...' : 'Login'}
        </button>
      </form>
    </div>
  )
}
