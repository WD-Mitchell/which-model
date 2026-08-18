import type { CSSProperties, KeyboardEvent } from 'react'
import { usePointerFraction } from '../../hooks/usePointerFraction'

export interface ComplexityScaleProps {
  stop: number
  labels?: [string, string]
  profileName?: string
  readOnly?: boolean
  onStop?: (i: number) => void
}

const KNOB_SHADOW = '0 0 0 1.5px var(--color-accent)'

function clampStop(value: number): number {
  if (!Number.isFinite(value)) return 0
  return Math.max(0, Math.min(4, Math.round(value)))
}

export function ComplexityScale({
  stop,
  labels = ['simple action', 'planning'],
  profileName,
  readOnly = false,
  onStop,
}: ComplexityScaleProps) {
  const current = clampStop(stop)
  const onFraction = usePointerFraction((fraction) => {
    if (readOnly) return
    const next = clampStop(Math.round(fraction * 4))
    if (next !== current) onStop?.(next)
  })

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (readOnly) return
    let delta = 0
    if (event.key === 'ArrowRight' || event.key === 'ArrowUp') delta = 1
    if (event.key === 'ArrowLeft' || event.key === 'ArrowDown') delta = -1
    if (delta === 0) return
    event.preventDefault()
    const next = clampStop(current + delta)
    if (next !== current) onStop?.(next)
  }

  const controlStyle: CSSProperties = {
    position: 'relative',
    width: '100%',
    height: '14px',
    cursor: readOnly ? 'default' : 'pointer',
    outline: 'none',
  }
  const trackStyle: CSSProperties = {
    position: 'absolute',
    top: '5px',
    left: 0,
    right: 0,
    height: '5px',
    borderRadius: '3px',
    background: 'linear-gradient(to right, var(--color-accent-800), var(--color-accent-500))',
  }

  return (
    <div data-testid="complexity-scale">
      <div
        data-testid="complexity-control"
        role="slider"
        aria-label={profileName ? `${profileName} complexity` : 'Complexity'}
        aria-valuemin={0}
        aria-valuemax={4}
        aria-valuenow={current}
        tabIndex={readOnly ? undefined : 0}
        onPointerDown={readOnly ? undefined : onFraction}
        onKeyDown={readOnly ? undefined : onKeyDown}
        style={controlStyle}
      >
        <span data-testid="complexity-track" style={trackStyle}>
          {[0, 1, 2, 3, 4].map((tick) => (
            <span
              key={tick}
              data-testid="complexity-tick"
              style={{
                position: 'absolute',
                top: '-4px',
                left: `${tick * 25}%`,
                width: '1px',
                height: '13px',
                background: 'color-mix(in srgb,var(--color-text) 20%,transparent)',
              }}
            />
          ))}
          <i
            data-testid="complexity-knob"
            aria-hidden="true"
            style={{
              position: 'absolute',
              top: '-5px',
              left: `calc(${current * 25}% - 7px)`,
              width: '15px',
              height: '15px',
              borderRadius: '50%',
              background: 'var(--color-bg)',
              boxShadow: KNOB_SHADOW,
            }}
          />
        </span>
      </div>
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          marginTop: '9px',
          fontFamily: 'var(--font-mono)',
          fontSize: '9px',
          color: 'color-mix(in srgb,var(--color-text) 45%,transparent)',
        }}
      >
        <span>{labels[0]}</span>
        <span>{labels[1]}</span>
      </div>
      {profileName ? (
        <div
          style={{
            marginTop: '15px',
            textAlign: 'center',
            fontFamily: 'var(--font-heading)',
            fontSize: '15px',
            color: 'var(--color-accent-300)',
          }}
        >
          {profileName}
        </div>
      ) : null}
    </div>
  )
}
