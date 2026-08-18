// U14 — Agent integration settings page: shell snippets preview + MCP/CLI
// toggles.
import { useCallback } from 'react'
import { SnippetPreview, Toggle, useToast } from '@which-model/ui'
import { useSettings, useSnippets } from '../../../lib/queries'
import { getHost } from '../../../lib/host'
import { DetailHeader } from '../../DetailHeader'
import { PAGE_META } from '../../pages'
import type { PageComponentProps } from '../../pages'
import styles from './AgentPage.module.css'

export function AgentPage(_props: PageComponentProps) {
  const toast = useToast()
  const { data: snippets } = useSnippets()
  const { data: settings } = useSettings()

  const copy = useCallback(
    async (text: string) => {
      await getHost().window.copyToClipboard(text)
      toast.show('copied')
    },
    [toast],
  )

  const setSetting = useCallback(
    async (patch: Partial<NonNullable<typeof settings>>) => {
      if (!settings) return
      try {
        await getHost().settings.set({ ...settings, ...patch })
      } catch (e) {
        toast.show((e as { message?: string }).message ?? 'save failed')
      }
    },
    [settings, toast],
  )

  return (
    <div className={styles.page}>
      <DetailHeader title={PAGE_META['Agent integration'][0]} blurb={PAGE_META['Agent integration'][1]} />

      <div className={styles.section}>
        <div className={styles.sectionHeader}>shell alias</div>
        {snippets ? (
          <div className={styles.snippet}>
            <SnippetPreview text={snippets.alias} copyable onCopy={(t) => void copy(t)} />
          </div>
        ) : null}
        <ToggleRow label="Install shell alias" on={settings?.shell_alias ?? false}
          onToggle={(on) => void setSetting({ shell_alias: on })} />
      </div>

      <div className={styles.section}>
        <div className={styles.sectionHeader}>claude.md hint</div>
        {snippets ? (
          <div className={styles.snippet}>
            <SnippetPreview text={snippets.claude_md} copyable onCopy={(t) => void copy(t)} />
          </div>
        ) : null}
        <ToggleRow label="Append claude.md hint" on={settings?.claude_md_hint ?? false}
          onToggle={(on) => void setSetting({ claude_md_hint: on })} />
      </div>

      <div className={styles.section}>
        <div className={styles.sectionHeader}>mcp server</div>
        <ToggleRow label="Run the MCP server in the background" on={settings?.mcp_server ?? false}
          onToggle={(on) => void setSetting({ mcp_server: on })} />
      </div>
    </div>
  )
}

function ToggleRow({ label, on, onToggle }: { label: string; on: boolean; onToggle(on: boolean): void }) {
  return (
    <div className={styles.row}>
      <span className={styles.label}>{label}</span>
      <span className={styles.toggle}><Toggle on={on} onToggle={onToggle} /></span>
    </div>
  )
}
export default AgentPage
