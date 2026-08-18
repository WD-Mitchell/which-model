import type React from 'react'
import { Menu, type MenuItem } from '@which-model/ui'
import './PopoverShell.css'

export interface PopoverShellProps {
  header: React.ReactNode
  menuOpen: boolean
  onToggleMenu(): void
  onCustomWeights(): void
  onOpenSettings(): void
  onQuit(): void
  children: React.ReactNode
}

const APP_MENU_ITEMS: MenuItem[] = [
  { key: 'weights', label: 'Custom weights…' },
  { key: 'settings', label: 'Settings…' },
  { key: 'divider', separator: true },
  { key: 'quit', label: 'Quit which-model', dim: true },
]

export function PopoverShell({
  header,
  menuOpen,
  onToggleMenu,
  onCustomWeights,
  onOpenSettings,
  onQuit,
  children,
}: PopoverShellProps) {
  function handlePick(key: string): void {
    if (key === 'weights') onCustomWeights()
    else if (key === 'settings') onOpenSettings()
    else if (key === 'quit') onQuit()
    onToggleMenu() // picking an item always closes the menu
  }

  return (
    <div className="ps-outer">
      <div className="ps-arrow" />
      <div className="ps-panel">
        <div className="ps-glow" />
        {header}
        {children}
        {menuOpen && (
          <Menu
            className="ps-appmenu"
            items={APP_MENU_ITEMS}
            onPick={handlePick}
            onClose={() => menuOpen && onToggleMenu()}
          />
        )}
      </div>
    </div>
  )
}