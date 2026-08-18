import type React from 'react'
import { Button, SplitButton } from '@which-model/ui'
import type { HarnessInfo } from '@which-model/core'
import type { PopoverView } from './PopoverApp'
import './Footer.css'

export interface PopoverFooterProps {
  variant: PopoverView
  // landing variant
  harnesses?: HarnessInfo[]
  harnessSlug?: string
  harnessMenuOpen?: boolean
  onToggleHarnessMenu?(): void
  onPickHarness?(slug: string): void
  onManage?(): void
  onLaunch?(): void
  // weights variant (U06 supplies the buttons)
  children?: React.ReactNode
}

export function PopoverFooter({
  variant,
  harnesses = [],
  harnessSlug,
  harnessMenuOpen = false,
  onToggleHarnessMenu,
  onPickHarness,
  onManage,
  onLaunch,
  children,
}: PopoverFooterProps) {
  if (variant === 'landing') {
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
          Manage profiles
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
  return <div className="pf-footer">{children}</div>
}