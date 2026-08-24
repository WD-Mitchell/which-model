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
  /** Merged onto every row, after the built-in classes. The row is the element
   *  the handle and the item node share, so a caller that needs the mockup's
   *  row rule and 22px gutter (demo.dc.html 745) puts them here rather than
   *  inside `node`, where they would leave the handle outside the padding. */
  rowClassName?: string
}

/** The mockup's six-dot grab glyph, ported verbatim (demo.dc.html 746). */
function DefaultHandle() {
  return (
    <svg width="11" height="11" viewBox="0 0 12 12" fill="currentColor" stroke="none" aria-hidden="true">
      <circle cx="4" cy="2.5" r="1" />
      <circle cx="8" cy="2.5" r="1" />
      <circle cx="4" cy="6" r="1" />
      <circle cx="8" cy="6" r="1" />
      <circle cx="4" cy="9.5" r="1" />
      <circle cx="8" cy="9.5" r="1" />
    </svg>
  )
}

interface SortableRowProps {
  item: DragListItem
  handle?: React.ReactNode
  rowClassName?: string
}

function SortableRow({ item, handle, rowClassName }: SortableRowProps) {
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
      className={cx(styles.row, rowClassName, isDragging && styles.rowActive)}
    >
      <span className={cx('ib', styles.handle)} {...attributes} {...listeners}>
        {handle ?? <DefaultHandle />}
      </span>
      {item.node}
    </div>
  )
}

export function DragList({ items, onReorder, handle, rowClassName }: DragListProps) {
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
            <SortableRow key={item.id} item={item} handle={handle} rowClassName={rowClassName} />
          ))}
        </div>
      </SortableContext>
    </DndContext>
  )
}