import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { screen, render, fireEvent, cleanup, waitFor, within } from '@testing-library/react'

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
    expect(screen.getByText('claude-opus-5')).toBeDefined()
    fireEvent.click(screen.getByTitle('Back'))
    await waitFor(() => {
      expect(screen.getByPlaceholderText('filter models')).toBeDefined()
    })
    expect(screen.getByText('GPT-5.6 Luna')).toBeDefined()
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
