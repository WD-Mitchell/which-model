// Copilot OAuth sign-in in the Providers page: Sign in… copies the code,
// opens GitHub, and starts polling immediately. Cancel aborts. Success
// marks the account signed in. Drives the real page through SettingsApp.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { screen, render, act, fireEvent, cleanup, waitFor } from '@testing-library/react'

afterEach(() => cleanup())
import { createMockEngineHost } from '@which-model/core/mock'
import type { EngineHost } from '@which-model/core'
import { ToastProvider } from '@which-model/ui'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { resetHost } from '../../../lib/host'
import { useEngineEvents } from '../../../lib/invalidate'
import { SettingsApp } from '../../SettingsApp'

function makeClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } })
}

function Root({ host }: { host: EngineHost }) {
  useEngineEvents()
  return <SettingsApp host={host} />
}

function renderApp(host: EngineHost) {
  const client = makeClient()
  render(
    <QueryClientProvider client={client}>
      <ToastProvider>
        <Root host={host} />
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
  const card = await screen.findByText((_, el) => el?.tagName === 'SPAN' && el.textContent === 'copilot' && el.className.includes('id'))
  act(() => card.dispatchEvent(new MouseEvent('click', { bubbles: true })))
  await screen.findByText('accounts')
}

function deferConfirm(host: EngineHost) {
  let resolve!: () => void
  let reject!: (e: unknown) => void
  const pending = new Promise<void>((res, rej) => {
    resolve = res
    reject = rej
  })
  const confirmSpy = vi.spyOn(host.signin, 'confirm').mockReturnValue(pending)
  return { confirmSpy, resolve, reject }
}

describe('Providers page copilot sign-in', () => {
  let host: EngineHost & { data: never }
  beforeEach(() => {
    host = createMockEngineHost() as EngineHost & { data: never }
    resetHost(host)
  })

  it('starts the device flow, auto-copies the code, auto-opens GitHub, and polls', async () => {
    const { confirmSpy } = deferConfirm(host)
    const startSpy = vi.spyOn(host.signin, 'start')
    const copySpy = vi.spyOn(host.window, 'copyToClipboard').mockResolvedValue(undefined)
    const openSpy = vi.spyOn(host.window, 'openURL').mockResolvedValue(undefined)
    await openCopilotDetail(host)
    fireEvent.click(await screen.findByText('Sign in…'))
    expect(await screen.findByText('WDML-MOCK')).toBeDefined()
    await waitFor(() => expect(startSpy).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(copySpy).toHaveBeenCalledWith('WDML-MOCK'))
    await waitFor(() =>
      expect(openSpy).toHaveBeenCalledWith('https://github.com/login/device?user_code=WDML-MOCK'),
    )
    await waitFor(() => expect(confirmSpy).toHaveBeenCalledWith('copilot'))
    expect(screen.queryByText('I entered the code')).toBeNull()
  })

  it('confirm resolves the modal and shows signed in', async () => {
    const { confirmSpy, resolve } = deferConfirm(host)
    await openCopilotDetail(host)
    fireEvent.click(await screen.findByText('Sign in…'))
    await screen.findByText('WDML-MOCK')
    await waitFor(() => expect(confirmSpy).toHaveBeenCalledWith('copilot'))
    resolve()
    await waitFor(() => expect(screen.queryByText('WDML-MOCK')).toBeNull())
    expect(await screen.findByText('signed in')).toBeDefined()
    expect(screen.getByText('Re-authenticate')).toBeDefined()
  })

  it('confirm failure surfaces the message as a toast', async () => {
    const { reject } = deferConfirm(host)
    await openCopilotDetail(host)
    fireEvent.click(await screen.findByText('Sign in…'))
    await screen.findByText('WDML-MOCK')
    reject({ code: 'runtime', message: 'GitHub device login expired.' })
    expect(await screen.findByText('GitHub device login expired.')).toBeDefined()
  })

  it('open-website button hands the verification URI with the user code', async () => {
    deferConfirm(host)
    const openSpy = vi.spyOn(host.window, 'openURL').mockResolvedValue(undefined)
    await openCopilotDetail(host)
    fireEvent.click(await screen.findByText('Sign in…'))
    await screen.findByText('WDML-MOCK')
    openSpy.mockClear()
    fireEvent.click(await screen.findByText('Open github.com/login/device'))
    await waitFor(() =>
      expect(openSpy).toHaveBeenCalledWith('https://github.com/login/device?user_code=WDML-MOCK'),
    )
  })

  it('cancel abandons the flow without treating it as success', async () => {
    deferConfirm(host)
    const cancelSpy = vi.spyOn(host.signin, 'cancel').mockResolvedValue(undefined)
    await openCopilotDetail(host)
    fireEvent.click(await screen.findByText('Sign in…'))
    await screen.findByText('WDML-MOCK')
    fireEvent.click(screen.getByText('Cancel'))
    await waitFor(() => expect(cancelSpy).toHaveBeenCalledWith('copilot'))
    await waitFor(() => expect(screen.queryByText('WDML-MOCK')).toBeNull())
    expect(screen.getByText('not signed in')).toBeDefined()
  })
})
