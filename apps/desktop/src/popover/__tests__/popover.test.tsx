import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import { render, screen, fireEvent, waitFor, cleanup, act } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ToastProvider } from '@which-model/ui'
import { getHost, resetHost, type MockEngineHost } from '../../lib/host'
import { useOverridesStore } from '../../lib/overrides'
import { useEngineEvents } from '../../lib/invalidate'
import { useProfiles, useRank, useSettings } from '../../lib/queries'
import { PopoverApp } from '../PopoverApp'

function makeClient(): QueryClient {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } })
}

function renderApp() {
  const client = makeClient()
  return render(
    <QueryClientProvider client={client}>
      <ToastProvider>
        <AppWithEvents />
      </ToastProvider>
    </QueryClientProvider>,
  )
}

function AppWithEvents() {
  useEngineEvents()
  return <PopoverApp />
}

const initialScaleProfile = 'simple_implementation' // scale[1] — mockup initial

async function settle() {
  // Wait for the initial profile + scale to resolve.
  await screen.findAllByRole('slider')
}

function searchInput() {
  return screen.getByPlaceholderText('type to find a profile') as HTMLInputElement
}

async function pickProfile(query: string) {
  const input = searchInput()
  // Flush the query state so the combobox recomputes matches before Enter.
  await act(async () => {
    fireEvent.change(input, { target: { value: query } })
  })
  await act(async () => {
    fireEvent.keyDown(input, { key: 'Enter' })
  })
}

describe('popover landing', () => {
  beforeEach(() => {
    resetHost()
    useOverridesStore.getState().clear()
  })

  afterEach(() => {
    cleanup()
    vi.restoreAllMocks()
  })

  it('renders hero, catalog line and launch in first harness', async () => {
    renderApp()
    expect(
      await screen.findByText('The right model for the job in front of you.'),
    ).toBeTruthy()
    expect(await screen.findByText(/models · .* providers on · .* harnesses/)).toBeTruthy()
    expect(await screen.findByText('Launch in Claude Code')).toBeTruthy()
    // rank resolved with a non-empty pick
    expect(await screen.findByText(/^rank \d+ of \d+$/)).toBeTruthy()
  })

  it('profile pick refetches rank with the new slug and reseeds overrides', async () => {
    const host = getHost() as MockEngineHost
    const rankSpy = vi.spyOn(host.pick, 'rank')
    renderApp()
    await settle()

    await pickProfile('review')
    await waitFor(() => {
      expect(rankSpy).toHaveBeenLastCalledWith(
        expect.objectContaining({ profile_slug: 'review', overrides: undefined }),
      )
    })
    expect(useOverridesStore.getState().baseSlug).toBe('review')
    expect(useOverridesStore.getState().isDirty(host.data.profiles.find((p) => p.slug === 'review')!)).toBe(
      false,
    )
    expect(searchInput().value).toBe('')
  })

  it('sticky stop: off-scale pick keeps the handle; scale pick moves it', async () => {
    renderApp()
    await settle()
    const slider = () => screen.getByRole('slider')
    expect(slider().getAttribute('aria-valuenow')).toBe('1')

    // Off-scale profile (not on the 5-stop scale) — handle keeps its stop.
    await pickProfile('review')
    await waitFor(() => expect(useOverridesStore.getState().baseSlug).toBe('review'))
    expect(slider().getAttribute('aria-valuenow')).toBe('1')

    // Scale profile — handle moves to that stop.
    await pickProfile('planning')
    await waitFor(() => expect(slider().getAttribute('aria-valuenow')).toBe('4'))
  })

  it('search filters profiles and shows the no-match row', async () => {
    renderApp()
    await settle()
    const input = searchInput()
    fireEvent.change(input, { target: { value: 'zzz-nope' } })
    expect(await screen.findByText('no profile by that name')).toBeTruthy()

    fireEvent.change(input, { target: { value: 'research' } })
    // 'research' + 'research_fast' both match (substring, cap 5)
    await waitFor(() => expect(screen.getAllByText(/research/).length).toBeGreaterThanOrEqual(2))
  })

  it('weights-view navigation', async () => {
    renderApp()
    await settle()

    fireEvent.click(screen.getByLabelText('App menu'))
    fireEvent.click(await screen.findByText('Custom weights…'))
    expect(await screen.findByText(/Weights for/)).toBeTruthy()

    // back to landing
    fireEvent.click(screen.getByLabelText('Back to landing'))
    expect(await screen.findByText('which-model')).toBeTruthy()
  })

  it('launch spawn mode toasts the command and never copies', async () => {
    const host = getHost() as MockEngineHost
    const clip = vi.spyOn(host.window, 'copyToClipboard').mockResolvedValue(undefined)
    renderApp()
    await settle()
    await screen.findByText(/^rank \d+ of \d+$/)

    fireEvent.click(screen.getByText('Launch in Claude Code'))
    expect(await screen.findByText(/^claude --model /)).toBeTruthy()
    expect(clip).not.toHaveBeenCalled()
  })

  it('launch copy mode copies once and toasts the command', async () => {
    const host = getHost() as MockEngineHost
    const clip = vi.spyOn(host.window, 'copyToClipboard').mockResolvedValue(undefined)
    host.data.settings.copy_command_instead = true
    renderApp()
    await settle()
    await screen.findByText(/^rank \d+ of \d+$/)

    fireEvent.click(screen.getByText('Launch in Claude Code'))
    await waitFor(() => expect(clip).toHaveBeenCalledTimes(1))
    expect(clip).toHaveBeenCalledWith(expect.stringMatching(/^claude --model /))
    expect(await screen.findByText(/^claude --model /)).toBeTruthy()
  })

  it('launch no pick toasts and never calls launch', async () => {
    const host = getHost() as MockEngineHost
    const launchSpy = vi.spyOn(host.harnesses, 'launch').mockResolvedValue({
      copied: false,
      command: '',
    })
    // Disable every provider via mutators so config:changed invalidates rank.
    for (const p of [...host.data.providers]) {
      if (p.on) await host.providers.setEnabled(p.id, false)
    }
    renderApp()
    await settle()
    expect(await screen.findByText('Enable a provider')).toBeTruthy()
    expect(await screen.findByText('no route')).toBeTruthy()

    fireEvent.click(screen.getByText('Launch in Claude Code'))
    expect(await screen.findByText('no model to launch — enable a provider')).toBeTruthy()
    expect(launchSpy).not.toHaveBeenCalled()
  })

  it('close after launch calls hidePopover only when enabled', async () => {
    const host = getHost() as MockEngineHost
    const getSpy = vi.spyOn(host.settings, 'get')
    const hide = vi.spyOn(host.window, 'hidePopover').mockResolvedValue(undefined)
    await host.settings.set({ ...host.data.settings, close_popover_after_launch: true })
    renderApp()
    await settle()
    await screen.findByText(/^rank \d+ of \d+$/)

    fireEvent.click(screen.getByText('Launch in Claude Code'))
    await waitFor(() => expect(hide).toHaveBeenCalledTimes(1))

    // Disable it, let settings refetch, relaunch — no second hide.
    hide.mockClear()
    await host.settings.set({ ...host.data.settings, close_popover_after_launch: false })
    await waitFor(() => expect(getSpy).toHaveBeenCalledTimes(2))
    fireEvent.click(screen.getByText('Launch in Claude Code'))
    await screen.findByText(/^claude --model /)
    expect(hide).not.toHaveBeenCalled()
  })

  it('harness selection lives in memory only and updates the label', async () => {
    const host = getHost() as MockEngineHost
    const setSpy = vi.spyOn(host.settings, 'set')
    const saveSpy = vi.spyOn(host.harnesses, 'save')
    renderApp()
    await settle()

    const chevron = document.querySelector('[data-launch-pill] [role="button"]') as HTMLElement
    expect(chevron).toBeTruthy()
    fireEvent.click(chevron)
    fireEvent.click(await screen.findByText('Codex CLI'))

    expect(await screen.findByText('Launch in Codex CLI')).toBeTruthy()
    expect(setSpy).not.toHaveBeenCalled()
    expect(saveSpy).not.toHaveBeenCalled()
  })

  it('event invalidation: config:changed and settings:changed refetch by map', async () => {
    const host = getHost() as MockEngineHost
    const listSpy = vi.spyOn(host.profiles, 'list')
    const getSpy = vi.spyOn(host.settings, 'get')
    const rankSpy = vi.spyOn(host.pick, 'rank')

    const client = makeClient()
    render(
      <QueryClientProvider client={client}>
        <Probe />
      </QueryClientProvider>,
    )
    // initial fetches
    await waitFor(() => expect(listSpy).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(getSpy).toHaveBeenCalledTimes(1))

    // config:changed ⇒ profiles + rank refetch
    await host.providers.setEnabled('cursor', true)
    await waitFor(() => expect(listSpy).toHaveBeenCalledTimes(2))
    await waitFor(() => expect(rankSpy.mock.calls.length).toBeGreaterThanOrEqual(2))

    // settings:changed ⇒ settings refetch
    await host.settings.set(host.data.settings)
    await waitFor(() => expect(getSpy).toHaveBeenCalledTimes(2))
  })
})

function Probe() {
  useEngineEvents()
  useProfiles()
  useSettings()
  useRank(initialScaleProfile, 'none', 5)
  return null
}