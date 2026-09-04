import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ToastProvider } from '@which-model/ui'
import { getHost, resetHost, type MockEngineHost } from '../../../lib/host'
import { useEngineEvents } from '../../../lib/invalidate'
import { ProfilesPage } from './ProfilesPage'

function Events() { useEngineEvents(); return null }
function renderPage(slug?: string) {
 const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
 const open = vi.fn(), close = vi.fn()
 return { ...render(<QueryClientProvider client={client}><ToastProvider><Events /><ProfilesPage detail={slug ? { kind: 'profile', id: slug } : null} openDetail={open} closeDetail={close} /></ToastProvider></QueryClientProvider>), open, close }
}
async function custom() {
 const host = getHost() as MockEngineHost
 const profile = { ...(await host.profiles.get('planning')), slug: 'custom', name: 'Custom', builtin: false, tier2_weights: { planning_capability: 1 } }
 await host.profiles.save(profile)
 return { host, profile }
}

describe('profile editor persistence', () => {
 beforeEach(() => resetHost())
 afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.useRealTimers() })
 it('flushes before duplicate and before delete without resurrecting a deleted profile', async () => {
  const { host } = await custom()
  const view = renderPage('custom')
  const intelligence = await screen.findByRole('slider', { name: 'intelligence' })
  for (let i = 0; i < 4; i++) fireEvent.keyDown(intelligence, { key: 'ArrowLeft' })
  fireEvent.click(screen.getByRole('button', { name: 'Duplicate' }))
  await waitFor(() => expect(view.open).toHaveBeenCalled())
  expect((await host.profiles.get('custom_copy')).tier1_weights.intelligence).toBe(1)
  fireEvent.keyDown(intelligence, { key: 'ArrowRight' })
  fireEvent.click(screen.getByRole('button', { name: 'Delete this profile' }))
  await waitFor(() => expect(view.close).toHaveBeenCalled())
  view.unmount()
  await expect(host.profiles.get('custom')).rejects.toMatchObject({ code: 'not_found' })
 })
 it('uses create-only suffix retries when list-count naming collides', async () => {
  const { host, profile } = await custom()
  const n = host.data.profiles.length + 2
  await host.profiles.save({ ...profile, slug: `profile_${n}`, name: `Existing ${n}`, core_share: 75 })
  const create = vi.spyOn(host.profiles, 'create')
  const view = renderPage()
  await screen.findByText('Custom')
  fireEvent.click(screen.getByRole('button', { name: 'New profile' }))
  await waitFor(() => expect(view.open).toHaveBeenCalledWith({ kind: 'profile', id: `profile_${n + 1}` }))
  expect(create).toHaveBeenCalledTimes(2)
  expect((await host.profiles.get(`profile_${n}`)).core_share).toBe(75)
 })
})

it('waits for the prior mount write before enabling a reopened editor', async () => {
 resetHost()
 const { host } = await custom()
 let release!: () => void
 const originalSave = host.profiles.save
 vi.spyOn(host.profiles, 'save').mockImplementationOnce(async (profile) => {
  await new Promise<void>((resolve) => { release = resolve })
  await originalSave(profile)
 })
 const first = renderPage('custom')
 fireEvent.keyDown(await screen.findByRole('slider', { name: 'intelligence' }), { key: 'ArrowLeft' })
 first.unmount()
 const second = renderPage('custom')
 const intelligence = await screen.findByRole('slider', { name: 'intelligence' })
 expect(intelligence.getAttribute('tabindex')).toBeNull()
 await act(async () => { release() })
 await waitFor(() => expect(intelligence.getAttribute('aria-valuenow')).toBe('4'))
 await waitFor(() => expect(intelligence.getAttribute('tabindex')).toBe('0'))
 fireEvent.keyDown(intelligence, { key: 'ArrowLeft' })
 second.unmount()
 await waitFor(async () => expect((await host.profiles.get('custom')).tier1_weights.intelligence).toBe(3))
 cleanup(); vi.restoreAllMocks()
})

it('list delete waits for the previous editor save', async () => {
 resetHost()
 const { host } = await custom()
 let release!: () => void
 const original = host.profiles.save.bind(host.profiles)
 vi.spyOn(host.profiles, 'save').mockImplementationOnce(async profile => {
  await new Promise<void>(resolve => { release = resolve })
  await original(profile)
 })
 const deleted = vi.spyOn(host.profiles, 'delete')
 const editor = renderPage('custom')
 fireEvent.keyDown(await screen.findByRole('slider', { name: 'intelligence' }), { key: 'ArrowLeft' })
 editor.unmount()
 const list = renderPage()
 fireEvent.click(await screen.findByRole('button', { name: 'Delete custom' }))
 expect(deleted).not.toHaveBeenCalled()
 await act(async () => { release() })
 await waitFor(() => expect(deleted).toHaveBeenCalledTimes(1))
 await expect(host.profiles.get('custom')).rejects.toMatchObject({ code: 'not_found' })
 list.unmount(); vi.restoreAllMocks()
})
