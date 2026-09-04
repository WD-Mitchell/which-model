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
 it('keeps persisted membership after a failed rename', async () => {
  const host = getHost() as MockEngineHost
  const names = await host.catalog.benchmarks()
  host.data.groups.push({ slug: 'custom', builtin: false, benchmarks: [names[0]] })
  const original = host.catalog.saveGroup.bind(host.catalog)
  vi.spyOn(host.catalog, 'saveGroup').mockImplementation(async (slug, members, rename) => {
   if (rename) throw new Error('rename collision')
   return original(slug, members, rename)
  })
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(<QueryClientProvider client={client}><ToastProvider><Events /><GroupsPage detail={{ kind: 'group', id: 'custom' }} openDetail={vi.fn()} closeDetail={vi.fn()} /></ToastProvider></QueryClientProvider>)
  const name = await screen.findByDisplayValue('custom')
  fireEvent.click(screen.getAllByRole('switch')[1])
  fireEvent.change(name, { target: { value: 'collision' } })
  fireEvent.blur(name)
  await screen.findByText('rename collision')
  const persisted = await host.catalog.groupDetail('custom')
  expect(screen.getAllByRole('switch').filter(node => node.getAttribute('aria-checked') === 'true')).toHaveLength(persisted.benchmarks.filter(row => row.on).length)
  expect((await screen.findByDisplayValue('custom'))).toBeDefined()
 })

})
