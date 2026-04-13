import { useEffect, useState, useRef } from 'react'
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core'
import {
  SortableContext,
  sortableKeyboardCoordinates,
  verticalListSortingStrategy,
  useSortable,
  arrayMove,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { Plus, Trash2, GripVertical } from 'lucide-react'
import {
  listTodos,
  createTodo,
  updateTodo,
  deleteTodo,
  reorderTodos,
  type Todo,
} from '../api'

interface TodoListProps {
  containerID: string
}

export default function TodoList({ containerID }: TodoListProps) {
  const [todos, setTodos] = useState<Todo[]>([])
  const [newContent, setNewContent] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    listTodos(containerID).then(setTodos)
  }, [containerID])

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  )

  const handleAdd = async () => {
    const content = newContent.trim()
    if (!content) return
    const todo = await createTodo(containerID, content)
    setTodos((prev) => [...prev, todo])
    setNewContent('')
    inputRef.current?.focus()
  }

  const handleToggle = async (todo: Todo) => {
    const done = !todo.done
    await updateTodo(containerID, todo.id, todo.content, done)
    setTodos((prev) => prev.map((t) => (t.id === todo.id ? { ...t, done } : t)))
  }

  const handleEdit = async (todo: Todo, content: string) => {
    await updateTodo(containerID, todo.id, content, todo.done)
    setTodos((prev) => prev.map((t) => (t.id === todo.id ? { ...t, content } : t)))
  }

  const handleDelete = async (id: number) => {
    await deleteTodo(containerID, id)
    setTodos((prev) => prev.filter((t) => t.id !== id))
  }

  const handleDragEnd = async (event: DragEndEvent) => {
    const { active, over } = event
    if (!over || active.id === over.id) return

    const oldIndex = todos.findIndex((t) => t.id === active.id)
    const newIndex = todos.findIndex((t) => t.id === over.id)
    const reordered = arrayMove(todos, oldIndex, newIndex)
    setTodos(reordered)
    await reorderTodos(containerID, reordered.map((t) => t.id))
  }

  return (
    <div className="flex flex-col h-full text-xs">
      {/* Add todo input */}
      <form
        className="flex items-center gap-1 p-1.5 border-b border-gray-800"
        onSubmit={(e) => { e.preventDefault(); handleAdd() }}
      >
        <input
          ref={inputRef}
          value={newContent}
          onChange={(e) => setNewContent(e.target.value)}
          placeholder="Add todo..."
          className="flex-1 min-w-0 bg-transparent text-gray-300 placeholder-gray-600 outline-none text-xs py-0.5"
        />
        <button
          type="submit"
          className="text-gray-600 hover:text-gray-400 p-0.5 shrink-0"
        >
          <Plus size={12} />
        </button>
      </form>

      {/* Todo list */}
      <div className="flex-1 overflow-y-auto">
        <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
          <SortableContext items={todos.map((t) => t.id)} strategy={verticalListSortingStrategy}>
            {todos.map((todo) => (
              <SortableTodoItem
                key={todo.id}
                todo={todo}
                onToggle={handleToggle}
                onEdit={handleEdit}
                onDelete={handleDelete}
              />
            ))}
          </SortableContext>
        </DndContext>
        {todos.length === 0 && (
          <div className="text-center text-gray-700 py-4 text-[10px]">No todos yet</div>
        )}
      </div>
    </div>
  )
}

interface SortableTodoItemProps {
  todo: Todo
  onToggle: (todo: Todo) => void
  onEdit: (todo: Todo, content: string) => void
  onDelete: (id: number) => void
}

function SortableTodoItem({ todo, onToggle, onEdit, onDelete }: SortableTodoItemProps) {
  const [editing, setEditing] = useState(false)
  const [content, setContent] = useState(todo.content)
  const inputRef = useRef<HTMLInputElement>(null)

  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: todo.id })

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  }

  const commitEdit = () => {
    const trimmed = content.trim()
    if (trimmed && trimmed !== todo.content) {
      onEdit(todo, trimmed)
    } else {
      setContent(todo.content)
    }
    setEditing(false)
  }

  return (
    <div
      ref={setNodeRef}
      style={style}
      className="flex items-center gap-1 px-1.5 py-1 border-b border-gray-800/50 group hover:bg-gray-800/30"
    >
      {/* Drag handle */}
      <button
        className="text-gray-700 hover:text-gray-500 cursor-grab active:cursor-grabbing p-0.5 shrink-0 opacity-0 group-hover:opacity-100 transition-opacity"
        {...attributes}
        {...listeners}
      >
        <GripVertical size={10} />
      </button>

      {/* Checkbox */}
      <button
        onClick={() => onToggle(todo)}
        className={`w-3 h-3 rounded-sm border shrink-0 flex items-center justify-center transition-colors ${
          todo.done
            ? 'bg-emerald-600 border-emerald-600'
            : 'border-gray-600 hover:border-gray-400'
        }`}
      >
        {todo.done && (
          <svg width="8" height="8" viewBox="0 0 8 8" fill="none">
            <path d="M1.5 4L3 5.5L6.5 2" stroke="white" strokeWidth="1.2" strokeLinecap="round" strokeLinejoin="round"/>
          </svg>
        )}
      </button>

      {/* Content */}
      {editing ? (
        <form
          className="flex-1 min-w-0"
          onSubmit={(e) => { e.preventDefault(); commitEdit() }}
        >
          <input
            ref={inputRef}
            value={content}
            onChange={(e) => setContent(e.target.value)}
            onBlur={commitEdit}
            autoFocus
            className="w-full bg-transparent text-gray-300 outline-none text-xs border-b border-gray-600 py-0"
          />
        </form>
      ) : (
        <span
          onDoubleClick={() => setEditing(true)}
          className={`flex-1 min-w-0 truncate cursor-default select-none ${
            todo.done ? 'text-gray-600 line-through' : 'text-gray-300'
          }`}
        >
          {todo.content}
        </span>
      )}

      {/* Delete */}
      <button
        onClick={() => onDelete(todo.id)}
        className="text-gray-700 hover:text-red-400 p-0.5 shrink-0 opacity-0 group-hover:opacity-100 transition-opacity"
      >
        <Trash2 size={10} />
      </button>
    </div>
  )
}
