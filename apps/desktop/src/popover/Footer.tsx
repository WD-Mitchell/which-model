import { Button, SplitButton } from '@which-model/ui'
import type { HarnessInfo } from '@which-model/core'
import './Footer.css'

export interface PopoverFooterProps {
  // The footer no longer varies by tab: Settings + "Launch in <harness>" are
  // present on both, and the weights actions moved into the editor's own row.
  harnesses?: HarnessInfo[]
  harnessSlug?: string
  harnessMenuOpen?: boolean
  onToggleHarnessMenu?(): void
  onPickHarness?(slug: string): void
  onManage?(): void
  onLaunch?(): void
  /** Copies the current pick's model id. Present on both tabs — it acts on the
   *  pick shown in the results band, which both tabs share. */
  onCopy?(): void
}

export function PopoverFooter({
  harnesses = [],
  harnessSlug,
  harnessMenuOpen = false,
  onToggleHarnessMenu,
  onPickHarness,
  onManage,
  onLaunch,
  onCopy,
}: PopoverFooterProps) {
  const harness =
    harnesses.find((h) => h.slug === harnessSlug) ?? harnesses[0]
  const harnessName = harness?.name ?? ''
  const menuItems = harnesses.map((h) => ({
    key: h.slug,
    label: h.name,
    selected: h.slug === (harnessSlug ?? harnesses[0]?.slug),
  }))
  return (
    <div className="pf-footer">
      <Button variant="ghost" className="pf-manage" onClick={onManage}>
        Settings
      </Button>
      <Button variant="ghost" className="pf-copy" onClick={onCopy}>
        Copy model id
      </Button>
      <span className="pf-launchWrap" data-launch-pill>
        <SplitButton
          label={`Launch in ${harnessName}`}
          onMain={onLaunch ?? (() => {})}
          menuItems={menuItems}
          onPick={(key) => onPickHarness?.(key)}
          open={harnessMenuOpen}
          onOpenChange={onToggleHarnessMenu ?? (() => {})}
        />
      </span>
    </div>
  )
}