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
import { PopoverShell } from './PopoverShell'
import { PopoverHeader } from './Header'
import { PopoverFooter } from './Footer'
import { LandingView, ResultsBand } from './LandingView'
import { WeightsView } from './WeightsView'
import './PopoverApp.css'

export type PopoverView = 'landing' | 'weights'

function WeightsHeader({
  slug,
  onBack,
  onToggleMenu,
}: {
  slug: string
  onBack(): void
  onToggleMenu(): void
}) {
  return (
    <div className="wa-wheaderRow">
      <button
        type="button"
        className="ib wa-wheaderBack"
        aria-label="Back to landing"
        onClick={(e) => {
          e.stopPropagation()
          onBack()
        }}
      >
        <svg
          width="13"
          height="13"
          viewBox="0 0 12 12"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.7"
          strokeLinecap="round"
          strokeLinejoin="round"
        >
          <path d="M7.2 2.2 3.4 6l3.8 3.8"></path>
        </svg>
      </button>
      <span className="wa-wheaderTitle">
        Weights for <span className="mono wa-wheaderSlug">{slug}</span>
      </span>
      <button
        type="button"
        className="ib wa-wheaderHamburger"
        aria-label="App menu"
        onClick={(e) => {
          e.stopPropagation()
          onToggleMenu()
        }}
      >
        <svg
          width="13"
          height="13"
          viewBox="0 0 16 16"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.5"
          strokeLinecap="round"
        >
          <path d="M1.8 4.2h12.4M1.8 8h12.4M1.8 11.8h12.4"></path>
          <circle cx="5.4" cy="4.2" r="1.7" fill="var(--color-bg)"></circle>
          <circle cx="10.6" cy="11.8" r="1.7" fill="var(--color-bg)"></circle>
        </svg>
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

  const [view, setView] = useState<PopoverView>('landing')
  const [activeSlug, setActiveSlug] = useState('')
  const [selectedIndex, setSelectedIndex] = useState(0)
  const [stop, setStop] = useState(1)
  const [harnessSlug, setHarnessSlug] = useState<string | undefined>(undefined)
  const [appMenuOpen, setAppMenuOpen] = useState(false)
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

  const activeName = profiles.find((p) => p.slug === activeSlug)?.name ?? activeSlug
  const overridesHash = useOverridesHash(activeSlug)
  const holds = settings?.holds ?? 5
  const rankQuery = useRank(activeSlug, overridesHash, holds)
  const candidates = rankQuery.data?.candidates ?? []
  const pickIndex = candidates.length === 0 ? 0 : Math.min(selectedIndex, candidates.length - 1)
  const pick = candidates[pickIndex]

  const handleSelectProfile = useCallback((slug: string, scaleIndex: number | null) => {
    setSelectedIndex(0)
    if (scaleIndex !== null) setStop(scaleIndex)
    // overrides clear + re-seed happens in the effect keyed on activeSlug
    setActiveSlug(slug)
  }, [])

  const toggleAppMenu = useCallback(() => {
    setAppMenuOpen((v) => !v)
    setHarnessMenuOpen(false)
  }, [])

  const handleCustomWeights = useCallback(() => {
    setAppMenuOpen(false)
    setView('weights')
  }, [])

  const handleOpenSettings = useCallback(() => {
    setAppMenuOpen(false)
    void openSettings()
  }, [])

  const handleQuit = useCallback(() => {
    setAppMenuOpen(false)
    void quit()
  }, [])

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

  const header =
    view === 'landing' ? (
      <PopoverHeader onToggleMenu={toggleAppMenu} />
    ) : (
      <WeightsHeader
        slug={activeSlug}
        onBack={() => setView('landing')}
        onToggleMenu={toggleAppMenu}
      />
    )

  const body =
    view === 'landing' ? (
      <LandingView
        catalog={catalogQuery.data}
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
        <div className="wa-wdivider">
          <div className="wa-wdividerLine" />
        </div>
        <WeightsView baseSlug={activeSlug} />
        <ResultsBand
          slug={activeSlug}
          overridesHash={overridesHash}
          index={selectedIndex}
          onIndex={setSelectedIndex}
        />
      </div>
    )

  const weightsFooter =
    view === 'weights' ? (
      <WeightsFooter
        pick={pick}
        baseProfile={activeProfile}
        onSaved={(slug) => {
          setView('landing')
          setActiveSlug(slug)
          setSelectedIndex(0)
          useOverridesStore.getState().clear()
        }}
      />
    ) : null

  return (
    <PopoverShell
      header={header}
      menuOpen={appMenuOpen}
      onToggleMenu={toggleAppMenu}
      onCustomWeights={handleCustomWeights}
      onOpenSettings={handleOpenSettings}
      onQuit={handleQuit}
    >
      {body}
      <PopoverFooter
        variant={view}
        harnesses={harnesses}
        harnessSlug={harnessSlug}
        harnessMenuOpen={harnessMenuOpen}
        onToggleHarnessMenu={() => setHarnessMenuOpen((v) => !v)}
        onPickHarness={handlePickHarness}
        onManage={handleManage}
        onLaunch={() => void handleLaunch()}
      >
        {weightsFooter}
      </PopoverFooter>
    </PopoverShell>
  )
}

// U06 footer buttons: Copy model id + Save as profile.
function WeightsFooter({
  pick,
  baseProfile,
  onSaved,
}: {
  pick: RankedModel | undefined
  baseProfile: ProfileDetail | undefined
  onSaved(slug: string): void
}) {
  const toast = useToast()
  const store = useOverridesStore()
  const baseSlug = store.baseSlug

  const handleCopy = async () => {
    if (!pick) {
      toast.show('nothing to copy')
      return
    }
    await copyToClipboard(pick.model_id)
    toast.show(`copied  ${pick.model_id}`)
  }

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
      <Button variant="ghost" onClick={() => void handleCopy()}>
        Copy model id
      </Button>
      <Button variant="ghost" onClick={() => void handleSave()}>
        Save as profile
      </Button>
    </>
  )
}