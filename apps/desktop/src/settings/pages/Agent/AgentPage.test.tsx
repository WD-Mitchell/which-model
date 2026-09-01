// Issue #42 — AgentPage clipboard error handling: a failed copy must surface
// a toast, not reject unhandled. Drives the real page through SettingsApp so
// the toast provider and host wiring are the production ones.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { screen, render, act, fireEvent, cleanup, waitFor } from '@testing-library/react'

afterEach(() => cleanup())
import { createMockEngineHost } from '@which-model/core/mock'
import type { EngineHost } from '@which-model/core'
import { ToastProvider } from '@which-model/ui'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { resetHost } from '../../../lib/host'
import { SettingsApp } from '../../SettingsApp'

function makeClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } })
}

function renderApp(host: EngineHost) {
  const client = makeClient()
  render(
    <QueryClientProvider client={client}>
      <ToastProvider>
        <SettingsApp host={host} />
      </ToastProvider>
    </QueryClientProvider>,
  )
}

describe('AgentPage clipboard', () => {
  let host: EngineHost & { data: never }
  beforeEach(() => {
    host = createMockEngineHost() as EngineHost & { data: never }
    // The page reads through getHost(); register the host BEFORE rendering
    // so the spy below intercepts the singleton the page actually uses.
    resetHost(host)
  })

  async function openAgentPageAndCopy() {
    renderApp(host)
    await screen.findByText('which-model — Settings')
    const nav = await screen.findAllByText('Agent integration')
    const navBtn = nav.find((el) => el.tagName === 'BUTTON')
    act(() => navBtn!.dispatchEvent(new MouseEvent('click', { bubbles: true })))
    // The snippet box is a clickable <pre> (U14 §3.2), not a button. It
    // renders once the ['snippets'] query settles — wait for the preview
    // line, whose $ prefix is split across text nodes.
    await screen.findByText((_, el) => el?.tagName === 'PRE')
    const snippet = document.querySelector('pre')
    expect(snippet).not.toBeNull()
    fireEvent.click(snippet!)
  }

  it('calls copyToClipboard on the registered host', async () => {
    const spy = vi.spyOn(host.window, 'copyToClipboard').mockResolvedValue(undefined)
    await openAgentPageAndCopy()
    await waitFor(() => expect(spy).toHaveBeenCalledTimes(1))
  })

  it('toasts copied on success', async () => {
    const spy = vi.spyOn(host.window, 'copyToClipboard').mockResolvedValue(undefined)
    await openAgentPageAndCopy()
    await waitFor(() => expect(spy).toHaveBeenCalled())
    expect(await screen.findByText('copied')).toBeDefined()
  })

  it('toasts the failure message instead of failing silently', async () => {
    vi.spyOn(host.window, 'copyToClipboard').mockRejectedValue({ code: 'io_error', message: 'clipboard denied' })
    await openAgentPageAndCopy()
    expect(await screen.findByText('clipboard denied')).toBeDefined()
  })

  it('toasts the Error message when the rejection carries one', async () => {
    vi.spyOn(host.window, 'copyToClipboard').mockRejectedValue(new Error('boom'))
    await openAgentPageAndCopy()
    expect(await screen.findByText('boom')).toBeDefined()
  })
})
