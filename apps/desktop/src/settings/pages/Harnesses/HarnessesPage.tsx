// U11 — Harnesses settings page: the harness list (provider pips + count,
// launch command, remove) and a harness detail view (launch-command preview,
// per-provider switches), plus the add-custom form the page action opens.
// Markup and metrics are a port of the mockup, demo.dc.html L492-523 (list)
// and L524-573 (detail); the layout contract is U07's — <main> has no
// horizontal padding, so every block here carries its own 22px gutter.
import { useCallback, useState } from 'react'
import {
  Button,
  cx,
  EmptyState,
  Input,
  ProviderPips,
  ProviderUsageRow,
  SnippetPreview,
  Tag,
  useToast,
} from '@which-model/ui'
import type { HarnessInfo } from '@which-model/core'
import { useHarnesses, useProviders } from '../../../lib/queries'
import { getHost } from '../../../lib/host'
import { DetailHeader } from '../../DetailHeader'
import { PAGE_META } from '../../pages'
import type { Detail, PageComponentProps } from '../../pages'
import styles from './HarnessesPage.module.css'

// demo.dc.html 1386 (`detectLine`) — verbatim, typographic apostrophe included.
const DETECT_LINE =
  'harnesses and their providers are read from each harness’ own config on launch'
// demo.dc.html 1382 (`harnessTokenNote`) and 1192 (the harness detail blurb).
const TOKEN_NOTE = 'substituted at launch from the pick'
const DETAIL_BLURB =
  'Providers this harness may use, detected from its own configuration. Switch one off to keep it out of every launch here.'
// demo.dc.html 570.
const CLOSING_NOTE =
  'A provider switched off here is never used when launching in this harness.'

/** demo.dc.html 516 — the row's remove control (an `.ib` box, 22×22). */
function TrashIcon() {
  return (
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
  )
}

/** demo.dc.html 517 — the row-trailing chevron: NOT an `.ib`, just 38% ink. */
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

export function HarnessesPage({ detail, openDetail, closeDetail }: PageComponentProps) {
  return detail?.kind === 'harness' ? (
    <HarnessDetailView slug={detail.id} onBack={closeDetail} openDetail={openDetail} />
  ) : (
    <HarnessesListView openDetail={openDetail} />
  )
}

function HarnessesListView({ openDetail }: { openDetail(d: Detail): void }) {
  const toast = useToast()
  const { data: harnesses } = useHarnesses()
  // One providers read for the whole list: the pips and the count in every row
  // are that list projected through `h.providers`.
  const { data: providers } = useProviders()
  const list = harnesses ?? []
  const providerIds = (providers ?? []).map((p) => p.id)
  const [adding, setAdding] = useState(false)
  const [slug, setSlug] = useState('')
  const [name, setName] = useState('')
  const [command, setCommand] = useState('')

  const handleAdd = useCallback(async () => {
    if (!slug.trim() || !name.trim() || !command.includes('{model_id}')) {
      toast.show('require slug, name and a command containing {model_id}')
      return
    }
    try {
      await getHost().harnesses.save({
        slug: slug.trim(),
        name: name.trim(),
        command: command.trim(),
        builtin: false,
        installed: true,
        providers: {},
      } as HarnessInfo)
      toast.show(`added ${slug.trim()}`)
      setAdding(false)
      setSlug('')
      setName('')
      setCommand('')
    } catch (e) {
      toast.show((e as { message?: string }).message ?? 'add failed')
    }
  }, [slug, name, command, toast])

  const handleDelete = useCallback(
    async (h: HarnessInfo) => {
      try {
        await getHost().harnesses.delete(h.slug)
        toast.show(`deleted ${h.slug}`)
      } catch (e) {
        toast.show((e as { message?: string }).message ?? 'delete failed')
      }
    },
    [toast],
  )

  return (
    <div className={styles.page}>
      <DetailHeader
        title={PAGE_META.Harnesses[0]}
        blurb={PAGE_META.Harnesses[1]}
        action={{ label: PAGE_META.Harnesses[2] as string, onAction: () => setAdding((v) => !v) }}
      />
      {adding ? (
        <div className={styles.addRow}>
          <span className={styles.addSlug}>
            <Input value={slug} onChange={setSlug} placeholder="slug" mono />
          </span>
          <span className={styles.addName}>
            <Input value={name} onChange={setName} placeholder="name" mono={false} />
          </span>
          <span className={styles.addCmd}>
            {/* demo.dc.html 1207 seeds a custom harness with exactly this command. */}
            <Input
              value={command}
              onChange={setCommand}
              placeholder="my-agent --model {model_id}"
              mono
            />
          </span>
          <Button variant="primary" size="sm" onClick={() => void handleAdd()}>
            Create
          </Button>
          <Button variant="ghost" size="sm" onClick={() => setAdding(false)}>
            Cancel
          </Button>
        </div>
      ) : null}

      {/* demo.dc.html 493 */}
      <span className={cx('mono', styles.kicker)}>harnesses</span>

      {/* demo.dc.html 494-499 */}
      <div className={styles.head}>
        <span className={cx('mono', styles.headCell, styles.colName)}>harness</span>
        <span className={cx('mono', styles.headCell, styles.colProv)}>providers</span>
        {/* No "launch command" column: long, always truncated at this width,
            and shown in full on the harness detail view. The name column takes
            the freed space rather than leaving a dead gutter. */}
        <span className={styles.colEnd} />
      </div>

      {list.map((h) => {
        const on = providerIds.filter((id) => h.providers[id]).length
        return (
          <div
            key={h.slug}
            className={cx('row', styles.row)}
            onClick={() => openDetail({ kind: 'harness', id: h.slug })}
          >
            <span className={styles.nameCell}>
              <span
                className={cx('mono', styles.name, !h.installed && styles.nameOff)}
                title={h.installed ? h.slug : `${h.slug} — not detected on this Mac`}
              >
                {h.name}
              </span>
              {/* demo.dc.html 504 — only user-added harnesses are badged. */}
              {h.builtin ? null : (
                <Tag variant="neutral" size="badge" className={styles.badge}>
                  custom
                </Tag>
              )}
            </span>
            <span className={styles.provCell}>
              <ProviderPips states={providerIds.map((id) => h.providers[id] ?? false)} />
              {/* demo.dc.html 1510-1511 — "n of m", or a DIM "none". */}
              <span className={cx('mono', styles.provCount, on === 0 && styles.provNone)}>
                {on === 0 ? 'none' : `${on} of ${providerIds.length}`}
              </span>
            </span>
            <span className={styles.actions} onClick={(e) => e.stopPropagation()}>
              <button
                type="button"
                className={cx('ib', h.builtin && 'off', styles.del)}
                title={h.builtin ? 'Built-in harness — cannot be removed' : `Remove ${h.slug}`}
                disabled={h.builtin}
                onClick={() => void handleDelete(h)}
              >
                <TrashIcon />
              </button>
              <span className={styles.chev}>
                <ChevronIcon />
              </span>
            </span>
          </div>
        )
      })}

      {list.length === 0 ? (
        <div className={styles.empty}>
          <EmptyState text="no harnesses yet — add a custom one to launch a pick into your own agent." />
        </div>
      ) : null}

      {/* demo.dc.html 521 */}
      <div className={styles.note}>{DETECT_LINE}</div>
    </div>
  )
}

function HarnessDetailView({
  slug,
  onBack,
  openDetail,
}: {
  slug: string
  onBack(): void
  openDetail(d: Detail): void
}) {
  const toast = useToast()
  const { data: harnesses } = useHarnesses()
  const { data: providers } = useProviders()
  void openDetail

  const handleProvider = useCallback(
    async (p: string, on: boolean) => {
      try {
        await getHost().harnesses.setProvider(slug, p, on)
      } catch (e) {
        toast.show((e as { message?: string }).message ?? 'update failed')
      }
    },
    [slug, toast],
  )

  // demo.dc.html 1384-1385 — the header's bulk switches.
  const handleAll = useCallback(
    async (on: boolean) => {
      try {
        await getHost().harnesses.setAllProviders(slug, on)
      } catch (e) {
        toast.show((e as { message?: string }).message ?? 'update failed')
      }
    },
    [slug, toast],
  )

  const h = harnesses?.find((x) => x.slug === slug)
  if (!h) return <div className={styles.note}>loading…</div>

  const list = providers ?? []
  const onCount = list.filter((p) => h.providers[p.id]).length

  return (
    <div className={styles.page}>
      <DetailHeader title={h.name} blurb={DETAIL_BLURB} backLabel="Harnesses" onBack={onBack} />

      {/* demo.dc.html 525-531 */}
      <section className={styles.cmdBlock}>
        <div className={styles.blockHead}>
          <span className={styles.label}>launch command</span>
          <span className={cx('mono', styles.headNote)}>{TOKEN_NOTE}</span>
          <span className={cx('mono', styles.slugNote)}>
            {h.slug}
            {h.installed ? null : (
              <Tag variant="neutral" size="badge">
                not detected
              </Tag>
            )}
          </span>
        </div>
        {/* variant="command" is `class="mono input"` at the mockup's metrics. */}
        <SnippetPreview text={h.command} variant="command" />
      </section>

      {/* demo.dc.html 533-571 */}
      <section className={styles.provBlock}>
        <div className={styles.provHead}>
          <span className={styles.label}>providers</span>
          <span className={cx('mono', styles.headNote)}>
            {`${onCount} of ${list.length} enabled`}
          </span>
          <span className={styles.headActions}>
            <Button variant="ghost" size="xs" onClick={() => void handleAll(true)}>
              Enable all
            </Button>
            <Button
              variant="ghost"
              size="xs"
              className={styles.ghostOff}
              onClick={() => void handleAll(false)}
            >
              Disable all
            </Button>
          </span>
        </div>
        <div className={styles.provList}>
          {list.map((p) => (
            <ProviderUsageRow
              key={p.id}
              provider={p}
              on={h.providers[p.id] ?? false}
              // A provider switched on for this harness still reports nothing
              // when it is off app-wide, so "live" needs both.
              live={p.enabled && (h.providers[p.id] ?? false)}
              offLabel={p.enabled ? 'not used here' : 'off globally'}
              onToggle={(next: boolean) => void handleProvider(p.id, next)}
            />
          ))}
        </div>
        {list.length === 0 ? (
          <EmptyState text="no providers configured yet — add one on the Providers page." />
        ) : null}
        <div className={styles.closing}>{CLOSING_NOTE}</div>
      </section>
    </div>
  )
}

export default HarnessesPage
