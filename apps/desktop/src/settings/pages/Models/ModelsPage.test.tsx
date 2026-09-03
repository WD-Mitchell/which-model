import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { screen, render, fireEvent, cleanup, waitFor, within } from '@testing-library/react'

afterEach(() => cleanup())
import { createMockEngineHost } from '@which-model/core/mock'
import type { EngineHost } from '@which-model/core'
import { ToastProvider } from '@which-model/ui'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { resetHost } from '../../../lib/host'
import { MODELS_LIST_INITIAL, useModelsListStore } from './listState'
import { SettingsApp } from '../../SettingsApp'

// List control state is module-level now; reset it per test so cases stay
// isolated the way unmount-on-cleanup used to guarantee.
beforeEach(() => {
  useModelsListStore.setState({ ...MODELS_LIST_INITIAL })
})

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

async function openModels() {
  await screen.findByText('which-model — Settings')
  const nav = await screen.findAllByText('Models')
  const navBtn = nav.find((el) => el.tagName === 'BUTTON')
  fireEvent.click(navBtn!)
  await screen.findByText(
    'Every model in the catalog. Open one for its identity, reasoning levels, and catalog scores.',
  )
}

describe('Models page', () => {
  let host: EngineHost
  beforeEach(() => {
    host = createMockEngineHost()
    resetHost(host)
  })

  it('lists distinct catalog models from the mock host', async () => {
    renderApp(host)
    await openModels()
    const names = [
      'Claude Opus 5',
      'Claude Sonnet 5.2',
      'Gemini 3.5 Ultra',
      'GPT-5.6 Luna',
      'GPT-5.6 Sol',
      'Grok 5 Fast',
      'Llama 5 405B',
      'Qwen 3.5 Max',
    ]
    for (const name of names) {
      expect(await screen.findByText(name)).toBeDefined()
    }
    expect(screen.getAllByText('Claude Opus 5')).toHaveLength(1)
  })

  it('filters the list by name', async () => {
    renderApp(host)
    await openModels()
    fireEvent.change(screen.getByPlaceholderText('filter models'), { target: { value: 'opus' } })
    expect(await screen.findByText('Claude Opus 5')).toBeDefined()
    expect(screen.queryByText('GPT-5.6 Luna')).toBeNull()
  })

  it('shows an empty filter miss', async () => {
    renderApp(host)
    await openModels()
    fireEvent.change(screen.getByPlaceholderText('filter models'), { target: { value: 'zzzz' } })
    expect(await screen.findByText('no models match')).toBeDefined()
  })

  it('opens a summary detail and returns to the list', async () => {
    renderApp(host)
    await openModels()
    fireEvent.click(await screen.findByText('Claude Opus 5'))
    expect(await screen.findByText('catalog scores')).toBeDefined()
    expect(screen.getAllByText('claude-opus-5').length).toBeGreaterThan(0)
    expect(screen.getByText('enabled providers')).toBeDefined()
    expect(screen.getByText('$15 in / $75 out per 1M')).toBeDefined()
    fireEvent.click(screen.getByTitle('Back'))
    await waitFor(() => {
      expect(screen.getByPlaceholderText('filter models')).toBeDefined()
    })
    expect(screen.getByText('GPT-5.6 Luna')).toBeDefined()
  })

  it('drills into benchmark scores from a provider reasoning chip and returns', async () => {
    renderApp(host)
    await openModels()
    fireEvent.click(await screen.findByText('Claude Opus 5'))
    expect(await screen.findByText('enabled providers')).toBeDefined()
    const chips = screen.getAllByText('max')
    const providerChip = chips[chips.length - 1]!
    fireEvent.click(providerChip)
    expect(await screen.findByText('benchmarks')).toBeDefined()
    fireEvent.click(screen.getByTitle('Back'))
    expect(await screen.findByText('enabled providers')).toBeDefined()
  })

  it('filters the list by maker multi-select', async () => {
    renderApp(host)
    await openModels()

    const makerBtn = await screen.findByRole('button', { name: 'Filter by maker' })
    fireEvent.click(makerBtn)

    const anthropicOption = await screen.findByText('Anthropic')
    fireEvent.click(anthropicOption)

    expect(await screen.findByText('Claude Opus 5')).toBeDefined()
    expect(screen.getByText('Claude Sonnet 5.2')).toBeDefined()
    expect(screen.queryByText('GPT-5.6 Luna')).toBeNull()
    expect(screen.queryByText('Gemini 3.5 Ultra')).toBeNull()
  })

  it('filters the list by provider multi-select', async () => {
    renderApp(host)
    await openModels()

    const provBtn = await screen.findByRole('button', { name: 'Filter by provider' })
    fireEvent.click(provBtn)

    const codexOption = await screen.findByText('codex')
    fireEvent.click(codexOption)

    expect(await screen.findByText('GPT-5.6 Luna')).toBeDefined()
    expect(screen.getByText('GPT-5.6 Sol')).toBeDefined()
    expect(screen.queryByText('Claude Opus 5')).toBeNull()
  })

  it('composes maker and provider filters and allows clearing them', async () => {
    renderApp(host)
    await openModels()

    const makerBtn = await screen.findByRole('button', { name: 'Filter by maker' })
    fireEvent.click(makerBtn)
    fireEvent.click(await screen.findByText('OpenAI'))

    const provBtn = await screen.findByRole('button', { name: 'Filter by provider' })
    fireEvent.click(provBtn)
    fireEvent.click(await screen.findByText('copilot'))

    // Both GPT-5.6 Luna and GPT-5.6 Sol are OpenAI models available on copilot
    expect(await screen.findByText('GPT-5.6 Luna')).toBeDefined()
    expect(screen.getByText('GPT-5.6 Sol')).toBeDefined()
    expect(screen.queryByText('Claude Opus 5')).toBeNull()

    // Clear all filters
    const clearBtn = await screen.findByRole('button', { name: 'Clear all' })
    fireEvent.click(clearBtn)

    expect(await screen.findByText('Claude Opus 5')).toBeDefined()
    expect(screen.getByText('GPT-5.6 Luna')).toBeDefined()
  })

  it('keeps search and filters across a detail round-trip', async () => {
    renderApp(host)
    await openModels()
    fireEvent.change(screen.getByPlaceholderText('filter models'), {
      target: { value: 'GPT' },
    })

    fireEvent.click(await screen.findByText('GPT-5.6 Luna'))
    expect(await screen.findByText('catalog scores')).toBeDefined()
    fireEvent.click(screen.getByTitle('Back'))
    await screen.findByPlaceholderText('filter models')

    expect((screen.getByPlaceholderText('filter models') as HTMLInputElement).value).toBe('GPT')
    expect(screen.queryByText('Claude Opus 5')).toBeNull()
    expect(screen.getByText('GPT-5.6 Luna')).toBeDefined()

    // A provider multi-select survives the same way.
    fireEvent.click(screen.getByRole('button', { name: 'Filter by provider' }))
    fireEvent.click(await screen.findByText('copilot'))
    fireEvent.click(await screen.findByText('GPT-5.6 Luna'))
    expect(await screen.findByText('catalog scores')).toBeDefined()
    fireEvent.click(screen.getByTitle('Back'))
    await screen.findByPlaceholderText('filter models')

    expect(
      screen.getByRole('button', { name: 'Filter by provider' }).textContent,
    ).toContain('(1)')
    expect(screen.getByText('GPT-5.6 Sol')).toBeDefined()
  })
})

describe('Models nav', () => {
  it('places Models in the ranking group of the sidebar', async () => {
    const host = createMockEngineHost()
    resetHost(host)
    renderApp(host)
    const nav = await screen.findByLabelText('Settings sections')
    expect(within(nav).getByRole('button', { name: 'Models' })).toBeDefined()
  })
})
