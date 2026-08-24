import type React from 'react'
import './PopoverShell.css'

export interface PopoverShellProps {
  header: React.ReactNode
  children: React.ReactNode
}

export function PopoverShell({ header, children }: PopoverShellProps) {
  return (
    <div className="ps-outer">
      <div className="ps-arrow" />
      <div className="ps-panel">
        <div className="ps-glow" />
        {header}
        {children}
      </div>
    </div>
  )
}