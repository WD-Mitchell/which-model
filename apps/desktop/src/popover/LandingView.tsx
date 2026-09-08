import { useState } from 'react'
import { Combobox, RankCarousel, RankList } from '@which-model/ui'
import type { ProfileSummary, UserProfile } from '@which-model/core'
import { useRank, useSettings } from '../lib/queries'
import './LandingView.css'

/** The popover's two landing tabs (gui.default_tab). */
export type PopoverTab = 'profiles' | 'sliders'

export interface LandingViewProps {
  profiles: ProfileSummary[]
  userProfile?: UserProfile
  activeSlug: string
  activeName: string
  overridesHash: string
  /** Select a use case without changing the saved user profile. */
  onSelectProfile(slug: string): void
  /** Controlled rank selection index. */
  index: number
  onIndex(i: number): void
}

export function ResultsBand({ slug, overridesHash, index, onIndex }: {
  slug: string
  overridesHash: string
  index: number
  onIndex(i: number): void
}) {
  const { data: settings } = useSettings()
  const layout = settings?.layout ?? 'carousel'
  const holds = settings?.holds ?? 5
  const { data: rank, isError } = useRank(slug, overridesHash, holds)
  const candidates = rank?.candidates ?? []
  const pickIndex = candidates.length === 0 ? 0 : Math.min(index, candidates.length - 1)

  if (isError) {
    return (
      <div className="lv-errorBand">
        <div className="lv-errorCenter">
          <span className="lv-errorLine">—</span>
          <span className="lv-errorLine">—</span>
          <span className="lv-errorLine">—</span>
        </div>
      </div>
    )
  }

  // List layout still falls back to the carousel's empty state (U04 §2.2.4).
  if (layout === 'carousel' || candidates.length === 0) {
    return <RankCarousel items={candidates} index={pickIndex} onIndex={onIndex} />
  }
  return <RankList items={candidates} index={pickIndex} onPick={onIndex} />
}

export function LandingView({
  profiles,
  userProfile,
  activeSlug,
  activeName,
  overridesHash,
  onSelectProfile,
  index,
  onIndex,
}: LandingViewProps) {
  const [showAll, setShowAll] = useState(false)
  const [query, setQuery] = useState('')
  const [searchOpen, setSearchOpen] = useState(false)

  const q = query.trim().toLowerCase()
  const defaults = (userProfile?.use_case_slugs ?? [])
    .map((slug) => profiles.find((p) => p.slug === slug))
    .filter((p): p is ProfileSummary => Boolean(p))
  const matches = (q || showAll ? profiles : defaults)
    .filter((p) => !q || `${p.name} ${p.slug} ${p.description ?? ''}`.toLowerCase().includes(q))
  const comboboxItems = matches.map((p) => ({ key: p.slug, label: p.name, sub: p.builtin ? '' : 'Custom' }))
  const active = profiles.find((p) => p.slug === activeSlug)

  function handlePick(slug: string): void {
    setQuery('')
    setSearchOpen(false)
    onSelectProfile(slug)
  }

  return (
    <div className="lv">
      <div className="lv-use-case">
        <div className="lv-use-case-scope">
          <span>{showAll ? 'All use cases' : `${userProfile?.name ?? 'Your profile'} use cases`}</span>
          <button type="button" onClick={() => { setShowAll((v) => !v); setSearchOpen(true) }}>
            {showAll ? 'Profile defaults' : 'All use cases'}
          </button>
        </div>
        <h2>{activeName || 'Choose a use case'}</h2>
        {active?.description && <p>{active.description}</p>}
        {active?.evidence_note && <p className="lv-evidence-note">{active.evidence_note}</p>}
      </div>

      <div className="lv-search">
        <Combobox
          items={comboboxItems}
          query={query}
          onQuery={(v) => {
            setQuery(v)
            setSearchOpen(true)
          }}
          open={searchOpen}
          onOpenChange={setSearchOpen}
          onPick={handlePick}
          emptyText="no use case by that name"
          placeholder="type to find a use case"
          selectedKey={activeSlug}
        />
      </div>

      {/* Mockup's fixed 10px filler (line 87); the window is content-sized. */}
      <div className="lv-spacer" />

      <ResultsBand slug={activeSlug} overridesHash={overridesHash} index={index} onIndex={onIndex} />
    </div>
  )
}