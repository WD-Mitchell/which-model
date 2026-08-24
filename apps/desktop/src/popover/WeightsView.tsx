// U06 — WeightsView: the popover's ephemeral weights editor. Renders the
// tinted editor section (core/task rows), the add-metric / Revert actions,
// and the core-task balance band. Mounted by PopoverApp in the 'weights' view
// above the shared carousel; the weights actions render in the editor's own
// action row (the popover footer is tab-independent).
import type { ReactNode } from 'react'
import { useEffect, useMemo, useState } from 'react'
import { BalanceSlider, useToast, WeightEditor } from '@which-model/ui'
import { useGroups, useProfile, useSettings } from '../lib/queries'
import { CORE_KEYS, useOverridesStore } from '../lib/overrides'
import './WeightsView.css'

export interface WeightsViewProps {
  baseSlug: string
  /** Buttons for the editor's action row (Copy model id / Save as profile).
   *  They live here rather than the footer, which no longer changes per tab. */
  actions?: ReactNode
}

export function WeightsView({ baseSlug, actions }: WeightsViewProps) {
  const profile = useProfile(baseSlug).data
  const groups = useGroups().data ?? []
  const settings = useSettings().data
  const toast = useToast()
  const [addOpen, setAddOpen] = useState(false)

  const store = useOverridesStore()
  const coreShare = store.coreShare
  const tier1 = store.tier1
  const tier2 = store.tier2
  const sliderStyle = settings?.weight_control ?? 'slider'

  // Re-seed the store when the base profile differs/empty (after render).
  useEffect(() => {
    if (profile && (store.baseSlug !== baseSlug || !store.baseSlug)) {
      store.clear()
      store.seed(profile)
    }
  }, [profile, baseSlug, store])

  const coreRows = useMemo(
    () => CORE_KEYS.map((k) => ({ key: k, value: tier1[k] ?? 0, accent: k === 'cost' })),
    [tier1],
  )

  const taskRows = useMemo(
    () =>
      Object.keys(tier2)
        .sort()
        .map((k) => ({ key: k, value: tier2[k] ?? 0 })),
    [tier2],
  )

  const addable = useMemo(() => {
    const present = new Set([...Object.keys(tier1), ...Object.keys(tier2)])
    const groupsSorted = [...groups].sort((a, b) => a.slug.localeCompare(b.slug))
    return [...CORE_KEYS, ...groupsSorted.map((g) => g.slug)].filter((k) => !present.has(k))
  }, [tier1, tier2, groups])

  return (
    <div className="wv-root">
      {/* Mockup line 133: the metric rows are a full-bleed tinted section, and
          in the fixed-height popover window they are the scroll region. */}
      <div className="wv-metrics scroll">
        <WeightEditor
          variant="popover"
          sliderStyle={sliderStyle}
          coreRows={coreRows}
          taskRows={taskRows}
          sectionPcts={{ core: `${coreShare}%`, task: `${100 - coreShare}%` }}
          addable={addable}
          addOpen={addOpen}
          onChangeWeight={(key, v) => store.setWeight(key, v)}
          onRemoveWeight={(key) => store.removeMetric(key)}
          onAddMetric={(key) => {
            store.addMetric(key)
            setAddOpen(false)
          }}
          extraActions={actions}
          onToggleAdd={() => setAddOpen((v) => !v)}
          onRevert={() => {
            if (profile) {
              store.revert(profile)
              toast.show(`weights reverted to ${baseSlug}`)
            }
          }}
        />
      </div>
      {/* Mockup line 181: balance band — tinted, divider-topped. BalanceSlider
          renders its own core/task caption row, so none is added here. */}
      <div className="wv-balance">
        <BalanceSlider core={coreShare} onChange={(v) => store.setCoreShare(v)} />
      </div>
    </div>
  )
}