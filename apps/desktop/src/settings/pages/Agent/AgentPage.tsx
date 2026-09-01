// U14 — Agent integration settings page (U14 SPEC §3): the three integration
// toggles and the shell-hook snippet box. Markup ported from the mockup's
// `onAgent` block (demo.dc.html L784-799). Unlike the list pages this block is
// *inset*: the wrapper pays the 22px gutter once and the row rules stop at it,
// rather than each row bleeding edge-to-edge.
import { useCallback } from 'react'
import { SnippetPreview, Toggle, useToast } from '@which-model/ui'
import type { GUISettings } from '@which-model/core'
import { useSettings, useSnippets } from '../../../lib/queries'
import { getHost } from '../../../lib/host'
import { DetailHeader } from '../../DetailHeader'
import { PAGE_META } from '../../pages'
import type { PageComponentProps } from '../../pages'
import styles from './AgentPage.module.css'

/** The three GUISettings booleans this page owns. */
type IntegrationKey = 'mcp_server' | 'claude_md_hint' | 'shell_alias'

/** SPEC §3.1 — names and notes verbatim, in the mockup's `agentRows` order
 *  (demo.dc.html L1556-1560): MCP server, CLAUDE.md hint, shell alias. */
const INTEGRATIONS: readonly { key: IntegrationKey; name: string; note: string }[] = [
  {
    key: 'mcp_server',
    name: 'Expose as an MCP server',
    note: 'Agents can ask which-model for a pick mid-session instead of being told one up front.',
  },
  {
    key: 'claude_md_hint',
    name: 'Write a CLAUDE.md hint',
    note: 'Adds a short note to new repositories describing the available profiles.',
  },
  {
    key: 'shell_alias',
    name: 'Shell alias wm',
    note: 'wm research launches the top pick for a profile without the popover.',
  },
]

export function AgentPage(_props: PageComponentProps) {
  const toast = useToast()
  const { data: snippets } = useSnippets()
  const { data: settings } = useSettings()

  const copy = useCallback(
    async (text: string) => {
      try {
        await getHost().window.copyToClipboard(text)
        toast.show('copied')
      } catch (e) {
        // Silent failure reads as success; report it like the popover's
        // clipboard path does (issue #42).
        toast.show((e as { message?: string }).message ?? 'copy failed')
      }
    },
    [toast],
  )

  // Same one-delta whole-struct write as U12 (SPEC §3.1 / §5): read the cached
  // settings, change exactly one field, hand the whole struct back. No
  // debounce — `config:changed` invalidates ['settings'] and re-renders.
  const setSetting = useCallback(
    async (patch: Partial<GUISettings>) => {
      if (!settings) return
      try {
        await getHost().settings.set({ ...settings, ...patch })
      } catch (e) {
        toast.show((e as { message?: string }).message ?? 'save failed')
      }
    },
    [settings, toast],
  )

  // SPEC §3.2 / §5 — the box shows the alias to install and then the preview
  // line the mockup renders as `agentSnippet`
  // ("$ wm {slug}  →  {model_id}  ({route})").
  const hook = snippets ? [snippets.alias, snippets.preview].filter(Boolean).join('\n') : ''

  return (
    <div className={styles.page}>
      <DetailHeader title={PAGE_META['Agent integration'][0]} blurb={PAGE_META['Agent integration'][1]} />

      <div className={styles.body}>
        <span className={`mono ${styles.kicker}`}>integrations</span>
        {INTEGRATIONS.map((row) => {
          const on = settings?.[row.key] ?? false
          return (
            <div key={row.key} className={styles.row}>
              <span className={styles.rowText}>
                {/* 12.5px, BRIGHT (88%) when on and MUTED (72%) when off —
                    the mockup's `a.fg` binding. */}
                <span className={on ? `${styles.name} ${styles.nameOn}` : styles.name}>{row.name}</span>
                <span className={styles.note}>{row.note}</span>
              </span>
              <Toggle
                on={on}
                onToggle={(next) =>
                  // Computed key over a union of GUISettings booleans: TS widens
                  // it to an index signature, so the shape is asserted back.
                  void setSetting({ [row.key]: next } as Partial<GUISettings>)
                }
              />
            </div>
          )
        })}

        <span className={`mono ${styles.kicker} ${styles.kickerGap}`}>shell hook</span>
        {snippets ? (
          // SnippetPreview's `block` variant is the mockup's snippet box
          // verbatim (11px/12px padding, radius 8, 6% ground, 11px, lh 1.7,
          // 62% text). Display-only per SPEC §3.2, but click-to-copy is kept.
          <SnippetPreview text={hook} copyable onCopy={(t) => void copy(t)} />
        ) : null}
      </div>
    </div>
  )
}
export default AgentPage
