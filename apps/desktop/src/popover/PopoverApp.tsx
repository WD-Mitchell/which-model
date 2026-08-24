import { useCallback, useEffect, useState } from 'react'
import { Button, useToast } from '@which-model/ui'
import type { ErrorDTO, ProfileDetail, RankedModel } from '@which-model/core'
import {
  useCatalogLine,
  useComplexityScale,
  useHarnesses,
  useOverridesHash,
  useProfile,
  useProfiles,
  useRank,
  useSettings,
} from '../lib/queries'
import { copyToClipboard, getHost, hidePopover, openSettings, quit } from '../lib/host'
import { useOverridesStore } from '../lib/overrides'
import { onTrayProfile, onTrayView } from '../lib/trayEvents'
import { PopoverShell } from './PopoverShell'
import { PopoverHeader } from './Header'
import { PopoverFooter } from './Footer'
import { LandingView, ResultsBand } from './LandingView'
import { WeightsView } from './WeightsView'
import './PopoverApp.css'

/** The popover's two tabs. 'profiles' is Quick — the search + complexity
 *  scale; 'sliders' is Advanced — the per-benchmark weight editor. The ids
 *  keep their original names because gui.default_tab persists them. */
export type PopoverTab = 'profiles' | 'sliders'

/** Retained name for the footer's variant, which still speaks in views. */
export type PopoverView = 'landing' | 'weights'

/**
 * The popover's tab strip: Quick (search + complexity scale) and Advanced
 * (the per-benchmark weight editor).
 *
 * This replaced the weights view's own back-chevron header — with tabs, the
 * strip IS the navigation, so a second way back would be redundant.
 */
function PopoverTabs({ tab, onTab }: { tab: PopoverTab; onTab(next: PopoverTab): void }) {
  return (
    <div className="lv-tabs" role="tablist">
      <button
        type="button"
        role="tab"
        aria-selected={tab === 'profiles'}
        className={tab === 'profiles' ? 'lv-tab lv-tabOn' : 'lv-tab'}
        onClick={() => onTab('profiles')}
      >
        Quick
      </button>
      <button
        type="button"
        role="tab"
        aria-selected={tab === 'sliders'}
        className={tab === 'sliders' ? 'lv-tab lv-tabOn' : 'lv-tab'}
        onClick={() => onTab('sliders')}
      >
        Advanced
      </button>
    </div>
  )
}

export function PopoverApp() {
  const profilesQuery = useProfiles()
  const scaleQuery = useComplexityScale()
  const harnessesQuery = useHarnesses()
  const settingsQuery = useSettings()
  const catalogQuery = useCatalogLine()

  // null until settings resolve, so the first paint cannot show the wrong tab
  // and then swap under the user.
  const [tabOverride, setTabOverride] = useState<PopoverTab | null>(null)
  const [activeSlug, setActiveSlug] = useState('')
  const [selectedIndex, setSelectedIndex] = useState(0)
  const [stop, setStop] = useState(1)
  const [harnessSlug, setHarnessSlug] = useState<string | undefined>(undefined)
  const [harnessMenuOpen, setHarnessMenuOpen] = useState(false)

  const toast = useToast()

  const profiles = profilesQuery.data ?? []
  const scale = scaleQuery.data ?? []
  const harnesses = harnessesQuery.data ?? []
  const settings = settingsQuery.data

  // Seed the initial active profile (mockup: scale[1]) once the scale loads.
  useEffect(() => {
    if (!activeSlug && scale.length > 0) {
      const initial = scale[1] ?? scale[0]
      if (initial) {
        setActiveSlug(initial)
        setStop(scale.indexOf(initial))
      }
    }
  }, [scale, activeSlug])

  // Default harness = first listed (SPEC §2.9; reverts on relaunch).
  useEffect(() => {
    if (harnessSlug === undefined && harnesses.length > 0) {
      setHarnessSlug(harnesses[0].slug)
    }
  }, [harnesses, harnessSlug])

  // Close the harness menu when clicking outside the launch pill.
  useEffect(() => {
    if (!harnessMenuOpen) return
    function onDown(e: PointerEvent): void {
      const el = e.target as Element | null
      if (el && el.closest && !el.closest('[data-launch-pill]')) {
        setHarnessMenuOpen(false)
      }
    }
    window.addEventListener('pointerdown', onDown)
    return () => window.removeEventListener('pointerdown', onDown)
  }, [harnessMenuOpen])

  // Seed the overrides store whenever the active profile changes
  // (clear() + re-seed discards any ephemeral edits, U05 §2.7).
  const activeProfile = useProfile(activeSlug).data
  useEffect(() => {
    if (activeProfile && useOverridesStore.getState().baseSlug !== activeSlug) {
      useOverridesStore.getState().clear()
      useOverridesStore.getState().seed(activeProfile)
    }
  }, [activeProfile, activeSlug])

  // Content-driven window height (U05 divergence from the fixed 620 window):
  // the design's panel is content-sized, so the host window follows the
  // panel's natural height. Measured off .ps-panel — with .ps-outer at
  // height:auto its scrollHeight IS the natural stack height, independent of
  // the current window size, so this never feeds back on itself. Rounded up
  // and change-guarded to keep resize traffic at zero when nothing moved.
  useEffect(() => {
    const panel = document.querySelector('.ps-panel')
    if (!panel) return
    let last = 0
    const push = (): void => {
      const h = Math.ceil((panel as HTMLElement).scrollHeight)
      if (h > 0 && h !== last) {
        last = h
        void getHost().window.setPopoverHeight(h).catch(() => {})
      }
    }
    push()
    // jsdom (vitest) has no ResizeObserver; the initial push above still runs,
    // so the measurement contract is exercised without the observer.
    if (typeof ResizeObserver === 'undefined') return
    const ro = new ResizeObserver(push)
    ro.observe(panel)
    return () => ro.disconnect()
  }, [])

  const activeName = profiles.find((p) => p.slug === activeSlug)?.name ?? activeSlug
  const overridesHash = useOverridesHash(activeSlug)
  const holds = settings?.holds ?? 5
  const rankQuery = useRank(activeSlug, overridesHash, holds)
  const candidates = rankQuery.data?.candidates ?? []
  const pickIndex = candidates.length === 0 ? 0 : Math.min(selectedIndex, candidates.length - 1)
  const pick = candidates[pickIndex]

  // Keep the menu bar showing what the popover shows. The host ranks for the
  // menu bar itself at startup, but the active profile and the ephemeral weight
  // overrides never leave the webview — so without this push the menu bar kept
  // naming whichever profile the host last ranked, unmoved by the scale or the
  // sliders. Runs on every change of profile, pick or rank (a weight edit
  // refetches the rank, which lands here as a new pick).
  useEffect(() => {
    // Nothing is pushed until there IS something: the first paints have no
    // profile and no rank yet, and pushing those would blank the menu bar for
    // a beat every time the popover mounts. Until the first real pick, the
    // host's own startup ranking holds the title.
    if (!activeName || !pick) return
    void getHost()
      .window.setTrayPick(activeName, pick.model_name, pick.reasoning, pick.provider)
      .catch(() => {})
  }, [activeName, pick?.model_name, pick?.reasoning, pick?.provider])

  const handleSelectProfile = useCallback((slug: string, scaleIndex: number | null) => {
    setSelectedIndex(0)
    if (scaleIndex !== null) setStop(scaleIndex)
    // overrides clear + re-seed happens in the effect keyed on activeSlug
    setActiveSlug(slug)
  }, [])

  // Menu-bar right-click → profile quick-select (traymenu.go emits
  // "tray:profile"). Same entry point as clicking a stop on the scale; profiles
  // that are not on the complexity scale leave the scale position alone.
  useEffect(
    () =>
      onTrayProfile((slug) => {
        const scaleIndex = scale.indexOf(slug)
        handleSelectProfile(slug, scaleIndex === -1 ? null : scaleIndex)
      }),
    [scale, handleSelectProfile],
  )

  const handleCustomWeights = useCallback(() => {
    setTabOverride('sliders')
  }, [])

  // Tray menu -> "Custom weights…". The menu shows the popover host-side; this
  // only decides which view it lands on.
  useEffect(
    () =>
      onTrayView((view) => {
        // The tray still speaks in views ("Custom weights…"); they map onto tabs.
        if (view === 'weights') setTabOverride('sliders')
        else if (view === 'landing') setTabOverride('profiles')
      }),
    [],
  )

  // Copy the current pick's model id. Lives here, not in the weights actions,
  // because the footer offers it on both tabs now.
  const handleCopyModelId = useCallback(async () => {
    if (!pick) {
      toast.show('no model to copy — enable a provider')
      return
    }
    try {
      await copyToClipboard(pick.model_id)
      toast.show(`copied ${pick.model_id}`)
    } catch (e) {
      toast.show((e as ErrorDTO).message ?? 'copy failed')
    }
  }, [pick, toast])

  const handleManage = useCallback(() => {
    void openSettings()
  }, [])

  const handlePickHarness = useCallback((slug: string) => {
    setHarnessSlug(slug)
    setHarnessMenuOpen(false)
  }, [])

  const handleLaunch = useCallback(async () => {
    if (!pick || !harnessSlug) {
      toast.show('no model to launch — enable a provider')
      return
    }
    try {
      const result = await getHost().harnesses.launch(harnessSlug, pick.route_key, activeSlug)
      if (result.copied) {
        await copyToClipboard(result.command)
      }
      toast.show(result.command)
      if (settings?.close_popover_after_launch) {
        await hidePopover()
      }
    } catch (e) {
      toast.show((e as ErrorDTO).message ?? 'launch failed')
    }
  }, [pick, harnessSlug, activeSlug, settings, toast])

  // Shipped default is 'profiles'; gui.default_tab overrides it until the user
  // picks a tab in this session.
  const tab: PopoverTab =
    tabOverride ?? (settings?.default_tab === 'sliders' ? 'sliders' : 'profiles')

  // The header carries the hero — it describes the app, so it sits above the
  // tab strip and shows on both tabs.
  const catalog = catalogQuery.data
  const catalogLine = catalog
    ? `${catalog.models} models · ${catalog.providers_on} providers on · ${catalog.harnesses} harnesses`
    : '—'
  const header = <PopoverHeader catalogLine={catalogLine} />

  const body =
    tab === 'profiles' ? (
      <LandingView
        profiles={profiles}
        scale={scale}
        activeSlug={activeSlug}
        activeName={activeName}
        stop={stop}
        overridesHash={overridesHash}
        onSelectProfile={handleSelectProfile}
        index={selectedIndex}
        onIndex={setSelectedIndex}
      />
    ) : (
      <div className="wa-wbody">
        {/* No divider rule here: it used to separate the weights body from the
            "Weights for <slug>" header row, which the tab strip replaced. */}
        <WeightsView
          baseSlug={activeSlug}
          actions={
            <WeightsActions
              baseProfile={activeProfile}
              onSaved={(slug) => {
                setTabOverride('profiles')
                setActiveSlug(slug)
                setSelectedIndex(0)
                useOverridesStore.getState().clear()
              }}
            />
          }
        />
        <ResultsBand
          slug={activeSlug}
          overridesHash={overridesHash}
          index={selectedIndex}
          onIndex={setSelectedIndex}
        />
      </div>
    )

  return (
    <PopoverShell header={header}>
      <PopoverTabs tab={tab} onTab={setTabOverride} />
      {body}
      {/* Tab-independent: Settings + Launch stay put on both tabs. Only the
          content area above changes. */}
      <PopoverFooter
        harnesses={harnesses}
        harnessSlug={harnessSlug}
        harnessMenuOpen={harnessMenuOpen}
        onToggleHarnessMenu={() => setHarnessMenuOpen((v) => !v)}
        onPickHarness={handlePickHarness}
        onManage={handleManage}
        onCopy={() => void handleCopyModelId()}
        onLaunch={() => void handleLaunch()}
      />
    </PopoverShell>
  )
}

// U06 weights actions: Copy model id + Save as profile. They sit in the
// weights editor's action row — the footer is the same on both tabs.
function WeightsActions({
  baseProfile,
  onSaved,
}: {
  baseProfile: ProfileDetail | undefined
  onSaved(slug: string): void
}) {
  const toast = useToast()
  const store = useOverridesStore()
  const baseSlug = store.baseSlug

  const handleSave = async () => {
    if (!baseProfile) return
    let n = 1
    // eslint-disable-next-line no-constant-condition
    while (true) {
      const slug = n === 1 ? `${baseSlug}_custom` : `${baseSlug}_custom_${n}`
      const name = n === 1 ? `${baseProfile.name} (custom)` : `${baseProfile.name} (custom ${n})`
      const detail: ProfileDetail = {
        ...baseProfile,
        slug,
        name,
        builtin: false,
        core_share: store.coreShare,
        tier1_weights: { ...store.tier1 },
        tier2_weights: { ...store.tier2 },
        picks: 0,
        last_used: '',
      }
      try {
        await getHost().profiles.save(detail)
        toast.show(`saved as ${slug}`)
        store.clear()
        onSaved(slug)
        return
      } catch (e) {
        if ((e as ErrorDTO).code === 'conflict') {
          n += 1
          continue
        }
        toast.show((e as ErrorDTO).message ?? 'save failed')
        return
      }
    }
  }

  return (
    <>
      {/* Copy model id moved to the footer (both tabs offer it). What is left
          is the one action that only makes sense here, at the action row's
          `xs` scale, bordered as this tab's committing action. */}
      <Button variant="secondary" size="xs" onClick={() => void handleSave()}>
        Save as profile
      </Button>
    </>
  )
}