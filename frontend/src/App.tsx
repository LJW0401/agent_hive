import Terminal from './components/Terminal'

export default function App() {
  return (
    <div className="flex flex-col h-screen bg-[#0a0a0b]">
      <header className="flex items-center px-4 h-12 border-b border-gray-800 shrink-0">
        <h1 className="text-sm font-semibold text-gray-200 tracking-wide">
          Agent Hive
        </h1>
      </header>
      <main className="flex-1 min-h-0">
        <Terminal />
      </main>
    </div>
  )
}
