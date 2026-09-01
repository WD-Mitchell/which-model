// U07 CONTRACTS §6 — SettingsApp tests: nav clearing detail, back semantics.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { screen, render, act, fireEvent, cleanup, waitFor } from '@testing-library/react'

afterEach(() => cleanup())
import { createMockEngineHost } from '@which-model/core/mock'
import type { EngineHost } from '@which-model/core'
import { ToastProvider } from '@which-model/ui'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { resetHost } from '../lib/host'
import { SettingsApp } from './SettingsApp'

function makeClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } })
}

function renderApp(host: EngineHost) {
  const client = makeClient()
  const utils = render(
    <QueryClientProvider client={client}>
      <ToastProvider>
        <SettingsApp host={host} />
      </ToastProvider>
    </QueryClientProvider>,
  )
  return { ...utils, client }
}

describe('SettingsApp', () => {
  let host: EngineHost & { data: never }
  beforeEach(() => {
    resetHost()
    host = createMockEngineHost() as EngineHost & { data: never }
  })

  it('renders the General page by default', async () => {
    renderApp(host)
    expect(await screen.findByText('How which-model runs on this Mac, and how the pick is drawn in the popover.')).toBeDefined()
  })

  // Issue: the user could not tell which build was running. The sidebar
  // footer shows the bare version, REPLACING the config-path line.
  it('shows the bare version in the sidebar footer, replacing the config path', async () => {
    renderApp(host)
    expect(await screen.findByText('which-model dev (commit unknown, built unknown)')).toBeDefined()
    expect(screen.queryByText('~/Library/Application Support/which-model/config.toml')).toBeNull()
  })

  it('persists the system-keychain preference as one settings delta', async () => {
    const initial = await host.settings.get()
    const spy = vi.spyOn(host.settings, 'set')
    resetHost(host)
    renderApp(host)
    const label = await screen.findByText('Store sign-ins in system keychain')
    const toggle = label.closest('div')?.querySelector('[role="switch"]')
    expect(toggle).not.toBeNull()
    fireEvent.click(toggle!)
    await waitFor(() => expect(spy).toHaveBeenCalledTimes(1))
    expect(spy).toHaveBeenCalledWith({ ...initial, use_keychain: false })
  })

  it('navigates to a page when a sidebar item is clicked', async () => {
    renderApp(host)
    const profiles = await screen.findAllByText('Profiles')
    const navBtn = profiles.find((el) => el.tagName === 'BUTTON')
    act(() => navBtn!.dispatchEvent(new MouseEvent('click', { bubbles: true })))
    expect(await screen.findByText('Built-in profiles are read-only; duplicate one to edit its weights.')).toBeDefined()
  })

  it('requests an engine-valid rank for the Favourites add-model search', async () => {
    const rank = vi.spyOn(host.pick, 'rank')
    resetHost(host)
    renderApp(host)
    const favourites = await screen.findAllByText('Favourites')
    const navBtn = favourites.find((el) => el.tagName === 'BUTTON')
    act(() => navBtn!.dispatchEvent(new MouseEvent('click', { bubbles: true })))
    await screen.findByText('Pinned models are offered first when they rank in range for the profile.')
    await waitFor(() => expect(rank).toHaveBeenCalled())
    expect(rank).toHaveBeenCalledWith(
      expect.objectContaining({
        profile_slug: 'balanced_implementation',
        holds: 0,
      }),
    )
    fireEvent.click(screen.getByRole('button', { name: 'Add model' }))
    expect(await screen.findByText('Claude Sonnet 5.2')).toBeDefined()
  })
})

describe('SettingsShell close', () => {
  let host: EngineHost & { data: never }
  beforeEach(() => {
    resetHost()
    host = createMockEngineHost() as EngineHost & { data: never }
  })

  // The shell draws NO window buttons: AppKit draws the real ones over the
  // titlebar (settings.go, MacTitleBarHidden), which is what gives them
  // standard sizing, hover symbols and a working zoom on a resizable window.
  // Escape is the web-side close.
  it('calls window.closeSettings when Escape is pressed', async () => {
    const spy = vi.spyOn(host.window, 'closeSettings').mockResolvedValue(undefined)
    renderApp(host)
    await screen.findByText('How which-model runs on this Mac, and how the pick is drawn in the popover.')
    fireEvent.keyDown(document.body, { key: 'Escape' })
    expect(spy).toHaveBeenCalledTimes(1)
  })

  // Issue #31: Escape inside an open Combobox belongs to the Combobox (it
  // stopPropagation()s), so the window must NOT close. Fires from the input
  // — the same surface the shell's window listener would see — without any
  // dialog mounted.
  it('keeps the window open when Escape is pressed inside a Combobox', async () => {
    const spy = vi.spyOn(host.window, 'closeSettings').mockResolvedValue(undefined)
    renderApp(host)
    await screen.findByText('which-model — Settings')
    // Open the Providers page and its add-provider Combobox (U07 §5 action
    // label), then press Escape from the search input. The Combobox owns
    // Escape; the window must stay open.
    const navProviders = await screen.findAllByText('Providers')
    const navBtn = navProviders.find((el) => el.tagName === 'BUTTON')
    act(() => navBtn!.dispatchEvent(new MouseEvent('click', { bubbles: true })))
    fireEvent.click(await screen.findByRole('button', { name: 'Add provider' }))
    const input = await screen.findByPlaceholderText('search the model catalogue…')
    fireEvent.input(input, { target: { value: 'zzz' } })
    fireEvent.keyDown(input, { key: 'Escape' })
    expect(spy).not.toHaveBeenCalled()
  })

  it('draws no window buttons of its own — AppKit owns them', async () => {
    renderApp(host)
    await screen.findByText('which-model — Settings')
    // A web-drawn set would shadow the native one and could not reproduce the
    // hover symbols or press states, so there must be none.
    expect(screen.queryByLabelText('Close settings')).toBeNull()
    expect(screen.queryByLabelText('Minimise settings')).toBeNull()
    expect(screen.queryByLabelText('Zoom settings')).toBeNull()
  })
})