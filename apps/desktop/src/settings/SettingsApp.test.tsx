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