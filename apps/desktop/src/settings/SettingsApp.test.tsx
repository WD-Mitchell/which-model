// U07 CONTRACTS §6 — SettingsApp tests: nav clearing detail, back semantics.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { screen, render, act, fireEvent, cleanup } from '@testing-library/react'

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

  it('calls window.closeSettings when the close dot is clicked', async () => {
    const spy = vi.spyOn(host.window, 'closeSettings').mockResolvedValue(undefined)
    renderApp(host)
    const closeBtn = (await screen.findAllByLabelText('Close settings'))[0]
    fireEvent.click(closeBtn)
    expect(spy).toHaveBeenCalledTimes(1)
  })
})