// Copilot OAuth sign-in in the Providers page: Sign in… copies the code,
// opens GitHub, and starts polling immediately. Cancel aborts. Success
// marks the account signed in. Drives the real page through SettingsApp.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { screen, render, act, fireEvent, cleanup, waitFor, within } from '@testing-library/react'

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

async function openProvidersList(host: EngineHost) {
  renderApp(host)
  await screen.findByText('which-model — Settings')
  const nav = await screen.findAllByText('Providers')
  const navBtn = nav.find((el) => el.tagName === 'BUTTON')
  act(() => navBtn!.dispatchEvent(new MouseEvent('click', { bubbles: true })))
  await screen.findByLabelText('Search providers')
}

function providerOrder(): string[] {
  return Array.from(document.querySelectorAll<HTMLElement>('[data-provider-id]')).map(
    (row) => row.dataset.providerId!,
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
  await screen.findByText('Accounts')
}

function deferConfirm(host: EngineHost) {
  let resolve!: () => void
  let reject!: (e: unknown) => void
  const pending = new Promise<void>((res, rej) => {
    resolve = res
    reject = rej
  })
  const confirm = host.signin.confirm.bind(host.signin)
  const confirmSpy = vi.spyOn(host.signin, 'confirm').mockImplementation(async (provider, flowId, name) => {
    await pending
    await confirm(provider, flowId, name)
  })
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
    await waitFor(() =>
      expect(confirmSpy).toHaveBeenCalledWith('copilot', expect.any(String), 'GitHub'),
    )
    expect(screen.queryByText('I entered the code')).toBeNull()
  })

  it('confirm resolves the modal and shows signed in', async () => {
    const { confirmSpy, resolve } = deferConfirm(host)
    await openCopilotDetail(host)
    fireEvent.click(await screen.findByText('Sign in…'))
    await screen.findByText('WDML-MOCK')
    await waitFor(() =>
      expect(confirmSpy).toHaveBeenCalledWith('copilot', expect.any(String), 'GitHub'),
    )
    resolve()
    await waitFor(() => expect(screen.queryByText('WDML-MOCK')).toBeNull())
    expect(await screen.findByText(/OAuth · signed in/)).toBeDefined()
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
    await waitFor(() =>
      expect(cancelSpy).toHaveBeenCalledWith('copilot', expect.any(String)),
    )
    await waitFor(() => expect(screen.queryByText('WDML-MOCK')).toBeNull())
    expect(screen.getByText(/OAuth · not signed in/)).toBeDefined()
  })

  it('does not tell signed-out users to run the CLI', async () => {
    host = createMockEngineHost({ models: [] }) as EngineHost & { data: never }
    resetHost(host)
    await openCopilotDetail(host)
    expect(screen.queryByText(/which-model routes refresh/)).toBeNull()
    expect(
      screen.queryByText('No models for this provider yet. Sign in, then refresh models.'),
    ).toBeNull()
    expect(await screen.findByText('GPT-5.6 Luna')).toBeDefined()
  })

  it('shows Refresh models after sign-in and the button rebuilds routes', async () => {
    const { confirmSpy, resolve } = deferConfirm(host)
    const refreshSpy = vi.spyOn(host.providers, 'refreshRoutes').mockResolvedValue(undefined)
    await openCopilotDetail(host)
    expect(screen.queryByText('Refresh models')).toBeNull()
    fireEvent.click(await screen.findByText('Sign in…'))
    await screen.findByText('WDML-MOCK')
    await waitFor(() =>
      expect(confirmSpy).toHaveBeenCalledWith('copilot', expect.any(String), 'GitHub'),
    )
    resolve()
    await waitFor(() => expect(screen.queryByText('WDML-MOCK')).toBeNull())
    fireEvent.click(await screen.findByText('Refresh models'))
    await waitFor(() => expect(refreshSpy).toHaveBeenCalledTimes(1))
  })
})


async function openProviderDetail(host: EngineHost, id: string) {
  renderApp(host)
  await screen.findByText('which-model — Settings')
  const nav = await screen.findAllByText('Providers')
  const navBtn = nav.find((el) => el.tagName === 'BUTTON')
  act(() => navBtn!.dispatchEvent(new MouseEvent('click', { bubbles: true })))
  const card = await screen.findByText((_, el) => el?.tagName === 'SPAN' && el.textContent === id && el.className.includes('id'))
  act(() => card.dispatchEvent(new MouseEvent('click', { bubbles: true })))
  await screen.findByText('Accounts')
}

describe('Providers page claude and codex sign-in', () => {
  let host: EngineHost
  beforeEach(() => {
    host = createMockEngineHost()
    resetHost(host)
  })

  it('codex uses the device-code modal and opens auth.openai.com', async () => {
    const { confirmSpy } = deferConfirm(host)
    const openSpy = vi.spyOn(host.window, 'openURL').mockResolvedValue(undefined)
    const copySpy = vi.spyOn(host.window, 'copyToClipboard').mockResolvedValue(undefined)
    await openProviderDetail(host, 'codex')
    fireEvent.click(await screen.findByText('Sign in…'))
    expect(await screen.findByText('WDML-MOCK')).toBeDefined()
    await waitFor(() => expect(copySpy).toHaveBeenCalledWith('WDML-MOCK'))
    await waitFor(() =>
      expect(openSpy).toHaveBeenCalledWith('https://auth.openai.com/codex/device?user_code=WDML-MOCK'),
    )
    await waitFor(() =>
      expect(confirmSpy).toHaveBeenCalledWith('codex', expect.any(String), 'ChatGPT'),
    )
    expect(screen.getByText('Open auth.openai.com')).toBeDefined()
  })

  it('claude opens the authorize URL, shows paste, and submitCode continues', async () => {
    const { confirmSpy } = deferConfirm(host)
    const openSpy = vi.spyOn(host.window, 'openURL').mockResolvedValue(undefined)
    const copySpy = vi.spyOn(host.window, 'copyToClipboard').mockResolvedValue(undefined)
    const submitSpy = vi.spyOn(host.signin, 'submitCode').mockResolvedValue(undefined)
    await openProviderDetail(host, 'claude')
    fireEvent.click(await screen.findByText('Re-authenticate'))
    await waitFor(() => expect(openSpy).toHaveBeenCalledWith('https://claude.ai/oauth/authorize'))
    expect(copySpy).not.toHaveBeenCalled()
    expect(screen.queryByText('WDML-MOCK')).toBeNull()
    await waitFor(() =>
      expect(confirmSpy).toHaveBeenCalledWith('claude', expect.any(String), 'Work'),
    )
    const paste = screen.getByPlaceholderText('Paste the code from the page')
    fireEvent.change(paste, { target: { value: 'abc#state' } })
    fireEvent.click(await screen.findByText('Continue'))
    await waitFor(() =>
      expect(submitSpy).toHaveBeenCalledWith('claude', expect.any(String), 'abc#state'),
    )
  })
})

describe('Providers page model levels and benchmarks', () => {
  let host: EngineHost
  beforeEach(() => {
    host = createMockEngineHost()
    resetHost(host)
  })

  it('lists reasoning levels and opens that combo’s benchmarks on click', async () => {
    await openProviderDetail(host, 'claude')
    expect(screen.queryByText('no routes')).toBeNull()
    expect(await screen.findByText('Claude Haiku 4')).toBeDefined()
    const opus = (await screen.findByText('Claude Opus 5')).closest('div')
    expect(opus).not.toBeNull()
    fireEvent.click(within(opus!).getByRole('button', { name: 'max' }))
    expect(await screen.findByRole('heading', { name: /Claude Opus 5.*\(max\)/ })).toBeDefined()
    expect(await screen.findByText('SWE-Bench Verified')).toBeDefined()
    fireEvent.click(await screen.findByText('claude'))
    expect(await screen.findByText('Accounts')).toBeDefined()
    const haiku = (await screen.findByText('Claude Haiku 4')).closest('div')
    fireEvent.click(within(haiku!).getByRole('button', { name: 'low' }))
    expect(await screen.findByRole('heading', { name: /Claude Haiku 4.*\(low\)/ })).toBeDefined()
    expect(await screen.findByText('No benchmarks for this model and reasoning level yet.')).toBeDefined()
  })

  it('opens shared model summary when model name is clicked', async () => {
    await openProviderDetail(host, 'claude')
    fireEvent.click(await screen.findByText('Claude Opus 5'))
    expect(await screen.findByText('catalog scores')).toBeDefined()
    expect(screen.getByText('enabled providers')).toBeDefined()
    expect(screen.getByText('$15 in / $75 out per 1M')).toBeDefined()
    fireEvent.click(screen.getByTitle('Back'))
    expect(await screen.findByText('Accounts')).toBeDefined()
  })
})

describe('Providers page custom provider adding', () => {
  let host: EngineHost
  beforeEach(() => {
    host = createMockEngineHost()
    resetHost(host)
  })
  it('adds a custom provider via the Add provider input', async () => {
    const addSpy = vi.spyOn(host.providers, 'add')
    renderApp(host)
    await screen.findByText('which-model — Settings')
    const nav = await screen.findAllByText('Providers')
    const navBtn = nav.find((el) => el.tagName === 'BUTTON')
    act(() => navBtn!.dispatchEvent(new MouseEvent('click', { bubbles: true })))
    await screen.findByText((_, el) => el?.tagName === 'SPAN' && el.textContent === 'copilot' && el.className.includes('id'))

    const addBtn = await screen.findByRole('button', { name: 'Add provider' })
    act(() => addBtn.dispatchEvent(new MouseEvent('click', { bubbles: true })))

    const input = await screen.findByPlaceholderText('custom-provider-id')
    fireEvent.change(input, { target: { value: 'custom_vllm' } })

    const submitBtn = await screen.findByRole('button', { name: 'Add' })
    act(() => submitBtn.dispatchEvent(new MouseEvent('click', { bubbles: true })))

    await waitFor(() => {
      expect(addSpy).toHaveBeenCalledWith('custom_vllm')
    })
  })
})

describe('Providers page list controls', () => {
  let host: EngineHost

  beforeEach(async () => {
    host = createMockEngineHost()
    resetHost(host)
    await host.providers.reorder([
      'xai',
      'mistral',
      'google',
      'commandcode',
      'antigravity',
      'cursor',
      'copilot',
      'codex',
      'claude',
    ])
  })

  it('defaults to enabled providers first and filters by a case-insensitive search', async () => {
    await openProvidersList(host)
    expect(providerOrder()).toEqual([
      'claude',
      'codex',
      'copilot',
      'antigravity',
      'commandcode',
      'cursor',
      'google',
      'mistral',
      'xai',
    ])

    fireEvent.change(screen.getByLabelText('Search providers'), {
      target: { value: 'PIL' },
    })
    expect(providerOrder()).toEqual(['copilot'])
  })

  it('filters providers by enabled state with a select', async () => {
    await openProvidersList(host)

    const filter = screen.getByRole('combobox', { name: 'Filter providers' })
    fireEvent.change(filter, { target: { value: 'disabled' } })
    expect(providerOrder()).toEqual([
      'antigravity',
      'commandcode',
      'cursor',
      'google',
      'mistral',
      'xai',
    ])

    fireEvent.change(filter, { target: { value: 'enabled' } })
    expect(providerOrder()).toEqual(['claude', 'codex', 'copilot'])

    fireEvent.change(filter, { target: { value: 'all' } })
    expect(providerOrder()).toHaveLength(9)
  })

  it('sorts provider names in both directions', async () => {
    await openProvidersList(host)
    fireEvent.change(screen.getByLabelText('Sort providers'), {
      target: { value: 'name-desc' },
    })
    expect(providerOrder()).toEqual([
      'xai',
      'mistral',
      'google',
      'cursor',
      'copilot',
      'commandcode',
      'codex',
      'claude',
      'antigravity',
    ])

    fireEvent.change(screen.getByLabelText('Sort providers'), {
      target: { value: 'name-asc' },
    })
    expect(providerOrder()[0]).toBe('antigravity')
  })

  it('sorts distinct model counts high-to-low and low-to-high', async () => {
    await openProvidersList(host)
    fireEvent.change(screen.getByLabelText('Sort providers'), {
      target: { value: 'models-desc' },
    })
    expect(providerOrder()).toEqual([
      'copilot',
      'cursor',
      'claude',
      'codex',
      'antigravity',
      'commandcode',
      'google',
      'mistral',
      'xai',
    ])

    fireEvent.change(screen.getByLabelText('Sort providers'), {
      target: { value: 'models-asc' },
    })
    expect(providerOrder()).toEqual([
      'antigravity',
      'commandcode',
      'google',
      'mistral',
      'xai',
      'claude',
      'codex',
      'cursor',
      'copilot',
    ])
  })

  it('sorts enabled and disabled providers in both directions', async () => {
    await openProvidersList(host)
    fireEvent.change(screen.getByLabelText('Sort providers'), {
      target: { value: 'disabled-first' },
    })
    expect(providerOrder()).toEqual([
      'antigravity',
      'commandcode',
      'cursor',
      'google',
      'mistral',
      'xai',
      'claude',
      'codex',
      'copilot',
    ])

    fireEvent.change(screen.getByLabelText('Sort providers'), {
      target: { value: 'enabled-first' },
    })
    expect(providerOrder()).toEqual([
      'claude',
      'codex',
      'copilot',
      'antigravity',
      'commandcode',
      'cursor',
      'google',
      'mistral',
      'xai',
    ])
  })

  it('keeps priority order available for drag-based fallback editing', async () => {
    await openProvidersList(host)
    fireEvent.change(screen.getByLabelText('Sort providers'), {
      target: { value: 'priority' },
    })
    expect(providerOrder()).toEqual([
      'xai',
      'mistral',
      'google',
      'commandcode',
      'antigravity',
      'cursor',
      'copilot',
      'codex',
      'claude',
    ])
    expect(screen.getByText('providers · drag to set fallback order')).toBeDefined()
  })
})

describe('Providers page account setup', () => {
  let host: EngineHost

  beforeEach(() => {
    host = createMockEngineHost()
    resetHost(host)
  })

  it('does not offer OAuth for a provider without a supported flow', async () => {
    await openProviderDetail(host, 'google')
    fireEvent.click(await screen.findByRole('button', { name: 'Add account' }))

    const method = screen.getByRole('combobox', {
      name: 'Authentication method',
    }) as HTMLSelectElement
    expect(Array.from(method.options).map((option) => option.value)).toEqual(['api_key'])
    expect(screen.queryByRole('option', { name: 'OAuth' })).toBeNull()
  })

  it('names a new Cursor OAuth account before starting sign-in', async () => {
    await host.providers.setAccounts('cursor', [])
    const { confirmSpy } = deferConfirm(host)
    const startSpy = vi.spyOn(host.signin, 'start')
    await openProviderDetail(host, 'cursor')
    fireEvent.click(await screen.findByRole('button', { name: 'Add account' }))

    const method = screen.getByRole('combobox', {
      name: 'Authentication method',
    }) as HTMLSelectElement
    expect(Array.from(method.options).map((option) => option.value)).toEqual([
      'oauth',
      'api_key',
    ])
    fireEvent.change(screen.getByLabelText('Account name'), {
      target: { value: 'Team' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Sign in with OAuth' }))

    await waitFor(() => expect(startSpy).toHaveBeenCalledWith('cursor'))
    await waitFor(() => expect(confirmSpy).toHaveBeenCalledWith('cursor', expect.any(String), 'Team'))
    expect(
      await screen.findByText(
        'A browser window opened. This closes automatically after authorization.',
      ),
    ).toBeDefined()
    expect(screen.queryByPlaceholderText('Paste the code from the page')).toBeNull()
    expect(screen.queryByRole('button', { name: 'Continue' })).toBeNull()
  })

  it('accepts an API key and never reflects it through provider data', async () => {
    const saveSpy = vi.spyOn(host.signin, 'saveAPIKey')
    await openProviderDetail(host, 'google')
    fireEvent.click(await screen.findByRole('button', { name: 'Add account' }))
    fireEvent.change(screen.getByLabelText('Account name'), {
      target: { value: 'Production' },
    })
    fireEvent.change(screen.getByLabelText('API key'), {
      target: { value: 'sk-ui-canary-123' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save account' }))

    await waitFor(() =>
      expect(saveSpy).toHaveBeenCalledWith('google', 'Production', 'sk-ui-canary-123'),
    )
    await waitFor(() => expect(screen.queryByDisplayValue('sk-ui-canary-123')).toBeNull())
    const detail = await host.providers.detail('google')
    expect(detail.accounts).toEqual([
      { name: 'Production', kind: 'token', ref: 'which-model' },
    ])
    expect(JSON.stringify(detail)).not.toContain('sk-ui-canary-123')
  })
})
