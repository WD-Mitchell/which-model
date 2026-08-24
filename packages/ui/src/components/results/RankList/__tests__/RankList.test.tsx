import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { RankedModel } from '@which-model/core'
import { RankList } from '../RankList'

// One model at three efforts is the ordinary case, not an edge case: adjacent
// ranks routinely differ only by reasoning.
const items: RankedModel[] = [
  { rank: 1, model_id: 'gpt-5.6-sol', model_name: 'GPT-5.6 Sol', provider: 'codex', reasoning: 'medium', score: 91.27, route_key: 'codex/gpt-5.6-sol@medium' },
  { rank: 2, model_id: 'gpt-5.6-sol', model_name: 'GPT-5.6 Sol', provider: 'codex', reasoning: 'low', score: 89.92, route_key: 'codex/gpt-5.6-sol@low' },
  { rank: 3, model_id: 'claude-opus-5', model_name: 'Claude Opus 5', provider: 'claude', reasoning: 'high', score: 89.4, route_key: 'claude/claude-opus-5@high' },
]

describe('RankList', () => {
  it('brackets the reasoning after the model name so rows are distinguishable', () => {
    render(<RankList items={items} index={0} onPick={vi.fn()} />)
    // Without reasoning the first two rows read identically: same name, same
    // provider — which is the bug this pins. It belongs to the model's
    // identity, so it renders inline in the name's own type.
    expect(screen.getByText('GPT-5.6 Sol (medium)')).toBeDefined()
    expect(screen.getByText('GPT-5.6 Sol (low)')).toBeDefined()
    expect(screen.getByText('Claude Opus 5 (high)')).toBeDefined()
  })

  it('leaves the name bare when a route carries no reasoning', () => {
    const noReasoning: RankedModel[] = [{ ...items[0], reasoning: '' }]
    render(<RankList items={noReasoning} index={0} onPick={vi.fn()} />)
    expect(screen.getByText('GPT-5.6 Sol')).toBeDefined()
  })
})
