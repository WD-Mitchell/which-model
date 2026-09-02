// U15 — Models settings page: catalog-wide list with search, and a summary
// detail for one catalog identity (name, id, reasoning, intel/cost/speed).
//
// Layout follows U09/U14: <main> has no horizontal padding, so every block
// here carries its own 22px gutter and `.row` hover tints bleed edge to edge.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Button, EmptyState, Input, Tag, cx } from '@which-model/ui'
import { useCatalogModels } from '../../../lib/queries'
import { DetailHeader } from '../../DetailHeader'
import { PAGE_META } from '../../pages'
import type { Detail, PageComponentProps } from '../../pages'
import { ModelBenchmarksView } from '../Providers/ProvidersPage'
import { ModelCard } from './ModelCard'
import styles from './ModelsPage.module.css'

const LIST_KICKER = 'catalog models'
const FILTER_PLACEHOLDER = 'filter models'
const EMPTY_CATALOG = 'no models in the catalog'
const EMPTY_FILTER = 'no models match'
const LOAD_ERROR = "couldn't load models"

function formatScore(n: number | null | undefined): string {
  if (n == null) return '—'
  return Number.isInteger(n) ? String(n) : n.toFixed(1)
}

function ChevronIcon() {
  return (
    <svg
      width="10"
      height="10"
      viewBox="0 0 12 12"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M4.4 2.2 8.2 6l-3.8 3.8" />
    </svg>
  )
}

export function ModelsPage({ detail, openDetail, closeDetail }: PageComponentProps) {
  if (detail?.kind === 'provider-model') {
    return (
      <ModelBenchmarksView
        provider={detail.provider}
        modelName={detail.modelName}
        reasoning={detail.reasoning}
        onBack={closeDetail}
      />
    )
  }
  if (detail?.kind === 'model') {
    return (
      <ModelCard
        name={detail.id}
        backLabel={detail.fromProvider ?? 'Models'}
        onBack={closeDetail}
        openDetail={openDetail}
      />
    )
  }
  return <ModelsListView openDetail={openDetail} />
}

function extractFallbackMaker(name: string): string {
  const lower = name.toLowerCase()
  if (lower.startsWith('claude')) return 'Anthropic'
  if (lower.startsWith('gpt') || lower.startsWith('o1') || lower.startsWith('o3') || lower.startsWith('o4')) return 'OpenAI'
  if (lower.startsWith('gemini') || lower.startsWith('gemma')) return 'Google'
  if (lower.startsWith('qwen')) return 'Qwen'
  if (lower.startsWith('deepseek')) return 'DeepSeek'
  if (lower.startsWith('grok')) return 'xAI'
  if (lower.startsWith('llama')) return 'Meta'
  if (lower.startsWith('mistral') || lower.startsWith('codestral')) return 'Mistral'
  if (lower.startsWith('command')) return 'Cohere'
  const first = name.split(/\s+/)[0]
  return first || 'Other'
}

function ModelsListView({ openDetail }: { openDetail(d: Detail): void }) {
  const { data, isError, isPending, refetch } = useCatalogModels()
  const [query, setQuery] = useState('')
  const [selectedMakers, setSelectedMakers] = useState<string[]>([])
  const [selectedProviders, setSelectedProviders] = useState<string[]>([])
  const [makerMenuOpen, setMakerMenuOpen] = useState(false)
  const [providerMenuOpen, setProviderMenuOpen] = useState(false)
  const filterRef = useRef<HTMLDivElement>(null)

  const list = data ?? []
  const needle = query.trim().toLowerCase()

  const allMakers = useMemo(() => {
    const set = new Set<string>()
    for (const m of list) {
      const maker = m.maker || extractFallbackMaker(m.model_name)
      if (maker) set.add(maker)
    }
    return Array.from(set).sort()
  }, [list])

  const allProviders = useMemo(() => {
    const set = new Set<string>()
    for (const m of list) {
      for (const p of m.providers ?? []) set.add(p)
    }
    return Array.from(set).sort()
  }, [list])

  useEffect(() => {
    function onDown(e: PointerEvent) {
      if (filterRef.current && !filterRef.current.contains(e.target as Node)) {
        setMakerMenuOpen(false)
        setProviderMenuOpen(false)
      }
    }
    window.addEventListener('pointerdown', onDown)
    return () => window.removeEventListener('pointerdown', onDown)
  }, [])

  const toggleMaker = useCallback((maker: string) => {
    setSelectedMakers((prev) =>
      prev.includes(maker) ? prev.filter((x) => x !== maker) : [...prev, maker],
    )
  }, [])

  const toggleProvider = useCallback((provider: string) => {
    setSelectedProviders((prev) =>
      prev.includes(provider) ? prev.filter((x) => x !== provider) : [...prev, provider],
    )
  }, [])

  const clearAllFilters = useCallback(() => {
    setQuery('')
    setSelectedMakers([])
    setSelectedProviders([])
  }, [])

  const visible = useMemo(() => {
    return list.filter((m) => {
      if (
        needle &&
        !m.model_name.toLowerCase().includes(needle) &&
        !m.model_id.toLowerCase().includes(needle)
      ) {
        return false
      }
      const maker = m.maker || extractFallbackMaker(m.model_name)
      if (selectedMakers.length > 0 && (!maker || !selectedMakers.includes(maker))) {
        return false
      }
      const provs = m.providers ?? []
      if (
        selectedProviders.length > 0 &&
        !selectedProviders.some((p) => provs.includes(p))
      ) {
        return false
      }
      return true
    })
  }, [list, needle, selectedMakers, selectedProviders])
  return (
    <div className={styles.page}>
      <DetailHeader title={PAGE_META.Models[0]} blurb={PAGE_META.Models[1]} />
      <div className={styles.filterSection} ref={filterRef}>
        <div className={styles.filterBar}>
          <div className={styles.searchWrap}>
            <Input
              value={query}
              onChange={setQuery}
              placeholder={FILTER_PLACEHOLDER}
              mono={false}
            />
          </div>
          <div className={styles.dropdownWrap}>
            <button
              type="button"
              className={cx(styles.filterBtn, selectedMakers.length > 0 && styles.filterBtnActive)}
              onClick={() => {
                setMakerMenuOpen((v) => !v)
                setProviderMenuOpen(false)
              }}
              aria-label="Filter by maker"
              aria-expanded={makerMenuOpen}
            >
              {selectedMakers.length === 0 ? 'All makers' : `Maker (${selectedMakers.length})`}
              <span className={styles.filterChev}>▾</span>
            </button>
            {makerMenuOpen && (
              <div className={styles.filterMenu} role="menu">
                {allMakers.map((maker) => {
                  const checked = selectedMakers.includes(maker)
                  return (
                    <div
                      key={maker}
                      className={cx(styles.filterItem, checked && styles.filterItemSelected)}
                      onClick={() => toggleMaker(maker)}
                      role="menuitemcheckbox"
                      aria-checked={checked}
                    >
                      <span className={styles.checkbox}>{checked ? '✓' : ''}</span>
                      <span className={styles.filterLabel}>{maker}</span>
                    </div>
                  )
                })}
              </div>
            )}
          </div>
          <div className={styles.dropdownWrap}>
            <button
              type="button"
              className={cx(styles.filterBtn, selectedProviders.length > 0 && styles.filterBtnActive)}
              onClick={() => {
                setProviderMenuOpen((v) => !v)
                setMakerMenuOpen(false)
              }}
              aria-label="Filter by provider"
              aria-expanded={providerMenuOpen}
            >
              {selectedProviders.length === 0 ? 'All providers' : `Provider (${selectedProviders.length})`}
              <span className={styles.filterChev}>▾</span>
            </button>
            {providerMenuOpen && (
              <div className={styles.filterMenu} role="menu">
                {allProviders.map((provider) => {
                  const checked = selectedProviders.includes(provider)
                  return (
                    <div
                      key={provider}
                      className={cx(styles.filterItem, checked && styles.filterItemSelected)}
                      onClick={() => toggleProvider(provider)}
                      role="menuitemcheckbox"
                      aria-checked={checked}
                    >
                      <span className={styles.checkbox}>{checked ? '✓' : ''}</span>
                      <span className={styles.filterLabel}>{provider}</span>
                    </div>
                  )
                })}
              </div>
            )}
          </div>
        </div>
        {selectedMakers.length > 0 || selectedProviders.length > 0 ? (
          <div className={styles.activeChips}>
            {selectedMakers.map((m) => (
              <span key={m} className={styles.filterChip}>
                <span className={styles.chipText}>Maker: {m}</span>
                <button
                  type="button"
                  className={styles.chipClose}
                  onClick={() => toggleMaker(m)}
                  aria-label={`Remove maker ${m}`}
                >
                  ×
                </button>
              </span>
            ))}
            {selectedProviders.map((p) => (
              <span key={p} className={styles.filterChip}>
                <span className={styles.chipText}>Provider: {p}</span>
                <button
                  type="button"
                  className={styles.chipClose}
                  onClick={() => toggleProvider(p)}
                  aria-label={`Remove provider ${p}`}
                >
                  ×
                </button>
              </span>
            ))}
            <button
              type="button"
              className={styles.clearAll}
              onClick={clearAllFilters}
            >
              Clear all
            </button>
          </div>
        ) : null}
      </div>
      {isPending ? null : (
      <>
      <span className={cx('mono', styles.kicker)}>{LIST_KICKER}</span>
      <div className={styles.colHeader}>
        <span className={cx('mono', styles.cName)}>model</span>
        <span className={cx('mono', styles.cReasoning)}>reasoning</span>
        <span className={cx('mono', styles.cScore)}>intel</span>
        <span className={cx('mono', styles.cScore)}>cost</span>
        <span className={cx('mono', styles.cScore)}>speed</span>
        <span className={cx('mono', styles.cProviders)}>providers</span>
        <span className={styles.cChevron} />
      </div>
      {isError ? (
        <div className={styles.empty}>
          <span>{LOAD_ERROR}</span>
          <Button variant="ghost" size="xs" onClick={() => void refetch()}>
            Retry
          </Button>
        </div>
      ) : visible.length === 0 ? (
        <div className={styles.empty}>
          <EmptyState text={needle ? EMPTY_FILTER : EMPTY_CATALOG} />
        </div>
      ) : (
        visible.map((m) => (
          <div
            key={m.model_name}
            className={cx('row', styles.modelRow)}
            onClick={() => openDetail({ kind: 'model', id: m.model_name })}
          >
            <span className={styles.nameCell}>
              <span className={styles.name}>{m.model_name}</span>
              {m.model_id ? <span className={cx('mono', styles.id)}>{m.model_id}</span> : null}
            </span>
            <span className={styles.reasoning}>
              {(m.reasoning ?? []).map((level) => (
                <Tag key={level} variant="neutral" size="chip">
                  {level}
                </Tag>
              ))}
            </span>
            <span className={cx('mono', styles.score)}>{formatScore(m.intelligence)}</span>
            <span className={cx('mono', styles.score)}>{formatScore(m.cost)}</span>
            <span className={cx('mono', styles.score)}>{formatScore(m.speed)}</span>
            <span className={cx('mono', styles.providers)}>{m.provider_count}</span>
            <span className={styles.chevron} aria-hidden="true">
              <ChevronIcon />
            </span>
          </div>
        ))
      )}
      </>
      )}
    </div>
  )
}

export default ModelsPage
