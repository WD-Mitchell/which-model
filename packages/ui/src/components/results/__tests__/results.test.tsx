import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { RankedModel } from '@which-model/core'
import { RankCard } from '../RankCard'
import { ResultsCarousel } from '../ResultsCarousel'
import { ResultsTable } from '../ResultsTable'
import { ModelRow } from '../ModelRow'
import { Sparkbar, sparkbarHeight } from '../Sparkbar'
import type { SparkbarEntry } from '../Sparkbar'
import { RouteKeyChip } from '../RouteKeyChip'
import { EmptyCandidatesState } from '../EmptyCandidatesState'
import sparkStyles from '../Sparkbar.module.css'
import rankStyles from '../RankCard.module.css'
import rowStyles from '../ModelRow.module.css'

const models: RankedModel[] = [
  { rank: 1, model_id: 'gpt-5.6-luna', model_name: 'GPT-5.6 Luna', provider: 'codex', reasoning: 'max', score: 92.41, route_key: 'codex/gpt-5.6-luna@max' },
  { rank: 2, model_id: 'claude-opus', model_name: 'Claude Opus', provider: 'claude', reasoning: 'high', score: 88.1, route_key: 'claude/claude-opus@high' },
  { rank: 3, model_id: 'gpt-4-mini', model_name: 'GPT-4 Mini', provider: 'codex', reasoning: 'medium', score: 90, route_key: 'codex/gpt-4-mini@medium' },
]

const coreMetrics: SparkbarEntry[] = [
  { key: 'intelligence', value: 5 },
  { key: 'speed', value: 3 },
  { key: 'cost', value: 1 },
]

function barHeights(container: HTMLElement): number[] {
  return Array.from(container.querySelectorAll(`.${sparkStyles.bar}`)).map(
    (el) => Number((el as HTMLElement).style.height.replace('px', '')),
  )
}

describe('Sparkbar', () => {
  it('scales bar heights 1→8, 3→16, 5→24 and clamps out-of-range values', () => {
    expect(sparkbarHeight(1)).toBe(8)
    expect(sparkbarHeight(3)).toBe(16)
    expect(sparkbarHeight(5)).toBe(24)
    // clamping for height purposes only
    expect(sparkbarHeight(9)).toBe(24)
    expect(sparkbarHeight(-2)).toBe(4)
  })

  it('renders bars with the scaled inline height', () => {
    const { container } = render(
      <Sparkbar
        metrics={[
          { key: 'intelligence', value: 1 },
          { key: 'speed', value: 3 },
          { key: 'cost', value: 5 },
        ]}
      />,
    )
    expect(barHeights(container)).toEqual([8, 16, 24])
  })

  it('exposes the tooltip text as each bar hit area aria-label', () => {
    render(<Sparkbar metrics={[{ key: 'cost', value: 3 }]} />)
    // testing-library normalises whitespace for the accessible name, so the
    // two-space `{key}  {value} / 5` string matches with a single space here.
    expect(screen.getByRole('img', { name: 'cost 3 / 5' })).toBeTruthy()
  })

  it('shows a tooltip on hover with `{key}  {value} / 5` and hides on leave', () => {
    render(<Sparkbar metrics={[{ key: 'cost', value: 3 }]} />)
    const bar = screen.getByRole('img', { name: 'cost 3 / 5' })
    expect(screen.queryByText('cost 3 / 5')).toBeNull()
    fireEvent.mouseEnter(bar)
    expect(screen.getByText('cost 3 / 5')).toBeTruthy()
    fireEvent.mouseLeave(bar)
    expect(screen.queryByText('cost 3 / 5')).toBeNull()
  })

  it('renders nothing throw-free for empty metrics', () => {
    const { container } = render(<Sparkbar metrics={[]} />)
    expect(container.querySelector(`.${sparkStyles.bar}`)).toBeNull()
  })
})

describe('RouteKeyChip', () => {
  it('renders provider and model@reasoning portions of the route key', () => {
    render(<RouteKeyChip routeKey="codex/gpt-5.6-luna@max" />)
    expect(screen.getByText('codex')).toBeTruthy()
    expect(screen.getByText('/gpt-5.6-luna@max')).toBeTruthy()
    expect(screen.getByTitle('codex/gpt-5.6-luna@max')).toBeTruthy()
  })
})

describe('RankCard', () => {
  function launchProps(overrides: Record<string, unknown> = {}) {
    return {
      launchLabel: 'Launch in Claude Code',
      harnesses: [
        { key: 'claude', label: 'Claude Code', selected: true },
        { key: 'codex', label: 'Codex', selected: false },
      ],
      launchMenuOpen: false,
      onLaunchMenuOpenChange: vi.fn(),
      onLaunch: vi.fn(),
      onHarnessChange: vi.fn(),
      ...overrides,
    }
  }

  it('renders rank line, name, meta and score badge with 2dp formatting', () => {
    render(
      <RankCard
        model={models[0]}
        rankLine="rank 1 of 3"
        metrics={coreMetrics}
      />,
    )
    expect(screen.getByText('rank 1 of 3')).toBeTruthy()
    expect(screen.getByText('GPT-5.6 Luna')).toBeTruthy()
    expect(screen.getByText('codex · max · 92.41')).toBeTruthy()
    expect(screen.getByText('92.41')).toBeTruthy()
  })

  it('formats an integer score as 90.00', () => {
    const m = { ...models[0], score: 90 }
    render(<RankCard model={m} rankLine="rank 1 of 1" metrics={coreMetrics} />)
    expect(screen.getByText('90.00')).toBeTruthy()
  })

  it('renders the route chip and sparkbars', () => {
    const { container } = render(
      <RankCard
        model={models[0]}
        rankLine="rank 1 of 3"
        metrics={coreMetrics}
      />,
    )
    expect(screen.getByText('/gpt-5.6-luna@max')).toBeTruthy()
    expect(container.querySelectorAll(`.${sparkStyles.bar}`).length).toBe(3)
  })

  it('launches the selected harness from the split-button main segment', () => {
    const p = launchProps()
    render(
      <RankCard
        model={models[0]}
        rankLine="rank 1 of 3"
        metrics={coreMetrics}
        {...p}
      />,
    )
    fireEvent.click(screen.getByText('Launch in Claude Code'))
    expect(p.onLaunch).toHaveBeenCalledTimes(1)
    expect(p.onHarnessChange).not.toHaveBeenCalled()
  })

  it('reports a harness change from the split-button menu', () => {
    const p = launchProps({ launchMenuOpen: true })
    render(
      <RankCard
        model={models[0]}
        rankLine="rank 1 of 3"
        metrics={coreMetrics}
        {...p}
      />,
    )
    fireEvent.click(screen.getByText('Codex'))
    expect(p.onHarnessChange).toHaveBeenCalledWith('codex')
    expect(p.onLaunch).not.toHaveBeenCalled()
  })

  it('toggles the pin/favourite with aria-pressed reflecting state', () => {
    const onTogglePin = vi.fn()
    const { rerender } = render(
      <RankCard
        model={models[0]}
        rankLine="rank 1 of 3"
        metrics={coreMetrics}
        pinned={false}
        onTogglePin={onTogglePin}
      />,
    )
    const btn = screen.getByRole('button', { name: 'pin model' })
    expect(btn.getAttribute('aria-pressed')).toBe('false')
    fireEvent.click(btn)
    expect(onTogglePin).toHaveBeenCalledTimes(1)
    rerender(
      <RankCard
        model={models[0]}
        rankLine="rank 1 of 3"
        metrics={coreMetrics}
        pinned={true}
        onTogglePin={onTogglePin}
      />,
    )
    expect(screen.getByRole('button', { name: 'unpin model' }).getAttribute('aria-pressed')).toBe('true')
  })

  it('renders no pin button without onTogglePin', () => {
    render(<RankCard model={models[0]} rankLine="rank 1 of 3" metrics={coreMetrics} />)
    expect(screen.queryByRole('button', { name: 'pin model' })).toBeNull()
  })
})

describe('ResultsCarousel', () => {
  const rankLabel = (i: number, total: number) => `rank ${i + 1} of ${total}`
  const metricsFor = () => coreMetrics

  it('renders one card per item with correct rank lines and a focused card', () => {
    render(
      <ResultsCarousel
        items={models}
        metrics={metricsFor}
        index={1}
        onIndex={vi.fn()}
        rankLabel={rankLabel}
      />,
    )
    expect(screen.getAllByRole('article').length).toBe(3)
    expect(screen.getByText('rank 1 of 3')).toBeTruthy()
    expect(screen.getByText('rank 2 of 3')).toBeTruthy()
    expect(screen.getByText('rank 3 of 3')).toBeTruthy()
    const cards = screen.getAllByRole('article')
    expect(cards[1].className).toContain(rankStyles.focused)
    expect(cards[0].className).not.toContain(rankStyles.focused)
  })

  it('renders pagination indicators with the focused dot marked', () => {
    render(
      <ResultsCarousel
        items={models}
        metrics={metricsFor}
        index={1}
        onIndex={vi.fn()}
        rankLabel={rankLabel}
      />,
    )
    const dots = screen.getAllByRole('button', { name: /go to rank/ })
    expect(dots.length).toBe(3)
    expect(dots[1].getAttribute('aria-current')).toBe('true')
    expect(dots[0].getAttribute('aria-current')).toBeNull()
  })

  it('sends prev/next onIndex and disables chevrons at the bounds', () => {
    const onIndex = vi.fn()
    const { rerender } = render(
      <ResultsCarousel
        items={models}
        metrics={metricsFor}
        index={0}
        onIndex={onIndex}
        rankLabel={rankLabel}
      />,
    )
    const prev = screen.getByRole('button', { name: 'previous rank' })
    const next = screen.getByRole('button', { name: 'next rank' })
    expect((prev as HTMLButtonElement).disabled).toBe(true)
    fireEvent.click(prev)
    expect(onIndex).not.toHaveBeenCalled()
    fireEvent.click(next)
    expect(onIndex).toHaveBeenCalledWith(1)
    onIndex.mockClear()

    rerender(
      <ResultsCarousel
        items={models}
        metrics={metricsFor}
        index={2}
        onIndex={onIndex}
        rankLabel={rankLabel}
      />,
    )
    const prev2 = screen.getByRole('button', { name: 'previous rank' })
    const next2 = screen.getByRole('button', { name: 'next rank' })
    expect((next2 as HTMLButtonElement).disabled).toBe(true)
    fireEvent.click(prev2)
    expect(onIndex).toHaveBeenCalledWith(1)
    fireEvent.click(next2)
    expect(onIndex).toHaveBeenCalledTimes(1)
  })

  it('clamps an out-of-range index for display', () => {
    render(
      <ResultsCarousel
        items={models}
        metrics={metricsFor}
        index={9}
        onIndex={vi.fn()}
        rankLabel={rankLabel}
      />,
    )
    const cards = screen.getAllByRole('article')
    expect(cards[2].className).toContain(rankStyles.focused)
    expect((screen.getByRole('button', { name: 'next rank' }) as HTMLButtonElement).disabled).toBe(true)
  })

  it('disables both chevrons with no candidates', () => {
    render(
      <ResultsCarousel
        items={[]}
        metrics={metricsFor}
        index={0}
        onIndex={vi.fn()}
        rankLabel={rankLabel}
      />,
    )
    expect((screen.getByRole('button', { name: 'previous rank' }) as HTMLButtonElement).disabled).toBe(true)
    expect((screen.getByRole('button', { name: 'next rank' }) as HTMLButtonElement).disabled).toBe(true)
    expect(screen.queryAllByRole('button', { name: /go to rank/ }).length).toBe(0)
  })

  it('fires pin and launch across the focused card wiring', () => {
    const onTogglePin = vi.fn()
    const onLaunch = vi.fn()
    const onHarnessChange = vi.fn()
    render(
      <ResultsCarousel
        items={models}
        metrics={metricsFor}
        index={0}
        onIndex={vi.fn()}
        rankLabel={rankLabel}
        pinned={(m) => m.model_id === 'gpt-5.6-luna'}
        onTogglePin={onTogglePin}
        harnessNames={['Claude Code', 'Codex']}
        selectedHarness="Claude Code"
        onLaunch={onLaunch}
        onHarnessChange={onHarnessChange}
      />,
    )
    // pin toggle present and reflects pinned state; gpt (index 0) is pinned,
    // so its toggle is "unpin model" and fires for that model
    expect(screen.getAllByRole('button', { name: 'unpin model' }).length).toBe(1)
    expect(screen.getAllByRole('button', { name: 'pin model' }).length).toBe(2)
    fireEvent.click(screen.getAllByRole('button', { name: 'unpin model' })[0])
    expect(onTogglePin).toHaveBeenCalledWith(models[0])
    // launch split button renders per card and routes the focused model
    fireEvent.click(screen.getAllByText('Launch in Claude Code')[0])
    expect(onLaunch).toHaveBeenCalledWith(models[0])
  })
})

describe('ModelRow', () => {
  it('marks the selected row and fires onSelect', () => {
    const onSelect = vi.fn()
    render(
      <table>
        <tbody>
          <ModelRow
            model={models[0]}
            metrics={coreMetrics}
            selected={true}
            onSelect={onSelect}
          />
        </tbody>
      </table>,
    )
    const row = screen.getAllByRole('row')[0]
    expect(row.className).toContain(rowStyles.selected)
    expect(row.getAttribute('aria-selected')).toBe('true')
    fireEvent.click(row)
    expect(onSelect).toHaveBeenCalledTimes(1)
  })

  it('launches without triggering row selection', () => {
    const onSelect = vi.fn()
    const onLaunch = vi.fn()
    render(
      <table>
        <tbody>
          <ModelRow
            model={models[0]}
            metrics={coreMetrics}
            selected={false}
            onSelect={onSelect}
            onLaunch={onLaunch}
          />
        </tbody>
      </table>,
    )
    const btn = screen.getByRole('button', { name: 'launch GPT-5.6 Luna' })
    fireEvent.click(btn)
    expect(onLaunch).toHaveBeenCalledTimes(1)
    expect(onSelect).not.toHaveBeenCalled()
  })
})

describe('ResultsTable', () => {
  it('renders header and a row per candidate with 2dp score', () => {
    render(
      <ResultsTable
        items={models}
        metrics={() => coreMetrics}
        selectedIndex={1}
        onSelect={vi.fn()}
      />,
    )
    for (const label of ['#', 'Model', 'Provider', 'Reasoning', 'Score', 'Metrics']) {
      expect(screen.getByText(label)).toBeTruthy()
    }
    expect(screen.getByText('GPT-5.6 Luna')).toBeTruthy()
    expect(screen.getByText('claude')).toBeTruthy()
    expect(screen.getByText('90.00')).toBeTruthy()
    expect(screen.getByText('92.41')).toBeTruthy()
    const rows = screen.getAllByRole('row')
    // header + 3 model rows
    expect(rows.length).toBe(4)
  })

  it('marks the selected row and picks on click', () => {
    const onSelect = vi.fn()
    const { getAllByRole } = render(
      <ResultsTable
        items={models}
        metrics={() => coreMetrics}
        selectedIndex={1}
        onSelect={onSelect}
      />,
    )
    const rows = getAllByRole('row')
    expect(rows[2].getAttribute('aria-selected')).toBe('true')
    fireEvent.click(rows[2])
    expect(onSelect).toHaveBeenCalledWith(1)
  })

  it('fires onLaunch with the clicked model', () => {
    const onLaunch = vi.fn()
    const { getAllByRole } = render(
      <ResultsTable
        items={models}
        metrics={() => coreMetrics}
        selectedIndex={0}
        onSelect={vi.fn()}
        onLaunch={onLaunch}
      />,
    )
    const buttons = getAllByRole('button', { name: /launch / })
    expect(buttons.length).toBe(3)
    fireEvent.click(
      screen.getByRole('button', { name: 'launch GPT-4 Mini' }),
    )
    expect(onLaunch).toHaveBeenCalledWith(models[2])
  })

  it('renders throw-free with no candidates', () => {
    render(
      <ResultsTable
        items={[]}
        metrics={() => coreMetrics}
        selectedIndex={0}
        onSelect={vi.fn()}
      />,
    )
    expect(screen.getByText('Model')).toBeTruthy()
  })
})

describe('EmptyCandidatesState', () => {
  it('renders friendly default copy', () => {
    render(<EmptyCandidatesState />)
    expect(screen.getByText('no routes')).toBeTruthy()
    expect(screen.getByText('No models match your routes')).toBeTruthy()
  })

  it('renders custom content and honours an optional action', () => {
    const onAction = vi.fn()
    render(
      <EmptyCandidatesState
        title="Nothing here yet"
        message="Enable a provider to rank candidates."
        actionLabel="Open providers"
        onAction={onAction}
      />,
    )
    expect(screen.getByText('Nothing here yet')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'Open providers' }))
    expect(onAction).toHaveBeenCalledTimes(1)
  })

  it('renders no action button without action props', () => {
    render(<EmptyCandidatesState />)
    expect(screen.queryByRole('button')).toBeNull()
  })
})