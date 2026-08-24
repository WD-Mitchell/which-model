// U07 — SettingsApp: settings window root. Owns page/detail nav state,
// provides the EngineHost via React context (useHost), and mounts the active
// page from PAGE_REGISTRY in a Suspense boundary.
import { createContext, Suspense, useCallback, useContext, useMemo, useState } from 'react'
import type { EngineHost } from '@which-model/core'
import { PAGE_REGISTRY, type Detail, type PageComponentProps, type SettingsPageName } from './pages'
import { useSettings } from '../lib/queries'
import { SettingsShell } from './SettingsShell'

// Host context: SettingsApp provides the host so pages can reach the engine
// without importing getHost() (kept testable with a stub host).
const HostContext = createContext<EngineHost | null>(null)

export function useHost(): EngineHost {
  const host = useContext(HostContext)
  if (!host) throw new Error('useHost requires SettingsApp provider')
  return host
}

export interface SettingsAppProps {
  host: EngineHost
}

type DetailStack = Detail[] | null

function push(stack: DetailStack, d: Detail): DetailStack {
  return stack === null ? [d] : [...stack, d]
}

function top(stack: DetailStack): Detail | null {
  return stack === null || stack.length === 0 ? null : stack[stack.length - 1]
}

function back(stack: DetailStack): DetailStack {
  if (stack === null || stack.length === 0) return null
  return stack.slice(0, -1)
}

export function SettingsApp({ host }: SettingsAppProps) {
  const [page, setPage] = useState<SettingsPageName>('General')
  const [stack, setStack] = useState<DetailStack>(null)

  const detail = top(stack)
  const settingsQuery = useSettings()
  const configPath = settingsQuery.data?.config_path ?? ''

  const openDetail = useCallback((d: Detail) => {
    setStack((s) => push(s, d))
  }, [])

  const closeDetail = useCallback(() => {
    setStack((s) => back(s))
  }, [])

  const onPage = useCallback((p: SettingsPageName) => {
    setStack(null) // switching page clears detail (U07 CONTRACTS §6)
    setPage(p)
  }, [])

  const onClose = useCallback(() => {
    void host.window.closeSettings().catch(() => {})
  }, [host])

  const pageProps = useMemo<PageComponentProps>(
    () => ({ detail, openDetail, closeDetail }),
    [detail, openDetail, closeDetail],
  )

  const PageComponent = PAGE_REGISTRY[page]

  return (
    <HostContext.Provider value={host}>
      <SettingsShell page={page} onPage={onPage} configPath={configPath} onClose={onClose}>
        <Suspense fallback={<div className="settings-suspend">loading…</div>}>
          <PageComponent {...pageProps} />
        </Suspense>
      </SettingsShell>
    </HostContext.Provider>
  )
}