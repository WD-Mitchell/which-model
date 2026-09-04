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

describe('profile create-only flow', () => {
 beforeEach(() => resetHost())
 afterEach(() => { cleanup(); vi.restoreAllMocks() })
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
