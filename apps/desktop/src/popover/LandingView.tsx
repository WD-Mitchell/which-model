import { useState } from 'react'
import { Combobox, ComplexityScale, RankCarousel, RankList } from '@which-model/ui'
import type { CatalogSummary, ProfileSummary } from '@which-model/core'
import { useRank, useSettings } from '../lib/queries'
import './LandingView.css'

export interface LandingViewProps {
  catalog?: CatalogSummary
  profiles: ProfileSummary[]
  scale: string[]
  activeSlug: string
  activeName: string
  /** 0..4, sticky — the displayed ComplexityScale handle (SPEC §2.6). */
  stop: number
  overridesHash: string
  /** Fired on any profile selection (search pick or scale drag) with the
   *  scale index when the slug is on the scale, else null. */
  onSelectProfile(slug: string, scaleIndex: number | null): void
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
  catalog,
  profiles,
  scale,
  activeSlug,
  activeName,
  stop,
  overridesHash,
  onSelectProfile,
  index,
  onIndex,
}: LandingViewProps) {
  const [query, setQuery] = useState('')
  const [searchOpen, setSearchOpen] = useState(false)

  const q = query.trim().toLowerCase()
  const matches = profiles
    .filter((p) => !q || p.name.toLowerCase().includes(q) || p.slug.includes(q))
    .slice(0, 5)
  const comboboxItems = matches.map((p) => ({
    key: p.slug,
    label: p.slug,
    sub: `${p.core_share}/${100 - p.core_share}`,
  }))

  function handlePick(slug: string): void {
    const i = scale.indexOf(slug)
    setQuery('')
    setSearchOpen(false)
    onSelectProfile(slug, i >= 0 ? i : null)
  }

  const catalogLine = catalog
    ? `${catalog.models} models · ${catalog.providers_on} providers on · ${catalog.harnesses} harnesses`
    : '—'

  return (
    <div className="lv">
      <div className="lv-hero">
        <h1 className="lv-title">The right model for the job in front of you.</h1>
        <div className="mono lv-catalog">{catalogLine}</div>
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
          emptyText="no profile by that name"
          placeholder="type to find a profile"
          selectedKey={activeSlug}
        />
      </div>

      <div className="lv-scaleLabel">or slide</div>
      <div className="lv-scale">
        <ComplexityScale
          stop={stop}
          labels={['simple action', 'planning']}
          profileName={activeName}
          onStop={(i) => {
            if (scale[i]) onSelectProfile(scale[i], i)
          }}
        />
      </div>

      <ResultsBand slug={activeSlug} overridesHash={overridesHash} index={index} onIndex={onIndex} />
    </div>
  )
}