import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { screen, render, fireEvent, cleanup, waitFor } from '@testing-library/react'

afterEach(() => cleanup())
import { createMockEngineHost } from '@which-model/core/mock'
import type { EngineHost } from '@which-model/core'
import { ToastProvider } from '@which-model/ui'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { resetHost } from '../../../lib/host'
import { SettingsApp } from '../../SettingsApp'
import { useEngineEvents } from '../../../lib/invalidate'

function makeClient() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } })
}

function renderApp(host: EngineHost) {
  const client = makeClient()
  function Root() {
    useEngineEvents()
    return <SettingsApp host={host} />
  }
  render(
    <QueryClientProvider client={client}>
      <ToastProvider>
        <Root />
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
    // Render comes straight from the ['settings'] cache (U12 §3: no local
    // optimistic state), so wait for each write's settings:changed refetch
    // to re-render before the next write spreads the refreshed struct.
    fireEvent.change(shortcut, { target: { value: 'ctrl+space' } })
    await waitFor(() => {
      expect((shortcut as HTMLSelectElement).value).toBe('ctrl+space')
    })
    fireEvent.change(weightControl, { target: { value: 'bar' } })
    await waitFor(() => {
      expect((weightControl as HTMLSelectElement).value).toBe('bar')
    })
    fireEvent.change(holds, { target: { value: '5' } })

    await waitFor(async () => {
      const settings = await host.settings.get()
      expect(settings.shortcut).toBe('ctrl+space')
      expect(settings.default_tab).toBe('profiles')
      expect(settings.weight_control).toBe('bar')
      expect(settings.holds).toBe(5)
    })
  })

  // Issue #41 regression: after a user interaction, the page must still
  // re-render from refetched settings when the engine reports
  // settings:changed (external edit — CLI, config file, another window).
  // The removed draft state pinned the first render forever.
  it('re-renders external settings changes after a prior user interaction', async () => {
    renderApp(host)
    const holds = await screen.findByRole('combobox', { name: 'Ranks held per pick' })
    fireEvent.change(holds, { target: { value: '3' } })
    await waitFor(() => {
      expect((holds as HTMLSelectElement).value).toBe('3')
    })

    // External write: settings.set directly on the host (any surface — CLI,
    // config file watcher) emits settings:changed, which invalidates
    // ['settings'] and refetches.
    const current = await host.settings.get()
    await host.settings.set({ ...current, layout: 'list', holds: 5 })

    await waitFor(() => {
      expect((holds as HTMLSelectElement).value).toBe('5')
    })
    const layout = screen.getByRole('radio', { name: 'list' })
    expect(layout.getAttribute('aria-checked')).toBe('true')
    // The user's earlier write must survive (no stale-draft overwrite).
    expect((await host.settings.get()).weight_control).toBe('slider')

    // Subsequent user interaction builds on the fresh external state,
    // not overwriting it with stale pre-edit data.
    const toggle = screen.getByRole('switch', { name: 'Show menu bar icon' })
    fireEvent.click(toggle)
    await waitFor(async () => {
      const updated = await host.settings.get()
      expect(updated.show_menu_bar_icon).toBe(false)
      expect(updated.holds).toBe(5)
      expect(updated.layout).toBe('list')
    })
  })
  it('persists source selections without requiring a text-field blur', async () => {
    renderApp(host)
    const select = await screen.findByRole('combobox', { name: 'Data source' })
    fireEvent.change(select, { target: { value: 'local' } })
    await waitFor(async () => expect((await host.settings.get()).use_local_aa).toBe(true))
    fireEvent.change(select, { target: { value: 'official' } })
    await waitFor(async () => {
      expect((await host.settings.get()).use_local_aa).toBe(false)
      expect((await host.settings.get()).catalog_repo).toBe('WD-Mitchell/which-model')
    })
  })

  it('serializes rapid changes against the last committed snapshot and recovers after failure', async () => {
    const original = host.settings.set.bind(host.settings)
    let release!: () => void
    const gate = new Promise<void>(resolve => { release = resolve })
    const write = vi.spyOn(host.settings, 'set').mockImplementationOnce(async value => {
      await gate
      await original(value)
    })
    renderApp(host)
    const holds = await screen.findByRole('combobox', { name: 'Ranks held per pick' })
    fireEvent.change(holds, { target: { value: '5' } })
    await waitFor(() => expect(write).toHaveBeenCalledTimes(1))
    fireEvent.change(screen.getByRole('combobox', { name: 'Weight control' }), { target: { value: 'bar' } })
    expect(write).toHaveBeenCalledTimes(1)
    release()
    await waitFor(async () => {
      expect((await host.settings.get()).holds).toBe(5)
      expect((await host.settings.get()).weight_control).toBe('bar')
    })
    write.mockRejectedValueOnce(new Error('save rejected'))
    fireEvent.change(holds, { target: { value: '3' } })
    await screen.findByText('save rejected')
    fireEvent.change(holds, { target: { value: '1' } })
    await waitFor(async () => expect((await host.settings.get()).holds).toBe(1))
  })

})
