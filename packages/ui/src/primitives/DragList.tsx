import type React from 'react'
import {
  closestCenter,
  DndContext,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
} from '@dnd-kit/core'
import type { DragEndEvent } from '@dnd-kit/core'
import {
  arrayMove,
  sortableKeyboardCoordinates,
  SortableContext,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { cx } from '../utils/cx'
import styles from './DragList.module.css'

export interface DragListItem {
  id: string
  node: React.ReactNode
}

export interface DragListProps {
  items: DragListItem[]
  onReorder: (ids: string[]) => void // full new order; unchanged drop fires nothing
  handle?: React.ReactNode // drag-handle slot; default six-dot glyph
}

function DefaultHandle() {
  return (
    <span className={styles.dots} aria-hidden="true">
      {Array.from({ length: 6 }, (_, i) => (
        <i key={i} className={styles.dot} />
      ))}
    </span>
  )
}

interface SortableRowProps {
  item: DragListItem
  handle?: React.ReactNode
}

function SortableRow({ item, handle }: SortableRowProps) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: item.id,
  })
  const style: React.CSSProperties = {
    transform: CSS.Transform.toString(transform),
    transition,
  }
  return (
    <div
      ref={setNodeRef}
      style={style}
      className={cx(styles.row, isDragging && styles.rowActive)}
    >
      <span className={styles.handle} {...attributes} {...listeners}>
        {handle ?? <DefaultHandle />}
      </span>
      {item.node}
    </div>
  )
}

export function DragList({ items, onReorder, handle }: DragListProps) {
  const sensors = useSensors(
    useSensor(PointerSensor),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  )

  function handleDragEnd(event: DragEndEvent) {
    const { active, over } = event
    if (!over) return
    const ids = items.map((item) => item.id)
    const oldIndex = ids.indexOf(String(active.id))
    const newIndex = ids.indexOf(String(over.id))
    if (oldIndex < 0 || newIndex < 0 || oldIndex === newIndex) return
    onReorder(arrayMove(ids, oldIndex, newIndex))
  }

  const ids = items.map((item) => item.id)

  return (
    <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
      <SortableContext items={ids} strategy={verticalListSortingStrategy}>
        <div className={styles.list}>
          {items.map((item) => (
            <SortableRow key={item.id} item={item} handle={handle} />
          ))}
        </div>
      </SortableContext>
    </DndContext>
  )
}