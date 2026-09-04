import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ToastProvider } from '@which-model/ui'
import { getHost, resetHost, type MockEngineHost } from '../../../lib/host'
import { useEngineEvents } from '../../../lib/invalidate'
import { GroupsPage } from './GroupsPage'
function Events() { useEngineEvents(); return null }

describe('group membership durability', () => {
 beforeEach(() => resetHost())
 afterEach(() => { cleanup(); vi.restoreAllMocks() })
 it('persists immediately and retains the latest membership through navigation', async () => {
  const host = getHost() as MockEngineHost
  const names = await host.catalog.benchmarks()
  host.data.groups.push({ slug: 'custom', builtin: false, benchmarks: [names[0]] })
  const save = vi.spyOn(host.catalog, 'saveGroup')
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const view = render(<QueryClientProvider client={client}><ToastProvider><Events /><GroupsPage detail={{ kind: 'group', id: 'custom' }} openDetail={vi.fn()} closeDetail={vi.fn()} /></ToastProvider></QueryClientProvider>)
  await screen.findByText('custom', { selector: 'h1' })
  const toggles = screen.getAllByRole('switch')
  fireEvent.click(toggles[1])
  await waitFor(() => expect(save).toHaveBeenCalledTimes(1), { timeout: 200 })
  fireEvent.click(toggles[2])
  view.unmount()
  await waitFor(() => expect(save).toHaveBeenCalledTimes(2))
  const detail = await host.catalog.groupDetail('custom')
  expect(detail.benchmarks.filter((b) => b.on)).toHaveLength(3)
 })
})
