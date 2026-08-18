import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { ReactNode } from 'react'
import { DragList } from '../DragList'
import type { DragListItem } from '../DragList'
import styles from '../DragList.module.css'

function makeRect(top: number, height: number, width = 200, left = 0) {
  return {
    top,
    left,
    width,
    height,
    bottom: top + height,
    right: left + width,
    x: left,
    y: top,
  }
}

function stubLayout() {
  const rows = Array.from(document.querySelectorAll<HTMLElement>(`.${styles.row}`))
  rows.forEach((el, i) => {
    el.getBoundingClientRect = () => makeRect(i * 60, 60) as DOMRect
  })
  const list = document.querySelector<HTMLElement>(`.${styles.list}`)
  list?.setAttribute('data-stub', '1')
  if (list) list.getBoundingClientRect = () => makeRect(0, rows.length * 60) as DOMRect
  return rows
}

const items: DragListItem[] = [
  { id: 'a', node: <span>A</span> },
  { id: 'b', node: <span>B</span> },
]

describe('DragList', () => {
  it('reorders on a drag that moves id a below id b', async () => {
    const onReorder = vi.fn()
    render(<DragList items={items} onReorder={onReorder} />)
    const [handleA] = screen.getAllByRole('button')
    stubLayout()

    fireEvent.pointerDown(handleA, {
      clientX: 5,
      clientY: 30,
      pointerType: 'mouse',
      button: 0,
      isPrimary: true,
    })
    // move to the second row's vertical center (row b at top 60)
    fireEvent.pointerMove(document, { clientX: 5, clientY: 90 })
    fireEvent.pointerUp(document, { clientX: 5, clientY: 90 })

    expect(onReorder).toHaveBeenCalledTimes(1)
    expect(onReorder).toHaveBeenCalledWith(['b', 'a'])
  })

  it('fires nothing for a no-move drop', () => {
    const onReorder = vi.fn()
    render(<DragList items={items} onReorder={onReorder} />)
    const [handleA] = screen.getAllByRole('button')
    stubLayout()

    fireEvent.pointerDown(handleA, {
      clientX: 5,
      clientY: 30,
      pointerType: 'mouse',
      button: 0,
      isPrimary: true,
    })
    fireEvent.pointerUp(document, { clientX: 5, clientY: 30 })

    expect(onReorder).not.toHaveBeenCalled()
  })
})