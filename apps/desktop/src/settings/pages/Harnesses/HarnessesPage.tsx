// U11 — Harnesses settings page: harness list (installed detection, provider
// pips) plus detail (provider-on toggles, command preview), and add-custom.
import { useCallback, useState } from 'react'
import { Button, Input, ProviderPips, SnippetPreview, Tag, Toggle, useToast } from '@which-model/ui'
import type { HarnessInfo } from '@which-model/core'
import { useHarnesses, useProviders } from '../../../lib/queries'
import { getHost } from '../../../lib/host'
import { DetailHeader } from '../../DetailHeader'
import { PAGE_META } from '../../pages'
import type { Detail, PageComponentProps } from '../../pages'
import styles from './HarnessesPage.module.css'

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
  const list = harnesses ?? []
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

  return (
    <div className={styles.page}>
      <DetailHeader
        title={PAGE_META.Harnesses[0]}
        blurb={PAGE_META.Harnesses[1]}
        action={{ label: PAGE_META.Harnesses[2] as string, onAction: () => setAdding((v) => !v) }}
      />
      {adding ? (
        <div className={styles.addRow}>
          <div style={{ width: '120px' }}><Input value={slug} onChange={setSlug} placeholder="slug" mono /></div>
          <div style={{ width: '140px' }}><Input value={name} onChange={setName} placeholder="name" /></div>
          <div style={{ flex: 1 }}><Input value={command} onChange={setCommand} placeholder="command ({model_id})" mono /></div>
          <Button variant="secondary" size="sm" onClick={() => void handleAdd()}>Create</Button>
        </div>
      ) : null}
      <div className={styles.kicker}>harnesses</div>
      {list.map((h) => (
        <div key={h.slug} className={styles.row} onClick={() => openDetail({ kind: 'harness', id: h.slug })}>
          <span className={styles.name}><span className="mono">{h.name}</span></span>
          <span className={styles.installed}>
            <Tag variant={h.installed ? 'accent' : 'outline'}>{h.installed ? 'installed' : 'not detected'}</Tag>
          </span>
          <span className={styles.pips}>
            <ProvidedPipsInfo h={h} />
          </span>
          <span className={styles.cmd}><code className="mono">{h.command}</code></span>
          <span className={styles.actions} onClick={(e) => e.stopPropagation()}>
            <button type="button" className={'ib ' + (h.builtin ? ' off' : '')}
              title={h.builtin ? 'Built-in harness' : `Delete ${h.slug}`}
              disabled={h.builtin}
              onClick={() =>
                void getHost().harnesses.delete(h.slug)
                  .then(() => toast.show(`deleted ${h.slug}`))
                  .catch((e) => toast.show((e as { message?: string }).message ?? 'delete failed'))
              }>×</button>
            <span className="ib">›</span>
          </span>
        </div>
      ))}
    </div>
  )
}

function ProvidedPipsInfo({ h }: { h: HarnessInfo }) {
  const { data: providers } = useProviders()
  const ids = (providers ?? []).map((p) => p.id)
  return <ProviderPips states={ids.map((id) => h.providers[id] ?? false)} />
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

  const h = harnesses?.find((x) => x.slug === slug)
  if (!h) return <div className={styles.page}>Loading…</div>

  const providerIds = (providers ?? []).map((p) => p.id)

  const handleProvider = async (p: string, on: boolean) => {
    try {
      await getHost().harnesses.setProvider(slug, p, on)
    } catch (e) {
      toast.show((e as { message?: string }).message ?? 'update failed')
    }
  }

  return (
    <div className={styles.page}>
      <DetailHeader title={h.name} blurb={`${h.slug} — ${h.installed ? 'installed' : 'not detected'}`} backLabel="Harnesses" onBack={onBack} />
      <div className={styles.preview}>
        <SnippetPreview text={h.command} />
      </div>
      <div className={styles.provLabel}>usable with</div>
      {providerIds.map((p) => (
        <div key={p} className={styles.provRow}>
          <span className="mono">{p}</span>
          <span className={styles.toggle}>
            <Toggle on={h.providers[p] ?? false} onToggle={(on) => void handleProvider(p, on)} />
          </span>
        </div>
      ))}
    </div>
  )
}
export default HarnessesPage
