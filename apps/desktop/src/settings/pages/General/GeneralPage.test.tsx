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

describe('GeneralPage benchmarks data source', () => {
  let host: EngineHost
  beforeEach(() => {
    host = createMockEngineHost()
    resetHost(host)
  })

  it('defaults the data source to Official (hiding custom repo and API key inputs)', async () => {
    renderApp(host)
    const select = (await screen.findByRole('combobox', { name: 'Data source' })) as HTMLSelectElement
    expect(select.value).toBe('official')
    expect(screen.queryByPlaceholderText('owner/repo')).toBeNull()
    expect(screen.queryByPlaceholderText('ARTIFICIAL_ANALYSIS_API')).toBeNull()
  })

  it('shows the self-hosted repo field when Self-Hosted is selected', async () => {
    renderApp(host)
    const select = await screen.findByRole('combobox', { name: 'Data source' })
    fireEvent.change(select, { target: { value: 'self-hosted' } })
    await waitFor(() => {
      expect(screen.getByPlaceholderText('owner/repo')).toBeDefined()
    })
  })

  it('shows the AA key field when Local Only is selected', async () => {
    renderApp(host)
    const select = await screen.findByRole('combobox', { name: 'Data source' })
    fireEvent.change(select, { target: { value: 'local' } })
    await waitFor(() => {
      expect(screen.getByPlaceholderText('ARTIFICIAL_ANALYSIS_API')).toBeDefined()
    })
  })

  it('defaults the benchmark check frequency to 6 hours and updates on change', async () => {
    renderApp(host)
    const select = (await screen.findByRole('combobox', {
      name: 'Check for new benchmarks',
    })) as HTMLSelectElement
    expect(select.value).toBe('6h')
    fireEvent.change(select, { target: { value: '1h' } })
    await waitFor(async () => {
      expect((await host.settings.get()).benchmark_check_frequency).toBe('1h')
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

  it('uses selects for compact multi-option settings', async () => {
    renderApp(host)
    const shortcut = await screen.findByRole('combobox', { name: 'Open the popover' })
    const weightControl = screen.getByRole('combobox', { name: 'Weight control' })
    const holds = screen.getByRole('combobox', { name: 'Ranks held per pick' })

    fireEvent.change(shortcut, { target: { value: 'ctrl+space' } })
    fireEvent.change(weightControl, { target: { value: 'bar' } })
    fireEvent.change(holds, { target: { value: '5' } })

    await waitFor(async () => {
      const settings = await host.settings.get()
      expect(settings.shortcut).toBe('ctrl+space')
      expect(settings.default_tab).toBe('profiles')
      expect(settings.weight_control).toBe('bar')
      expect(settings.holds).toBe(5)
    })
  })
})
