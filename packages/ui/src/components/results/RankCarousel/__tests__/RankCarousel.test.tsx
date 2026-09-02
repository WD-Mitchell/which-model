import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { RankedModel } from '@which-model/core'
import { RankCarousel } from '../RankCarousel'

const items: RankedModel[] = [
  {
    rank: 1,
    model_id: 'gpt-5.6-sol',
    model_name: 'GPT-5.6 Sol',
    provider: 'codex',
    reasoning: 'medium',
    score: 91.27,
    route_key: 'codex/gpt-5.6-sol@medium',
    intelligence: 85,
    cost: 92,
    speed: 78,
  },
  {
    rank: 2,
    model_id: 'claude-opus-5',
    model_name: 'Claude Opus 5',
    provider: 'claude',
    reasoning: 'high',
    score: 89.4,
    route_key: 'claude/claude-opus-5@high',
    intelligence: 95,
    cost: 60,
    speed: 70,
  },
]

describe('RankCarousel', () => {
  it('renders model name, reasoning, and metadata', () => {
    render(<RankCarousel items={items} index={0} onIndex={vi.fn()} />)
    expect(screen.getByText('GPT-5.6 Sol (medium)')).toBeDefined()
    expect(screen.getByText('codex · 91.27')).toBeDefined()
  })

  it('renders the 3 core ratings under the model', () => {
    render(<RankCarousel items={items} index={0} onIndex={vi.fn()} />)
    expect(screen.getByText('intel')).toBeDefined()
    expect(screen.getByText('85')).toBeDefined()
    expect(screen.getByText('cost')).toBeDefined()
    expect(screen.getByText('92')).toBeDefined()
    expect(screen.getByText('speed')).toBeDefined()
    expect(screen.getByText('78')).toBeDefined()
  })

  it('renders clean empty state when no items exist', () => {
    render(<RankCarousel items={[]} index={0} onIndex={vi.fn()} />)
    expect(screen.getByText('Enable a provider')).toBeDefined()
    expect(screen.getByText('every provider is switched off')).toBeDefined()
  })
})
