import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { screen, render, fireEvent, cleanup, waitFor, act } from '@testing-library/react'

afterEach(() => {
  cleanup()
  vi.useRealTimers()
})
import { createMockEngineHost } from '@which-model/core/mock'
import type { EngineHost } from '@which-model/core'
import { ToastProvider } from '@which-model/ui'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { resetHost } from '../../../lib/host'
import { SettingsApp } from '../../SettingsApp'
import { useEngineEvents } from '../../../lib/invalidate'

function makeClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } })
}

function renderApp(host: EngineHost) {
  const client = makeClient()
  function Root() {
    useEngineEvents()
    return <SettingsApp host={host} />
  }
  render(
    <QueryClientProvider client={client}>
      <ToastProvider>
        <Root />
      </ToastProvider>
    </QueryClientProvider>,
  )
}

describe('GroupsPage rename debounce cancellation', () => {
  let host: EngineHost

  beforeEach(() => {
    host = createMockEngineHost()
    resetHost(host)
  })

  it('cancels pending debounced membership save on rename and flushes toggled flags', async () => {
    // Duplicate a builtin group to obtain an editable custom group
    const customGroup = await host.catalog.duplicateGroup('reasoning')
    const oldSlug = customGroup.slug
    const newSlug = 'renamed_reasoning'

    const saveSpy = vi.spyOn(host.catalog, 'saveGroup')

    renderApp(host)

    // Navigate to Benchmark groups page
    const navBtn = await screen.findByRole('button', { name: /Benchmark groups/i })
    fireEvent.click(navBtn)

    // Open detail for the custom group
    const groupRow = await screen.findByText(oldSlug)
    fireEvent.click(groupRow)

    // Wait for the detail view to render
    const nameInput = await screen.findByDisplayValue(oldSlug)

    // Find the switches
    const toggles = screen.getAllByRole('switch')
    expect(toggles.length).toBeGreaterThan(0)

    // Switch to fake timers specifically for the debounced toggle and rename sequence
    vi.useFakeTimers()

    // Toggle the first benchmark switch (schedules 300ms saveTimer)
    fireEvent.click(toggles[0])

    // Immediately rename the group before the 300ms debounce fires
    fireEvent.change(nameInput, { target: { value: newSlug } })
    fireEvent.blur(nameInput)

    // Advance time past the 300ms debounce window
    act(() => {
      vi.advanceTimersByTime(350)
    })

    // Restore real timers so queries and async operations resolve normally
    vi.useRealTimers()

    // Wait for rename to complete
    await waitFor(() => {
      expect(saveSpy).toHaveBeenCalledWith(oldSlug, expect.any(Array), newSlug)
    })

    // Verify saveGroup was never called with (oldSlug, members) without renameTo
    const staleCalls = saveSpy.mock.calls.filter(
      (call) => call[0] === oldSlug && call[2] === undefined,
    )
    expect(staleCalls).toHaveLength(0)

    // Spurious error toast must not be shown
    expect(screen.queryByText(/save failed/i)).toBeNull()
  })
})
