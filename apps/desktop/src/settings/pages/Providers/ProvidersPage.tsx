// U10 — Providers settings page: the usage backend, then the priority-ordered
// provider list (drag to set fallback order, default-deny switch, live usage
// meters, route count) and a provider detail view where each model's reasoning
// levels route separately. Detail markup is the mockup's (demo.dc.html
// L756-783); its blurb is the mockup's own (L1191).
//
// Deliberate divergences from the mockup:
//   * List rows are the harness detail's provider CARDS (ProviderUsageRow),
//     not the mockup's one-line rows — they carry the live quota picture,
//     which is what this page is for.
//   * The usage backend selector lives here. The separate "Usage detection"
//     page was removed: its limits list duplicated this one, and the backend is
//     the only control worth keeping — now beside the limits it governs.
//   * An "Add provider" action exists (the mockup has none), offering
//     models.dev slugs from providers.addable() rather than free text.
//
// Layout contract (U07): <main> has no horizontal padding, so every block here
// carries its own 22px gutter — including the drag rows, which get theirs via
// DragList's `rowClassName` so the grab handle sits inside the gutter.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Button,
  Combobox,
  DragList,
  EmptyState,
  Input,
  Menu,
  ProviderUsageRow,
  SettingsModal,
  Tag,
  Toggle,
  cx,
  useToast,
} from '@which-model/ui'
import type { MenuItem } from '@which-model/ui'
import type { ProviderAccount, ProviderDetail, ProviderInfo } from '@which-model/core'
import { useModelScoreDetail, useProviderDetail, useProviders } from '../../../lib/queries'
import {
  useProvidersListStore,
  type EnabledFilter,
  type ProviderSort,
} from './listState'
import { getHost } from '../../../lib/host'
import { DetailHeader } from '../../DetailHeader'
import { PAGE_META } from '../../pages'
import type { Detail, PageComponentProps } from '../../pages'
import { ModelCard } from '../Models/ModelCard'
import styles from './ProvidersPage.module.css'

// demo.dc.html 735 — the kicker doubles as the drag affordance's label.
const LIST_KICKER = 'providers · drag to set fallback order'

function errText(e: unknown, fallback: string): string {
  return (e as { message?: string }).message ?? fallback
}


/** Device pages (GitHub) accept ?user_code= so the copied code is pre-filled. */
function deviceLoginURL(uri: string, userCode: string): string {
  try {
    const parsed = new URL(uri)
    if (userCode && !parsed.searchParams.has('user_code')) {
      parsed.searchParams.set('user_code', userCode)
    }
    return parsed.toString()
  } catch {
    return uri
  }
}

function openLoginLabel(uri: string): string {
  try {
    const parsed = new URL(uri)
    const host = parsed.hostname.replace(/^www\./, '')
    if (host === 'github.com' && parsed.pathname.startsWith('/login/device')) {
      return 'Open github.com/login/device'
    }
    return host ? `Open ${host}` : 'Open login page'
  } catch {
    return 'Open login page'
  }
}

function isCancelledSignIn(e: unknown): boolean {
  return errText(e, '').toLowerCase().includes('cancel')
}

/** Trash glyph (same path as U09's, mockup L455/L463) — 13px in the detail. */
function TrashIcon() {
  return (
    <svg
      width={13}
      height={13}
      viewBox="0 0 14 14"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.25"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M2.4 4h9.2M5.6 4V2.7h2.8V4M3.7 4l.45 7.3h5.7L10.3 4M5.9 6.2v3.2M8.1 6.2v3.2" />
    </svg>
  )
}

/** demo.dc.html 752 — the row-trailing chevron: a static affordance at 38%
 *  ink, deliberately not an `.ib` control. */
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


export function ProvidersPage({ detail, openDetail, closeDetail }: PageComponentProps) {
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
        backLabel={detail.fromProvider ?? 'Providers'}
        onBack={closeDetail}
        openDetail={openDetail}
      />
    )
  }
  return detail?.kind === 'provider' ? (
    <ProviderDetailView id={detail.id} onBack={closeDetail} openDetail={openDetail} />
  ) : (
    <ProvidersListView openDetail={openDetail} />
  )
}

// ——— provider list (mockup L734-755) —————————————————————————————————————

type Backend = 'off' | 'native' | 'codexbar'

function compareProviderIDs(left: ProviderInfo, right: ProviderInfo): number {
  return left.id < right.id ? -1 : left.id > right.id ? 1 : 0
}

function compareProviders(left: ProviderInfo, right: ProviderInfo, mode: ProviderSort): number {
  switch (mode) {
    case 'name-desc':
      return -compareProviderIDs(left, right)
    case 'models-desc':
      return right.models - left.models || compareProviderIDs(left, right)
    case 'models-asc':
      return left.models - right.models || compareProviderIDs(left, right)
    case 'enabled-first':
      return Number(right.enabled) - Number(left.enabled) || compareProviderIDs(left, right)
    case 'disabled-first':
      return Number(left.enabled) - Number(right.enabled) || compareProviderIDs(left, right)
    case 'priority':
      return left.priority - right.priority || compareProviderIDs(left, right)
    case 'name-asc':
      return compareProviderIDs(left, right)
  }
}


function ProvidersListView({ openDetail }: { openDetail(d: Detail): void }) {
  const toast = useToast()
  const { data: providers } = useProviders()
  const list = providers ?? []
  // List controls live in the module-level store (lib/listState) so they
  // survive the detail-view round-trip that unmounts this view (#142).
  const { query, setQuery, enabledFilter, setEnabledFilter, sortMode, setSortMode } =
    useProvidersListStore()


  // Usage backend — inherited from the removed Usage detection page.
  const [backend, setBackend] = useState<Backend | null>(null)
  useEffect(() => {
    void getHost()
      .usage.mode()
      .then((m) => setBackend(m.backend as Backend))
      .catch(() => {})
  }, [])

  const handleBackend = useCallback(
    (next: Backend) => {
      const prev = backend
      setBackend(next) // optimistic: the control must not lag the click
      void getHost()
        .usage.setBackend(next)
        .catch((e) => {
          setBackend(prev)
          toast.show(errText(e, 'update failed'))
        })
    },
    [backend, toast],
  )

  // Add custom provider — registers a custom provider id not in the default catalogue.
  const [adding, setAdding] = useState(false)
  const [customId, setCustomId] = useState('')

  const handleAdd = useCallback(
    (id: string) => {
      const trimmed = id.trim().toLowerCase()
      if (!trimmed || !/^[a-z0-9_-]+$/.test(trimmed)) {
        toast.show('invalid provider id — use lowercase letters, digits, - or _')
        return
      }
      void getHost()
        .providers.add(trimmed)
        .then(() => {
          toast.show(`added ${trimmed} — declare routes via config or which-model routes add`)
          setAdding(false)
          setCustomId('')
        })
        .catch((e) => toast.show(errText(e, 'add failed')))
    },
    [toast],
  )
  const handleReorder = useCallback(
    (ids: string[]) => {
      void getHost()
        .providers.reorder(ids)
        .catch((e) => toast.show(errText(e, 'reorder failed')))
    },
    [toast],
  )

  const handleToggle = useCallback(
    (id: string, on: boolean) => {
      void getHost()
        .providers.setEnabled(id, on)
        .catch((e) => toast.show(errText(e, 'update failed')))
    },
    [toast],
  )

  // Right-click context menu, positioned at the cursor.
  const [menu, setMenu] = useState<{ id: string; x: number; y: number } | null>(null)

  const move = useCallback(
    (id: string, to: 'up' | 'down' | 'top' | 'bottom') => {
      const ids = list.map((p) => p.id)
      const from = ids.indexOf(id)
      if (from < 0) return
      const target =
        to === 'up' ? from - 1 : to === 'down' ? from + 1 : to === 'top' ? 0 : ids.length - 1
      if (target < 0 || target >= ids.length || target === from) return
      const next = [...ids]
      next.splice(from, 1)
      next.splice(target, 0, id)
      handleReorder(next) // priorities are positions; Reorder rewrites them all
    },
    [list, handleReorder],
  )

  const runMenu = useCallback(
    (key: string, id: string) => {
      setMenu(null)
      const host = getHost()
      switch (key) {
        case 'edit':
          openDetail({ kind: 'provider', id })
          break
        case 'duplicate':
          void host.providers
            .duplicate(id)
            .then((copy) => toast.show(`duplicated to ${copy}`))
            .catch((e) => toast.show(errText(e, 'duplicate failed')))
          break
        case 'delete':
          void host.providers
            .delete(id)
            .then(() => toast.show(`deleted ${id}`))
            .catch((e) => toast.show(errText(e, 'delete failed')))
          break
        case 'up':
        case 'down':
        case 'top':
        case 'bottom':
          move(id, key)
          break
      }
    },
    [move, openDetail, toast],
  )

  const menuItems = useCallback(
    (id: string): MenuItem[] => {
      const index = list.findIndex((p) => p.id === id)
      const builtin = list[index]?.builtin ?? false
      const first = index <= 0
      const last = index === list.length - 1
      return [
        { key: 'edit', label: 'Edit…' },
        { key: 'duplicate', label: 'Duplicate' },
        // Builtins ship a usage adapter and stay in the universe whatever the
        // config says, so Delete would look like a no-op — dim it instead.
        { key: 'delete', label: builtin ? 'Delete (built-in)' : 'Delete', dim: builtin },
        { key: 'sep', separator: true },
        { key: 'up', label: 'Move up', dim: first },
        { key: 'down', label: 'Move down', dim: last },
        { key: 'top', label: 'Move to top', dim: first },
        { key: 'bottom', label: 'Move to bottom', dim: last },
      ]
    },
    [list],
  )
  const normalizedQuery = query.trim().toLowerCase()
  const visibleList = useMemo(
    () =>
      list
        .filter(
          (provider) =>
            provider.id.toLowerCase().includes(normalizedQuery) &&
            (enabledFilter === 'all' ||
              (enabledFilter === 'enabled' ? provider.enabled : !provider.enabled)),
        )
        .sort((left, right) => compareProviders(left, right, sortMode)),
    [enabledFilter, list, normalizedQuery, sortMode],
  )
  const canReorder =
    sortMode === 'priority' && enabledFilter === 'all' && normalizedQuery.length === 0
  const listKicker = canReorder
    ? LIST_KICKER
    : `${visibleList.length} of ${list.length} provider${list.length === 1 ? '' : 's'}`
  const providerItems = visibleList.map((provider) => ({
    id: provider.id,
    node: (
      <span
        className={styles.card}
        data-provider-id={provider.id}
        data-model-count={provider.models}
        onClick={() => openDetail({ kind: 'provider', id: provider.id })}
        onContextMenu={(e) => {
          e.preventDefault()
          setMenu({ id: provider.id, x: e.clientX, y: e.clientY })
        }}
      >
        <ProviderUsageRow
          provider={provider}
          on={provider.enabled}
          onToggle={(on) => handleToggle(provider.id, on)}
          live={provider.enabled}
          offLabel="not enabled"
          leading={<span className={cx('mono', styles.order)}>{provider.priority}</span>}
          trailing={
            <span
              className={styles.openCell}
              title={`${provider.models} models; ${provider.routes_on} of ${provider.routes_total} routes enabled`}
            >
              <span className={cx('mono', styles.routes)}>
                {provider.models} {provider.models === 1 ? 'model' : 'models'}
              </span>
              <span className={styles.chevron}>
                <ChevronIcon />
              </span>
            </span>
          }
        />
      </span>
    ),
  }))




  return (
    <div className={styles.page}>
      <DetailHeader
        title={PAGE_META.Providers[0]}
        blurb={PAGE_META.Providers[1]}
        action={{
          label: PAGE_META.Providers[2] as string,
          onAction: () => setAdding((v) => !v),
        }}
      />

      <div className={styles.backendRow}>
        <span className={styles.backendLabel}>
          <span className={styles.backendTitle}>Usage detection</span>
          <span className={styles.backendNote}>
            Where remaining quota is read from before a pick.
          </span>
        </span>
        <select
          className="wmsel"
          aria-label="Usage detection"
          value={backend ?? 'native'}
          onChange={(event) => handleBackend(event.target.value as Backend)}
        >
          <option value="off">Off</option>
          <option value="native">Native</option>
          <option value="codexbar">CodexBar</option>
        </select>
      </div>

      {adding ? (
        <div className={styles.addRow}>
          <span className={styles.addId}>
            <Input
              value={customId}
              onChange={setCustomId}
              placeholder="custom-provider-id"
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  handleAdd(e.currentTarget.value || customId)
                }
              }}
            />
          </span>
          <Button
            variant="primary"
            size="sm"
            onClick={() => handleAdd(customId)}
          >
            Add
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              setAdding(false)
              setCustomId('')
            }}
          >
            Cancel
          </Button>
          <span className={styles.addNote}>
            declare routes via config or `which-model routes add`
          </span>
        </div>
      ) : null}

      <div className={styles.controls}>
        <span className={styles.searchInput}>
          <Input
            type="search"
            value={query}
            onChange={setQuery}
            placeholder="Search providers"
            aria-label="Search providers"
          />
        </span>
        <select
          className="wmsel"
          aria-label="Filter providers"
          value={enabledFilter}
          onChange={(event) => setEnabledFilter(event.target.value as EnabledFilter)}
        >
          <option value="all">All providers</option>
          <option value="enabled">Enabled</option>
          <option value="disabled">Disabled</option>
        </select>
        <label className={styles.sortControl}>
          <span className={styles.sortLabel}>Sort</span>
          <select
            className="wmsel"
            aria-label="Sort providers"
            value={sortMode}
            onChange={(event) => setSortMode(event.target.value as ProviderSort)}
          >
            <option value="name-asc">Name A–Z</option>
            <option value="name-desc">Name Z–A</option>
            <option value="models-desc">Models high–low</option>
            <option value="models-asc">Models low–high</option>
            <option value="enabled-first">Enabled first</option>
            <option value="disabled-first">Disabled first</option>
            <option value="priority">Priority (drag)</option>
          </select>
        </label>
      </div>


      <span className={cx('mono', styles.kicker)}>{listKicker}</span>

      {canReorder ? (
        <DragList
          rowClassName={styles.dragRow}
          onReorder={handleReorder}
          items={providerItems}
        />
      ) : (
        <div className={styles.staticList}>
          {providerItems.map((item) => (
            <div className={cx(styles.dragRow, styles.staticRow)} key={item.id}>
              {item.node}
            </div>
          ))}
        </div>
      )}

      {/* The list is DERIVED (internal/service providerUniverseLocked unions
          [providers.*] config keys, every registered usage provider, and every
          provider in the route table), so empty means the route table and the
          catalogue are both missing — say so rather than showing nothing. */}
      {list.length === 0 ? (
        <div className={styles.empty}>
          <EmptyState text="No providers yet. Run `which-model routes refresh` in a terminal to build the route table from the model catalogue." />
        </div>
      ) : visibleList.length === 0 ? (
        <div className={styles.empty}>
          <EmptyState text="No providers match these filters." />
        </div>
      ) : null}

      {menu ? (
        <Menu
          className={styles.contextMenu}
          style={{ left: menu.x, top: menu.y }}
          items={menuItems(menu.id)}
          onPick={(key) => runMenu(key, menu.id)}
          onClose={() => setMenu(null)}
        />
      ) : null}
    </div>
  )
}

// ——— provider detail (mockup L756-783) ———————————————————————————————————

/** demo.dc.html 1191 — the detail view's blurb, interpolated on the id. */
function detailBlurb(id: string): string {
  return `Models ${id} can serve. Each reasoning level routes separately — switch off the ones the picker should not consider. Click a level to see its benchmarks.`
}

function countRoutes(detail: ProviderDetail): { on: number; total: number } {
  let on = 0
  let total = 0
  for (const m of detail.models) {
    for (const l of m.levels ?? []) {
      total += 1
      if (l.enabled) on += 1
    }
  }
  return { on, total }
}

function isSignedIn(accounts: readonly ProviderAccount[] | undefined): boolean {
  return (accounts ?? []).some((a) => a.ref.trim().length > 0)
}

function ProviderDetailView({
  id,
  onBack,
  openDetail,
}: {
  id: string
  onBack(): void
  openDetail(d: Detail): void
}) {
  const toast = useToast()
  const { data: detail } = useProviderDetail(id)
  const [refreshing, setRefreshing] = useState(false)
  const [authenticated, setAuthenticated] = useState(false)

  const handleRoute = useCallback(
    (modelId: string, reasoning: string, on: boolean) => {
      void getHost()
        .providers.setRouteEnabled(id, modelId, reasoning, on)
        .catch((e) => toast.show(errText(e, 'update failed')))
    },
    [id, toast],
  )

  // Whole-provider switch (host has a dedicated call, L1472-1473).
  const handleAll = useCallback(
    (on: boolean) => {
      void getHost()
        .providers.setAllRoutes(id, on)
        .catch((e) => toast.show(errText(e, 'update failed')))
    },
    [id, toast],
  )

  const handleRefreshData = useCallback(() => {
    setRefreshing(true)
    void getHost()
      .providers.refreshRoutes()
      .catch((e) => toast.show(errText(e, 'could not refresh data')))
      .finally(() => setRefreshing(false))
  }, [toast])

  // Per-model switch (L771) — no host call covers one model, so its levels are
  // written one at a time; the first failure reports and the rest still run.
  const handleModelAll = useCallback(
    (modelId: string, levels: readonly { reasoning: string }[], on: boolean) => {
      void Promise.allSettled(
        levels.map((l) => getHost().providers.setRouteEnabled(id, modelId, l.reasoning, on)),
      ).then((rs) => {
        const bad = rs.find((r) => r.status === 'rejected')
        if (bad && bad.status === 'rejected') toast.show(errText(bad.reason, 'update failed'))
      })
    },
    [id, toast],
  )

  if (!detail) return <div className={cx(styles.page, styles.loading)}>loading…</div>

  const { on, total } = countRoutes(detail)
  const signedIn = authenticated || isSignedIn(detail.accounts)

  return (
    <div className={styles.page}>
      <DetailHeader title={id} blurb={detailBlurb(id)} backLabel="Providers" onBack={onBack} />

      {/* L757-763 — the enable/disable pair lives here, not as the header
          action button: the mockup gives this page no page action. */}
      <div className={styles.summary}>
        <span className={cx('mono', styles.summaryText)}>{`${on} of ${total} routes enabled`}</span>
        <span className={styles.summaryActions}>
          {signedIn ? (
            <Button variant="ghost" size="xs" disabled={refreshing} onClick={handleRefreshData}>
              {refreshing ? 'Refreshing…' : 'Refresh data'}
            </Button>
          ) : null}
          <Button variant="ghost" size="xs" onClick={() => handleAll(true)}>
            Enable all
          </Button>
          <Button variant="ghost" size="xs" className={styles.muted} onClick={() => handleAll(false)}>
            Disable all
          </Button>
          {/* The list hides Delete behind the right-click menu; the detail
              view is where a provider is inspected, so it carries the visible
              delete. Builtins ship a usage adapter and stay in the universe
              whatever the config says — dim and disable rather than fail. */}
          <button
            type="button"
            className={cx('ib', detail.builtin && 'off', styles.iconBtnLg)}
            title={detail.builtin ? 'Built-in provider — cannot be deleted' : `Delete ${id}`}
            disabled={detail.builtin}
            onClick={() =>
              void getHost()
                .providers.delete(id)
                .then(() => {
                  toast.show(`deleted ${id}`)
                  onBack()
                })
                .catch((e) => toast.show(errText(e, 'delete failed')))
            }
          >
            <TrashIcon />
          </button>
        </span>
      </div>

      <AccountsSection
        id={id}
        accounts={detail.accounts}
        oauthSupported={detail.oauth_supported}
        onAuthenticated={() => setAuthenticated(true)}
        onError={(m) => toast.show(m)}
      />

      {detail.models.length === 0 ? (
        <div className={styles.empty}>
          <EmptyState
            text={
              signedIn
                ? 'No models yet. Refresh data to build them from the catalogue.'
                : 'No models for this provider yet. Add an account, then refresh models.'
            }
          />
        </div>
      ) : null}

      {detail.models.map((m) => {
        const levels = m.levels ?? []
        const allOn = levels.length > 0 && levels.every((l) => l.enabled)
        return (
          // L765
          <div key={m.model_id} className={styles.model}>
            <span className={styles.modelMeta}>
              <button
                type="button"
                className={styles.modelIdentity}
                onClick={() =>
                  openDetail({ kind: 'model', id: m.model_name, fromProvider: id })
                }
              >
                <span className={styles.modelName}>{m.model_name}</span>
                <span className={cx('mono', styles.modelId)}>{m.model_id}</span>
              </button>
              {levels.length > 0 ? (
                <Button
                  variant="ghost"
                  size="xs"
                  className={styles.modelAll}
                  onClick={() => handleModelAll(m.model_id, levels, !allOn)}
                >
                  {allOn ? 'Disable all' : 'Enable all'}
                </Button>
              ) : null}
            </span>
            <span className={styles.levels}>
              {levels.map((l) => (
                <span key={l.reasoning} className={styles.level}>
                  <Toggle
                    on={l.enabled}
                    onToggle={(next: boolean) => handleRoute(m.model_id, l.reasoning, next)}
                  />
                  <button
                    type="button"
                    className={cx('mono', styles.levelName, !l.enabled && styles.levelOff)}
                    onClick={() =>
                      openDetail({
                        kind: 'provider-model',
                        provider: id,
                        modelName: m.model_name,
                        reasoning: l.reasoning,
                      })
                    }
                  >
                    {l.reasoning}
                  </button>
                  {l.default ? <Tag variant="neutral">default</Tag> : null}
                </span>
              ))}
            </span>
          </div>
        )
      })}
    </div>
  )
}

// ——— accounts ————————————————————————————————————————————————————————————

type AccountMethod = 'oauth' | 'api_key'

function accountSource(account: ProviderAccount, authenticated: boolean): string {
  if (account.kind === 'oauth') {
    return authenticated || Boolean(account.ref)
      ? 'OAuth · signed in'
      : 'OAuth · not signed in'
  }
  if (account.kind === 'token') {
    return account.ref === 'which-model'
      ? 'API key · securely stored'
      : `API key · ${account.ref}`
  }
  return account.ref ? `Browser cookie · ${account.ref}` : 'Browser cookie'
}

/**
 * Provider accounts are created through an explicit authentication flow.
 * Secrets are sent once to the service layer and never copied into provider
 * state, query data, local storage, or config.toml.
 */
function AccountsSection({
  id,
  accounts,
  oauthSupported,
  onAuthenticated,
  onError,
}: {
  id: string
  accounts: ProviderAccount[]
  oauthSupported: boolean
  onAuthenticated(): void
  onError(message: string): void
}) {
  const [pendingAccounts, setPendingAccounts] = useState<ProviderAccount[] | null>(null)
  const commitSeq = useRef(0)
  const saveChain = useRef<Promise<void>>(Promise.resolve())
  const pendingWrites = useRef(0)
  const [removing, setRemoving] = useState(false)
  const credentialWrite = useRef(false)
  const rows = pendingAccounts ?? accounts
  const [addOpen, setAddOpen] = useState(false)
  const [accountName, setAccountName] = useState('')
  const [method, setMethod] = useState<AccountMethod>(oauthSupported ? 'oauth' : 'api_key')
  const [apiKey, setAPIKey] = useState('')
  const [saving, setSaving] = useState(false)
  const [authenticatedAccounts, setAuthenticatedAccounts] = useState<Set<string>>(
    () => new Set(),
  )
  const [signIn, setSignIn] = useState<null | {
    provider: string
    account: string
    flowId: string
    uri: string
    code: string
    pasteRequired: boolean
  }>(null)
  const [pastedCode, setPastedCode] = useState('')
  const signInLock = useRef<{ cancelled: boolean; flowId: string | null } | null>(null)
  const cancelActiveSignIn = useCallback(() => {
    const lock = signInLock.current
    if (!lock) return
    lock.cancelled = true
    signInLock.current = null
    if (lock.flowId) {
      void getHost().signin.cancel(id, lock.flowId).catch(() => {})
    }
  }, [id])
  useEffect(() => cancelActiveSignIn, [cancelActiveSignIn])

  const replaceAccounts = useCallback(
    (next: ProviderAccount[]) => {
      if (signInLock.current || credentialWrite.current) return
      pendingWrites.current++
      setRemoving(true)
      const seq = ++commitSeq.current
      setPendingAccounts(next)
      saveChain.current = saveChain.current
        .then(() => getHost().providers.setAccounts(id, next))
        .then(() => {
          if (commitSeq.current === seq) {
            setPendingAccounts(null)
          }
        })
        .catch((error) => {
          if (commitSeq.current === seq) {
            setPendingAccounts(null)
          }
          onError(errText(error, 'could not save accounts'))
        })
        .finally(() => {
          pendingWrites.current--
          if (pendingWrites.current === 0) setRemoving(false)
        })
    },
    [id, onError],
  )

  const closeAdd = useCallback(() => {
    setAddOpen(false)
    setAccountName('')
    setMethod(oauthSupported ? 'oauth' : 'api_key')
    setAPIKey('')
    setSaving(false)
  }, [oauthSupported])

  // OAuth sign-in: device-code providers copy the code and poll; Claude
  // opens the authorize URL and waits for a pasted code via submitCode.
  const onSignIn = (name: string) => {
    if (signInLock.current || pendingWrites.current || credentialWrite.current) return
    const lock = { cancelled: false, flowId: null as string | null }
    signInLock.current = lock
    setPastedCode('')
    void (async () => {
      try {
        const started = await getHost().signin.start(id)
        lock.flowId = started.flow_id
        if (lock.cancelled || signInLock.current !== lock) {
          void getHost().signin.cancel(id, started.flow_id).catch(() => {})
          return
        }
        if (started.user_code) {
          void getHost().window.copyToClipboard(started.user_code)
          void getHost().window.openURL(
            deviceLoginURL(started.verification_uri, started.user_code),
          )
        } else {
          void getHost().window.openURL(started.verification_uri)
        }
        setSignIn({
          provider: id,
          account: name,
          flowId: started.flow_id,
          uri: started.verification_uri,
          code: started.user_code,
          pasteRequired: started.paste_required,
        })
        await getHost().signin.confirm(id, started.flow_id, name)
        if (lock.cancelled || signInLock.current !== lock) return
        signInLock.current = null
        setSignIn(null)
        setAuthenticatedAccounts((current) => new Set(current).add(name))
        onAuthenticated()
      } catch (error) {
        if (
          lock.cancelled ||
          signInLock.current !== lock ||
          isCancelledSignIn(error)
        ) {
          if (signInLock.current === lock) {
            signInLock.current = null
            setSignIn(null)
          }
          return
        }
        if (signInLock.current === lock) signInLock.current = null
        setSignIn(null)
        onError(errText(error, 'sign-in failed'))
      }
    })()
  }

  const submitAccount = async () => {
    if (pendingWrites.current || credentialWrite.current || signInLock.current) return
    const name = accountName.trim()
    if (!name) {
      onError('Enter an account name')
      return
    }
    if (method === 'oauth') {
      closeAdd()
      onSignIn(name)
      return
    }
    const key = apiKey.trim()
    if (!key) {
      onError('Enter an API key')
      return
    }
    credentialWrite.current = true
    setSaving(true)
    try {
      await getHost().signin.saveAPIKey(id, name, key)
      onAuthenticated()
      closeAdd()
    } catch (error) {
      setSaving(false)
      onError(errText(error, 'could not save API key'))
    } finally {
      credentialWrite.current = false
    }
  }

  const onCancelSignIn = useCallback(() => {
    cancelActiveSignIn()
    setSignIn(null)
  }, [cancelActiveSignIn])

  return (
    <section className={styles.accounts}>
      <div className={styles.accountsHead}>
        <span>
          <span className={styles.accountsLabel}>Accounts</span>
          <span className={styles.accountsNote}>Authentication for this provider</span>
        </span>
        <Button
          variant="primary"
          size="xs"
          disabled={removing || saving || !!signIn}
          onClick={() => {
            if (pendingWrites.current || credentialWrite.current || signInLock.current) return
            setMethod(oauthSupported ? 'oauth' : 'api_key')
            setAddOpen(true)
          }}
        >
          Add account
        </Button>
      </div>

      {rows.length === 0 ? (
        <div className={styles.accountEmpty}>
          No accounts yet. Add an account to authenticate this provider.
        </div>
      ) : (
        <div className={styles.accountList}>
          {rows.map((account, index) => (
            <div key={`${account.name}-${account.kind}`} className={styles.accountCard}>
              <span className={styles.accountIdentity}>
                <span className={styles.accountName}>{account.name}</span>
                <span className={cx('mono', styles.accountSource)}>
                  {accountSource(account, authenticatedAccounts.has(account.name))}
                </span>
              </span>
              <span className={styles.accountActions}>
                {account.kind === 'oauth' && oauthSupported ? (
                  <Button
                    variant="secondary"
                    size="xs"
                    disabled={removing || saving || !!signIn}
                    onClick={() => onSignIn(account.name)}
                  >
                    {account.ref || authenticatedAccounts.has(account.name)
                      ? 'Re-authenticate'
                      : 'Sign in…'}
                  </Button>
                ) : null}
                <button
                  type="button"
                  className={cx('ib', styles.accountDelete)}
                  title={`Remove ${account.name}`}
                  disabled={saving || !!signIn}
                  onClick={() =>
                    replaceAccounts(rows.filter((_, rowIndex) => rowIndex !== index))
                  }
                >
                  <TrashIcon />
                </button>
              </span>
            </div>
          ))}
        </div>
      )}

      <SettingsModal
        open={addOpen}
        title="Add account"
        description="Choose how this provider should authenticate."
        onClose={closeAdd}
        actions={
          <>
            <Button variant="secondary" size="xs" onClick={closeAdd}>
              Cancel
            </Button>
            <Button
              variant="primary"
              size="xs"
              disabled={removing || saving || !!signIn || !accountName.trim() || (method === 'api_key' && !apiKey.trim())}
              onClick={() => void submitAccount()}
            >
              {saving ? 'Saving…' : method === 'oauth' ? 'Sign in with OAuth' : 'Save account'}
            </Button>
          </>
        }
      >
        <div className={styles.accountForm}>
          <label className={styles.accountField}>
            <span>Account name</span>
            <Input
              value={accountName}
              mono={false}
              aria-label="Account name"
              placeholder="Production"
              onChange={setAccountName}
            />
          </label>
          <label className={styles.accountField}>
            <span>Authentication method</span>
            <select
              className="wmsel"
              aria-label="Authentication method"
              value={method}
              onChange={(event) => setMethod(event.target.value as AccountMethod)}
            >
              {oauthSupported ? <option value="oauth">OAuth</option> : null}
              <option value="api_key">API key</option>
            </select>
          </label>
          {method === 'api_key' ? (
            <label className={styles.accountField}>
              <span>API key</span>
              <Input
                value={apiKey}
                type="password"
                aria-label="API key"
                placeholder="Paste API key"
                onChange={setAPIKey}
                onKeyDown={(event) => {
                  if (event.key === 'Enter' && accountName.trim() && apiKey.trim()) {
                    void submitAccount()
                  }
                }}
              />
              <span className={styles.accountHint}>
                Stored in the system keychain when available, otherwise in a private local file.
              </span>
            </label>
          ) : null}
        </div>
      </SettingsModal>

      {signIn && (
        <SettingsModal
          open
          title={`Sign in to ${signIn.account}`}
          description={
            signIn.code
              ? 'Your code is copied and the login page should open in your browser. This closes when the provider confirms.'
              : signIn.pasteRequired
                ? 'A browser window opened. After you authorize, paste the code from the page and continue.'
                : 'A browser window opened. This closes automatically after authorization.'
          }
          onClose={onCancelSignIn}
          closeOnBackdrop={false}
          actions={
            <>
              <Button
                variant="secondary"
                size="xs"
                onClick={() => {
                  if (signIn.uri) {
                    void getHost().window.openURL(
                      deviceLoginURL(signIn.uri, signIn.code ?? ''),
                    )
                  }
                }}
              >
                {openLoginLabel(signIn.uri ?? '')}
              </Button>
              {signIn.pasteRequired ? (
                <Button
                  variant="primary"
                  size="xs"
                  disabled={!pastedCode.trim()}
                  onClick={() => {
                    void getHost()
                      .signin.submitCode(id, signIn.flowId, pastedCode.trim())
                      .catch((error) => {
                        onError(errText(error, 'could not submit code'))
                      })
                  }}
                >
                  Continue
                </Button>
              ) : null}
              <Button variant="secondary" size="xs" onClick={onCancelSignIn}>
                Cancel
              </Button>
            </>
          }
        >
          {signIn.code ? (
            <button
              type="button"
              className={cx('mono', styles.deviceCode)}
              title="Copied to clipboard — click to copy again"
              onClick={() => void getHost().window.copyToClipboard(signIn.code)}
            >
              {signIn.code}
            </button>
          ) : signIn.pasteRequired ? (
            <Input
              value={pastedCode}
              mono
              placeholder="Paste the code from the page"
              onChange={setPastedCode}
              onKeyDown={(event) => {
                if (event.key === 'Enter' && pastedCode.trim()) {
                  void getHost()
                    .signin.submitCode(id, signIn.flowId, pastedCode.trim())
                    .catch((error) => {
                      onError(errText(error, 'could not submit code'))
                    })
                }
              }}
            />
          ) : (
            <span className={styles.accountHint}>Complete sign-in in the opened browser.</span>
          )}
        </SettingsModal>
      )}
    </section>
  )
}

type BenchSortKey = 'name' | 'value' | 'score'

export function ModelBenchmarksView({
  provider,
  modelName,
  reasoning,
  onBack,
}: {
  provider: string
  modelName: string
  reasoning: string
  onBack(): void
}) {
  const { data: detail } = useModelScoreDetail(modelName, reasoning)
  const [sort, setSort] = useState<{ key: BenchSortKey; dir: 'asc' | 'desc' }>({
    key: 'score',
    dir: 'desc',
  })
  const sortedRows = useMemo(() => {
    if (!detail) return []
    const sign = sort.dir === 'desc' ? -1 : 1
    return [...detail.rows].sort((a, b) => {
      if (sort.key === 'name') return sign * a.name.localeCompare(b.name)
      if (sort.key === 'score') return sign * (a.norm - b.norm)
      return sign * (a.value - b.value)
    })
  }, [detail, sort])

  if (!detail) return <div className={cx(styles.page, styles.loading)}>loading…</div>

  return (
    <div className={styles.page}>
      <DetailHeader
        title={`${detail.model}  (${detail.reasoning})`}
        blurb={`Benchmarks this model reports at the ${detail.reasoning} reasoning level.`}
        backLabel={provider}
        onBack={onBack}
      />
      <div className={styles.testedHead}>
        <span className={styles.testedLabel}>benchmarks</span>
        <span className={cx('mono', styles.testedCount)}>
          {sortedRows.length === 0 ? 'none yet' : `${sortedRows.length} tested`}
        </span>
      </div>
      <div className={styles.sortRow}>
        {(
          [
            { key: 'name', label: 'benchmark', cls: styles.sortName },
            { key: 'value', label: 'result', cls: styles.sortValue },
            { key: 'score', label: 'normalised score', cls: styles.sortScore },
          ] as const
        ).map((c) => {
          const active = sort.key === c.key
          return (
            <button
              key={c.key}
              type="button"
              className={cx('mono', styles.sortCol, c.cls, active && styles.sortColActive)}
              onClick={() =>
                setSort({ key: c.key, dir: active && sort.dir === 'desc' ? 'asc' : 'desc' })
              }
            >
              {c.label + (active ? (sort.dir === 'desc' ? '  ↓' : '  ↑') : '')}
            </button>
          )
        })}
      </div>
      <div className={cx('scroll', styles.benchList)}>
        {sortedRows.map((r) => {
          const score = Math.round(r.norm)
          return (
            <span key={r.name} className={styles.benchRow}>
              <span className={styles.benchLabel}>{r.name}</span>
              <span className={cx('mono', styles.benchValue)}>{r.value.toFixed(1)}</span>
              <span className={cx('mono', styles.benchScore)}>{score}</span>
              <span className={styles.benchBar}>
                <b
                  className={styles.benchBarFill}
                  style={{ width: `${Math.max(0, Math.min(100, score))}%` }}
                />
              </span>
            </span>
          )
        })}
        {sortedRows.length === 0 ? (
          <span className={cx('mono', styles.benchEmpty)}>
            No benchmarks for this model and reasoning level yet.
          </span>
        ) : null}
      </div>
    </div>
  )
}

export default ProvidersPage
