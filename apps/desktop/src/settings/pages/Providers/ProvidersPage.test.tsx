// Copilot OAuth sign-in in the Providers page: the Sign in… button must run
// the device flow through the host (show the code, confirm on demand,
// cancel safely) instead of the old "not wired up yet" stub. Drives the real
// page through SettingsApp so the host wiring is the production one.
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

async function openCopilotDetail(host: EngineHost) {
  renderApp(host)
  await screen.findByText('which-model — Settings')
  const nav = await screen.findAllByText('Providers')
  const navBtn = nav.find((el) => el.tagName === 'BUTTON')
  act(() => navBtn!.dispatchEvent(new MouseEvent('click', { bubbles: true })))
  // The list renders provider cards showing provider.id — the fixture's
  // copilot card is the element containing exactly that id text.
  const card = await screen.findByText((_, el) => el?.tagName === 'SPAN' && el.textContent === 'copilot' && el.className.includes('id'))
  act(() => card.dispatchEvent(new MouseEvent('click', { bubbles: true })))
  // Detail loaded when the accounts header is visible.
  await screen.findByText('accounts')
}

describe('Providers page copilot sign-in', () => {
  let host: EngineHost & { data: never }
  beforeEach(() => {
    host = createMockEngineHost() as EngineHost & { data: never }
    resetHost(host)
  })

  it('starts the device flow and shows the user code', async () => {
    const startSpy = vi.spyOn(host.signin, 'start')
    await openCopilotDetail(host)
    const signIn = await screen.findByText('Sign in…')
    fireEvent.click(signIn)
    expect(await screen.findByText('WDML-MOCK')).toBeDefined()
    // The start call went to the host for copilot.
    await waitFor(() => expect(startSpy).toHaveBeenCalledTimes(1))
  })

  it('confirm closes the modal after the flow resolves', async () => {
    const confirmSpy = vi.spyOn(host.signin, 'confirm').mockResolvedValue(undefined)
    await openCopilotDetail(host)
    fireEvent.click(await screen.findByText('Sign in…'))
    await screen.findByText('WDML-MOCK')
    fireEvent.click(screen.getByText('I entered the code'))
    await waitFor(() => expect(confirmSpy).toHaveBeenCalledWith('copilot'))
    await waitFor(() => expect(screen.queryByText('WDML-MOCK')).toBeNull())
  })

  it('confirm failure surfaces the message as a toast', async () => {
    vi.spyOn(host.signin, 'confirm').mockRejectedValue({ code: 'runtime', message: 'GitHub device login expired.' })
    await openCopilotDetail(host)
    fireEvent.click(await screen.findByText('Sign in…'))
    await screen.findByText('WDML-MOCK')
    fireEvent.click(screen.getByText('I entered the code'))
    expect(await screen.findByText('GitHub device login expired.')).toBeDefined()
  })

  it('cancel abandons the flow without confirming', async () => {
    const cancelSpy = vi.spyOn(host.signin, 'cancel').mockResolvedValue(undefined)
    await openCopilotDetail(host)
    fireEvent.click(await screen.findByText('Sign in…'))
    await screen.findByText('WDML-MOCK')
    fireEvent.click(screen.getByText('Cancel'))
    await waitFor(() => expect(cancelSpy).toHaveBeenCalledWith('copilot'))
    await waitFor(() => expect(screen.queryByText('WDML-MOCK')).toBeNull())
  })
})
