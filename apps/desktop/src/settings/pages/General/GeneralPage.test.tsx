import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { screen, render, fireEvent, cleanup, waitFor } from '@testing-library/react'

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

describe('GeneralPage catalogue source', () => {
  let host: EngineHost
  beforeEach(() => {
    host = createMockEngineHost()
    resetHost(host)
  })

  it('defaults the data source repo to the main which-model repository', async () => {
    renderApp(host)
    const input = (await screen.findByPlaceholderText(
      'WD-Mitchell/which-model',
    )) as HTMLInputElement
    expect(input.value).toBe('WD-Mitchell/which-model')
    expect(screen.queryByPlaceholderText('ARTIFICIAL_ANALYSIS_API')).toBeNull()
  })

  it('shows the AA key field only when collect locally is on', async () => {
    renderApp(host)
    fireEvent.click(await screen.findByRole('switch', { name: 'Collect locally' }))
    await waitFor(() => {
      expect(screen.getByPlaceholderText('ARTIFICIAL_ANALYSIS_API')).toBeDefined()
    })
  })

  it('toggles only show and recommend enabled providers', async () => {
    renderApp(host)
    const toggle = await screen.findByRole('switch', {
      name: 'Only show and recommend enabled providers',
    })
    expect(toggle.getAttribute('aria-checked')).toBe('false')
    fireEvent.click(toggle)
    await waitFor(async () => {
      expect((await host.settings.get()).only_enabled_providers).toBe(true)
    })
  })
})
