import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import {
  SETTINGS_NAV_ITEMS,
  SettingsDetailShell,
  SettingsHeader,
  SettingsModal,
  SettingsNav,
  SettingsRow,
  SettingsSection,
  SettingsShell,
} from '../index'

describe('SettingsNav', () => {
  it('renders the eight settings pages and marks only the active page', () => {
    render(<SettingsNav activeItem="Providers" />)

    expect(screen.getAllByRole('button')).toHaveLength(SETTINGS_NAV_ITEMS.length)
    expect(screen.getByRole('button', { name: 'Providers' })).toHaveAttribute('aria-current', 'page')
    expect(screen.getByRole('button', { name: 'Profiles' })).not.toHaveAttribute('aria-current')
  })

  it('reports the selected item', () => {
    const onSelect = vi.fn()
    render(<SettingsNav onSelect={onSelect} />)
    fireEvent.click(screen.getByRole('button', { name: 'Harnesses' }))
    expect(onSelect).toHaveBeenCalledWith('Harnesses')
  })
})

describe('SettingsShell', () => {
  it('places navigation beside content and forwards selection', () => {
    const onSectionChange = vi.fn()
    render(
      <SettingsShell activeSection="Profiles" onSectionChange={onSectionChange}>
        <p>Profile settings</p>
      </SettingsShell>,
    )

    expect(screen.getByText('Profile settings')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'General' }))
    expect(onSectionChange).toHaveBeenCalledWith('General')
  })
})

describe('SettingsHeader', () => {
  it('renders supporting copy and action buttons', () => {
    const onAction = vi.fn()
    render(
      <SettingsHeader
        title="Providers"
        description="Choose which providers may be used."
        action={{ label: 'Add provider', onAction }}
      />,
    )

    expect(screen.getByRole('heading', { name: 'Providers' })).toBeInTheDocument()
    expect(screen.getByText('Choose which providers may be used.')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Add provider' }))
    expect(onAction).toHaveBeenCalledTimes(1)
  })
})

describe('SettingsSection and SettingsRow', () => {
  it('renders a labelled section and trailing row control', () => {
    render(
      <SettingsSection label="Defaults" description="Configure defaults.">
        <SettingsRow label="Profile" description="Used for new picks." control={<button type="button">Change</button>} />
      </SettingsSection>,
    )

    expect(screen.getByRole('heading', { name: 'Defaults' })).toBeInTheDocument()
    expect(screen.getByText('Configure defaults.')).toBeInTheDocument()
    expect(screen.getByText('Used for new picks.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Change' })).toBeInTheDocument()
  })
})

describe('SettingsModal', () => {
  it('supports confirmation and escape dismissal', () => {
    const onConfirm = vi.fn()
    const onClose = vi.fn()
    render(
      <SettingsModal title="Delete profile" onConfirm={onConfirm} onClose={onClose}>
        Are you sure?
      </SettingsModal>,
    )

    expect(screen.getByRole('dialog', { name: 'Delete profile' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Save' }))
    expect(onConfirm).toHaveBeenCalledTimes(1)
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('does not render when closed', () => {
    render(<SettingsModal open={false} title="Hidden" />)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
})

describe('SettingsDetailShell', () => {
  it('renders master and detail panes and handles back', () => {
    const onBack = vi.fn()
    render(
      <SettingsDetailShell title="Profile" onBack={onBack} backLabel="Profiles" master={<p>Profiles list</p>}>
        <p>Profile detail</p>
      </SettingsDetailShell>,
    )

    expect(screen.getByText('Profiles list')).toBeInTheDocument()
    expect(screen.getByText('Profile detail')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Profiles' }))
    expect(onBack).toHaveBeenCalledTimes(1)
  })
})
