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
import { useCallback, useEffect, useState } from 'react'
import {
  Button,
  Combobox,
  DragList,
  EmptyState,
  Input,
  Menu,
  ProviderUsageRow,
  SegmentedControl,
  Tag,
  Toggle,
  cx,
  useToast,
} from '@which-model/ui'
import type { MenuItem } from '@which-model/ui'
import type { ProviderAccount, ProviderDetail, ProviderInfo } from '@which-model/core'
import { useProviderDetail, useProviders } from '../../../lib/queries'
import { getHost } from '../../../lib/host'
import { DetailHeader } from '../../DetailHeader'
import { PAGE_META } from '../../pages'
import type { Detail, PageComponentProps } from '../../pages'
import styles from './ProvidersPage.module.css'

// demo.dc.html 735 — the kicker doubles as the drag affordance's label.
const LIST_KICKER = 'providers · drag to set fallback order'

function errText(e: unknown, fallback: string): string {
  return (e as { message?: string }).message ?? fallback
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
  return detail?.kind === 'provider' ? (
    <ProviderDetailView id={detail.id} onBack={closeDetail} />
  ) : (
    <ProvidersListView openDetail={openDetail} />
  )
}

// ——— provider list (mockup L734-755) —————————————————————————————————————

type Backend = 'off' | 'native' | 'codexbar'

function ProvidersListView({ openDetail }: { openDetail(d: Detail): void }) {
  const toast = useToast()
  const { data: providers } = useProviders()
  const list = providers ?? []

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

  // Add provider — ids come from the catalogue, never free text: only a
  // models.dev slug can ever acquire routes.
  const [adding, setAdding] = useState(false)
  const [query, setQuery] = useState('')
  const [open, setOpen] = useState(false)
  const [addable, setAddable] = useState<string[]>([])

  useEffect(() => {
    if (!adding) return
    void getHost()
      .providers.addable()
      .then(setAddable)
      .catch(() => setAddable([]))
  }, [adding, list.length])

  const handleAdd = useCallback(
    (id: string) => {
      void getHost()
        .providers.add(id)
        .then(() => {
          toast.show(`added ${id} — run \`which-model routes refresh\` for its routes`)
          setAdding(false)
          setQuery('')
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

  const q = query.trim().toLowerCase()
  const matches = addable.filter((id) => !q || id.includes(q)).slice(0, 6)

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
        <SegmentedControl
          className={styles.seg}
          options={[
            { value: 'off', label: 'off' },
            { value: 'native', label: 'native' },
            { value: 'codexbar', label: 'codexbar' },
          ]}
          value={backend ?? 'native'}
          onChange={(v) => handleBackend(v as Backend)}
        />
      </div>

      {adding ? (
        <div className={styles.addRow}>
          <span className={styles.addId}>
            <Combobox
              items={matches.map((id) => ({ key: id, label: id, sub: 'models.dev' }))}
              query={query}
              onQuery={(v) => {
                setQuery(v)
                setOpen(true)
              }}
              open={open}
              onOpenChange={setOpen}
              onPick={handleAdd}
              emptyText={
                addable.length === 0
                  ? 'no catalogue yet — run `which-model routes refresh`'
                  : 'no provider by that name'
              }
              placeholder="search the model catalogue…"
            />
          </span>
          <Button variant="ghost" size="sm" onClick={() => setAdding(false)}>
            Cancel
          </Button>
          <span className={styles.addNote}>
            routes are built on the next `which-model routes refresh`
          </span>
        </div>
      ) : null}

      <span className={cx('mono', styles.kicker)}>{LIST_KICKER}</span>

      <DragList
        rowClassName={styles.dragRow}
        onReorder={handleReorder}
        items={list.map((p: ProviderInfo, i) => ({
          id: p.id,
          node: (
            <span
              className={styles.card}
              onContextMenu={(e) => {
                e.preventDefault()
                setMenu({ id: p.id, x: e.clientX, y: e.clientY })
              }}
            >
              <ProviderUsageRow
                provider={p}
                on={p.enabled}
                onToggle={(on) => handleToggle(p.id, on)}
                live={p.enabled}
                offLabel="not enabled"
                leading={<span className={cx('mono', styles.order)}>{i + 1}</span>}
                trailing={
                  <span
                    className={styles.openCell}
                    title={`${p.routes_on} of ${p.routes_total} routes enabled`}
                    onClick={() => openDetail({ kind: 'provider', id: p.id })}
                  >
                    <span className={styles.chevron}>
                      <ChevronIcon />
                    </span>
                  </span>
                }
              />
            </span>
          ),
        }))}
      />

      {/* The list is DERIVED (internal/service providerUniverseLocked unions
          [providers.*] config keys, every registered usage provider, and every
          provider in the route table), so empty means the route table and the
          catalogue are both missing — say so rather than showing nothing. */}
      {list.length === 0 ? (
        <div className={styles.empty}>
          <EmptyState text="No providers yet. Run `which-model routes refresh` in a terminal to build the route table from the model catalogue." />
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
  return `Models ${id} can serve. Each reasoning level routes separately — switch off the ones the picker should not consider.`
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

function ProviderDetailView({ id, onBack }: { id: string; onBack(): void }) {
  const toast = useToast()
  const { data: detail } = useProviderDetail(id)

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

  return (
    <div className={styles.page}>
      <DetailHeader title={id} blurb={detailBlurb(id)} backLabel="Providers" onBack={onBack} />

      {/* L757-763 — the enable/disable pair lives here, not as the header
          action button: the mockup gives this page no page action. */}
      <div className={styles.summary}>
        <span className={cx('mono', styles.summaryText)}>{`${on} of ${total} routes enabled`}</span>
        <span className={styles.summaryActions}>
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
        onError={(m) => toast.show(m)}
      />

      {detail.models.length === 0 ? (
        <div className={styles.empty}>
          <EmptyState text="No routes for this provider yet. Run `which-model routes refresh` to build them from the model catalogue." />
        </div>
      ) : null}

      {detail.models.map((m) => {
        // A model with no levels is available from the provider but produced
        // no routes (no benchmark row matches it) — show it, name it, and
        // mark it unrouted; there is nothing to toggle.
        const levels = m.levels ?? []
        const allOn = levels.length > 0 && levels.every((l) => l.enabled)
        return (
          // L765
          <div key={m.model_id} className={styles.model}>
            <span className={styles.modelMeta}>
              <span className={styles.modelName}>{m.model_name}</span>
              <span className={cx('mono', styles.modelId)}>{m.model_id}</span>
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
              {levels.length > 0 ? (
                levels.map((l) => (
                  // L773
                  <span key={l.reasoning} className={styles.level}>
                    <Toggle
                      on={l.enabled}
                      onToggle={(next: boolean) => handleRoute(m.model_id, l.reasoning, next)}
                    />
                    <span className={cx('mono', styles.levelName, !l.enabled && styles.levelOff)}>
                      {l.reasoning}
                    </span>
                    {l.default ? <Tag variant="neutral">default</Tag> : null}
                  </span>
                ))
              ) : (
                <span className={cx('mono', styles.levelName, styles.levelOff)}>no routes</span>
              )}
            </span>
          </div>
        )
      })}
    </div>
  )
}

// ——— accounts ————————————————————————————————————————————————————————————

const ACCOUNT_KINDS: ReadonlyArray<ProviderAccount['kind']> = ['oauth', 'cookie', 'token']

/** What `ref` means per kind — the field holds a REFERENCE, never a secret. */
const REF_PLACEHOLDER: Record<ProviderAccount['kind'], string> = {
  // oauth never renders an input (it gets a sign-in button), but the map stays
  // total so a new kind cannot be added without deciding its placeholder.
  oauth: 'credentials file or keychain service',
  cookie: 'cookie file path',
  token: 'environment variable name',
}

/**
 * A provider's accounts: name + the credential it links to.
 *
 * The whole list is written at once (providers.setAccounts) so add, rename,
 * re-kind and remove are one atomic config write — the UI never half-applies.
 */
function AccountsSection({
  id,
  accounts,
  onError,
}: {
  id: string
  accounts: ProviderAccount[]
  onError(message: string): void
}) {
  const [draft, setDraft] = useState<ProviderAccount[] | null>(null)
  const rows = draft ?? accounts

  const commit = useCallback(
    (next: ProviderAccount[]) => {
      setDraft(next) // optimistic; cleared when the query refetches
      void getHost()
        .providers.setAccounts(id, next)
        .then(() => setDraft(null))
        .catch((e) => {
          setDraft(null)
          onError(errText(e, 'could not save accounts'))
        })
    },
    [id, onError],
  )

  const update = (i: number, patch: Partial<ProviderAccount>) =>
    commit(rows.map((a, n) => (n === i ? { ...a, ...patch } : a)))

  // OAuth sign-in is not implemented host-side yet: there is no per-account
  // auth flow, and the usage adapters still resolve credentials through their
  // own descriptor chain. Rather than render a button that silently does
  // nothing, say so and point at the file the credential is read from.
  const onSignIn = (i: number) => {
    onError(
      `sign-in for ${rows[i]?.name ?? 'this account'} is not wired up yet — point it at an existing credential file for now`,
    )
  }

  return (
    <section className={styles.accounts}>
      <div className={styles.accountsHead}>
        <span className={styles.accountsLabel}>accounts</span>
        <span className={cx('mono', styles.accountsNote)}>
          {rows.length === 0 ? 'none' : `${rows.length} configured`}
        </span>
        <span className={styles.accountsActions}>
          <Button
            variant="ghost"
            size="xs"
            onClick={() =>
              commit([...rows, { name: `Account ${rows.length + 1}`, kind: 'oauth', ref: '' }])
            }
          >
            Add account
          </Button>
        </span>
      </div>

      {rows.map((account, i) => (
        <div key={i} className={styles.accountRow}>
          <span className={styles.accountName}>
            <Input
              value={account.name}
              mono={false}
              placeholder="name"
              onChange={(v) => setDraft(rows.map((a, n) => (n === i ? { ...a, name: v } : a)))}
              onBlur={() => commit(rows)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') e.currentTarget.blur()
              }}
            />
          </span>
          <SegmentedControl
            className={styles.accountKind}
            options={ACCOUNT_KINDS.map((k) => ({ value: k, label: k }))}
            value={account.kind}
            onChange={(v) => update(i, { kind: v as ProviderAccount['kind'] })}
          />
          <span className={styles.accountRef}>
            {account.kind === 'oauth' ? (
              // OAuth is not something you type. The credential is obtained by
              // signing in, so the field is a button; `ref` still holds where
              // the resulting credential lives, shown once it does.
              <span className={styles.oauthCell}>
                <Button variant="secondary" size="xs" onClick={() => onSignIn(i)}>
                  {account.ref ? 'Re-authenticate' : 'Sign in…'}
                </Button>
                <span className={cx('mono', styles.oauthRef)}>
                  {account.ref || 'not signed in'}
                </span>
              </span>
            ) : (
              <Input
                value={account.ref}
                mono
                placeholder={REF_PLACEHOLDER[account.kind]}
                onChange={(v) => setDraft(rows.map((a, n) => (n === i ? { ...a, ref: v } : a)))}
                onBlur={() => commit(rows)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') e.currentTarget.blur()
                }}
              />
            )}
          </span>
          <button
            type="button"
            className={cx('ib', styles.accountDelete)}
            title={`Remove ${account.name}`}
            onClick={() => commit(rows.filter((_, n) => n !== i))}
          >
            <svg
              width="12"
              height="12"
              viewBox="0 0 14 14"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.25"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d="M2.4 4h9.2M5.6 4V2.7h2.8V4M3.7 4l.45 7.3h5.7L10.3 4M5.9 6.2v3.2M8.1 6.2v3.2" />
            </svg>
          </button>
        </div>
      ))}

      <div className={styles.accountsFoot}>
        Accounts record where a credential lives — an environment variable, file path or keychain
        service — never the secret itself.
      </div>
    </section>
  )
}

export default ProvidersPage
