import { it, expect, afterEach, vi } from 'vitest'
import { render, screen, waitFor, cleanup, act } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useEngineEvents } from './invalidate'
import { useProfile } from './queries'
import { getHost, resetHost } from './host'
afterEach(cleanup)
function Viewer() { useEngineEvents(); const profile = useProfile('custom').data; return <div>{profile ? `${profile.core_share}:${profile.picks}` : 'loading'}</div> }
it('invalidates mounted detail on config changes and pick recording', async () => {
 resetHost()
 const host = getHost()
 const profile = { ...(await host.profiles.get('planning')), slug: 'custom', builtin: false }
 await host.profiles.create(profile)
 const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
 render(<QueryClientProvider client={client}><Viewer /></QueryClientProvider>)
 await screen.findByText(`${profile.core_share}:0`)
 await act(async () => { await host.profiles.save({ ...profile, core_share: 75, picks: 0 }) })
 await screen.findByText('75:0')
 await act(async () => { await host.pick.recordPick('custom', 'claude/opus@high') })
 await waitFor(() => expect(screen.getByText('75:1')).toBeTruthy())
})

it('refreshes company route reports on usage updates without changing personal invalidation', async () => {
 resetHost()
 const host = getHost()
 const client = new QueryClient()
 const invalidate = vi.spyOn(client, 'invalidateQueries')
 function Events() { useEngineEvents(); return null }
 render(<QueryClientProvider client={client}><Events /></QueryClientProvider>)
 client.setQueryData(['rank', 'company'], {recommendation_mode: 'score_only', candidates: [], total: 0})
 await act(async () => { await host.usage.snapshots(true) })
 expect(invalidate).toHaveBeenCalledWith({queryKey: ['rank']})
 invalidate.mockClear()
 client.setQueryData(['rank', 'company'], {candidates: [], total: 0})
 await act(async () => { await host.usage.snapshots(true) })
 expect(invalidate).not.toHaveBeenCalledWith({queryKey: ['rank']})
})
