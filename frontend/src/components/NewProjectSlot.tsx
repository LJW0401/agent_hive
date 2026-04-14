import { Plus } from 'lucide-react'

interface NewProjectSlotProps {
  onClick: () => void
}

export default function NewProjectSlot({ onClick }: NewProjectSlotProps) {
  return (
    <button
      onClick={onClick}
      className="flex flex-col items-center justify-center w-full h-full rounded-lg border border-dashed border-gray-700 hover:border-gray-500 bg-[#0c0c0e] hover:bg-[#111114] transition-colors cursor-pointer gap-2"
    >
      <Plus size={24} className="text-gray-600" />
      <span className="text-xs text-gray-500">New Project</span>
    </button>
  )
}
