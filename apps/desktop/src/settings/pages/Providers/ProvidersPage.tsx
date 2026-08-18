// U10 — Providers settings page: priority-ordered provider list with
// enable toggles and usage meters, plus a provider detail view with per-route
// enablement toggles across models × reasoning levels.
import { useCallback, useState } from 'react'
import { DragList, Tag, Toggle, UsageMeter, useToast } from '@which-model/ui'
import type { ProviderInfo } from '@which-model/core'
import { useProviderDetail, useProviders } from '../../../lib/queries'
import { getHost } from '../../../lib/host'
import { DetailHeader } from '../../DetailHeader'
import { PAGE_META } from '../../pages'
import type { Detail, PageComponentProps } from '../../pages'
import styles from './ProvidersPage.module.css'

export function ProvidersPage({ detail, openDetail, closeDetail }: PageComponentProps) {
  return detail?.kind === 'provider' ? (
    <ProviderDetailView id={detail.id} onBack={closeDetail} openDetail={openDetail} />
  ) : (
    <ProvidersListView openDetail={openDetail} />
  )
}

function ProvidersListView({ openDetail }: { openDetail(d: Detail): void }) {
  const toast = useToast()
  const { data: providers } = useProviders()
  const list = providers ?? []

  const handleReorder = useCallback(
    async (ids: string[]) => {
      try {
        await getHost().providers.reorder(ids)
      } catch (e) {
        toast.show((e as { message?: string }).message ?? 'reorder failed')
      }
    },
    [toast],
  )

  const handleToggle = useCallback(
    async (id: string, on: boolean) => {
      try {
        await getHost().providers.setEnabled(id, on)
      } catch (e) {
        toast.show((e as { message?: string }).message ?? 'update failed')
      }
    },
    [toast],
  )

  return (
    <div className={styles.page}>
      <DetailHeader title={PAGE_META.Providers[0]} blurb={PAGE_META.Providers[1]} />
      <div className={styles.kicker}>providers</div>
      <DragList
        items={list.map((p) => ({
          id: p.id,
          node: (
            <div className={styles.row} onClick={() => openDetail({ kind: 'provider', id: p.id })}>
              <span className={styles.name}>
                <span className="mono">{p.id}</span>
              </span>
              <span className={styles.toggle}>
                <Toggle on={p.enabled} onToggle={(on) => void handleToggle(p.id, on)} />
              </span>
              <span className={styles.usage}>
                <UsageMeter label="session" percent={p.session} />
                <UsageMeter label="weekly" percent={p.weekly} />
                <UsageMeter label="monthly" percent={p.monthly} />
              </span>
              <span className={styles.limits} title={p.limits_line}>{p.limits_line}</span>
              <span className={styles.routes}>
                <Tag variant={p.routes_on > 0 ? 'accent' : 'outline'}>
                  {`${p.routes_on} / ${p.routes_total} routes`}
                </Tag>
              </span>
              <span className="ib">›</span>
            </div>
          ),
        }))}
        onReorder={handleReorder}
      />
    </div>
  )
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
  const { data: providers } = useProviders()
  void openDetail

  const info: ProviderInfo | undefined = providers?.find((p) => p.id === id)

  if (!detail) return <div className={styles.page}>Loading…</div>

  const handleRoute = async (modelId: string, reasoning: string, on: boolean) => {
    try {
      await getHost().providers.setRouteEnabled(id, modelId, reasoning, on)
    } catch (e) {
      toast.show((e as { message?: string }).message ?? 'update failed')
    }
  }

  const handleAll = async (on: boolean) => {
    try {
      await getHost().providers.setAllRoutes(id, on)
    } catch (e) {
      toast.show((e as { message?: string }).message ?? 'update failed')
    }
  }

  const allOn = detail.models.length > 0 && detail.models.every((m) => m.levels.every((l) => l.enabled))

  return (
    <div className={styles.page}>
      <DetailHeader
        title={id}
        blurb={info?.limits_line ?? ''}
        backLabel="Providers"
        onBack={onBack}
        action={{ label: allOn ? 'Disable all' : 'Enable all', onAction: () => void handleAll(!allOn) }}
      />
      {detail.models.map((m) => (
        <div key={m.model_id} className={styles.model}>
          <div className={styles.modelHead}>
            <span className="mono">{m.model_name}</span>
            <Tag variant="outline">{m.model_id}</Tag>
          </div>
          {m.levels.map((l) => (
            <div key={l.reasoning} className={styles.level}>
              <span className="mono levelName">{l.reasoning}</span>
              {l.default ? <Tag variant="neutral">default</Tag> : null}
              <span className={styles.toggle}>
                <Toggle on={l.enabled} onToggle={(on) => void handleRoute(m.model_id, l.reasoning, on)} />
              </span>
            </div>
          ))}
        </div>
      ))}
    </div>
  )
}
export default ProvidersPage
