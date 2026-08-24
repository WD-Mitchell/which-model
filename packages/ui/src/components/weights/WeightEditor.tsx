import type { CSSProperties, ReactNode } from 'react'
import { WeightRow, type WeightVariant } from './WeightRow'

export interface WeightEditorRow {
  key: string
  value: number
  accent?: boolean
}

export interface WeightEditorProps {
  variant: 'popover' | 'profile-detail'
  sliderStyle: WeightVariant
  coreRows: WeightEditorRow[]
  taskRows: WeightEditorRow[]
  sectionPcts: { core: string; task: string }
  readOnly?: boolean
  addable?: string[]
  addOpen?: boolean
  onChangeWeight?: (key: string, v: number) => void
  onRemoveWeight?: (key: string) => void
  onAddMetric?: (key: string) => void
  onToggleAdd?: () => void
  onRevert?: () => void
  /** Extra buttons in the action row, left of Revert. The popover puts its
   *  weights actions (Copy model id / Save as profile) here — they act on the
   *  weights, and the popover footer is now tab-independent. */
  extraActions?: ReactNode
}


const HEADER_COLOR = 'color-mix(in srgb,var(--color-text) 42%,transparent)'
const PCT_COLOR = 'color-mix(in srgb,var(--color-text) 62%,transparent)'
const POPUP_TEXT = 'color-mix(in srgb,var(--color-text) 80%,transparent)'

function SectionHeader({
  children,
  percentage,
}: {
  children: ReactNode
  percentage: string
}) {
  return (
    <div style={{ display: 'flex', alignItems: 'baseline', gap: '8px' }}>
      <span
        style={{
          color: HEADER_COLOR,
          fontSize: '10px',
          letterSpacing: '.11em',
          textTransform: 'uppercase',
        }}
      >
        {children}
      </span>
      <span
        style={{
          marginLeft: 'auto',
          color: PCT_COLOR,
          fontFamily: 'var(--font-mono)',
          fontSize: '10.5px',
        }}
      >
        {percentage}
      </span>
    </div>
  )
}

function Rows({
  rows,
  sliderStyle,
  labelWidth,
  valueStyle,
  readOnly,
  removable,
  onChangeWeight,
  onRemoveWeight,
}: {
  rows: WeightEditorRow[]
  sliderStyle: WeightVariant
  labelWidth: 104 | 150
  valueStyle: 'compact' | 'verbose'
  readOnly: boolean
  removable: boolean
  onChangeWeight?: (key: string, v: number) => void
  onRemoveWeight?: (key: string) => void
}) {
  return (
    <>
      {rows.map((row) => (
        <WeightRow
          key={row.key}
          variant={sliderStyle}
          label={row.key}
          value={row.value}
          accent={row.accent}
          labelWidth={labelWidth}
          valueStyle={valueStyle}
          readOnly={readOnly}
          // 1..5, never 0: a metric is dropped with the row's × button, and a
          // 0 would be a weight the engine refuses to rank on.
          min={1}
          onChange={onChangeWeight ? (value) => onChangeWeight(row.key, value) : undefined}
          onRemove={removable && !readOnly && onRemoveWeight ? () => onRemoveWeight(row.key) : undefined}
        />
      ))}
    </>
  )
}

export function WeightEditor({
  variant,
  sliderStyle,
  coreRows,
  taskRows,
  sectionPcts,
  readOnly = false,
  addable = [],
  addOpen = false,
  onChangeWeight,
  onRemoveWeight,
  onAddMetric,
  onToggleAdd,
  onRevert,
  extraActions,
}: WeightEditorProps) {
  const isPopover = variant === 'popover'
  const labelWidth = isPopover ? 104 : 150
  const valueStyle = isPopover ? 'compact' : 'verbose'

  const actionRowStyle: CSSProperties = {
    display: 'flex',
    alignItems: 'center',
    gap: '8px',
    position: 'relative',
    padding: '2px 0 10px',
  }
  const buttonStyle: CSSProperties = {
    fontSize: '11.5px',
    padding: '2px 6px',
  }

  return (
    <div
      data-testid="weight-editor"
      style={{
        display: 'flex',
        flexDirection: 'column',
        gap: '12px',
      }}
    >
      <SectionHeader percentage={sectionPcts.core}>
        core benchmarks{' '}
        <span style={{ textTransform: 'none', letterSpacing: 0 }}>(higher = better, cheaper, faster)</span>
      </SectionHeader>
      <Rows
        rows={coreRows}
        sliderStyle={sliderStyle}
        labelWidth={labelWidth}
        valueStyle={valueStyle}
        readOnly={readOnly}
        removable={false}
        onChangeWeight={onChangeWeight}
      />
      <SectionHeader percentage={sectionPcts.task}>task benchmarks</SectionHeader>
      <Rows
        rows={taskRows}
        sliderStyle={sliderStyle}
        labelWidth={labelWidth}
        valueStyle={valueStyle}
        readOnly={readOnly}
        removable={isPopover}
        onChangeWeight={onChangeWeight}
        onRemoveWeight={onRemoveWeight}
      />
      {isPopover ? (
        <div style={actionRowStyle}>
          <button type="button" className="btn btn-ghost" style={buttonStyle} onClick={onToggleAdd}>
            + Add metric
          </button>
          <span style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: 4 }}>
            {extraActions}
            <button type="button" className="btn btn-ghost" style={buttonStyle} onClick={onRevert}>
              Revert
            </button>
          </span>
          {addOpen ? (
            <span
              data-testid="weight-editor-add-popup"
              style={{
                position: 'absolute',
                left: 0,
                bottom: '26px',
                zIndex: 9,
                width: '180px',
                maxHeight: '150px',
                padding: '5px',
                display: 'flex',
                flexDirection: 'column',
                gap: '1px',
                overflow: 'auto',
                borderRadius: '8px',
                background: 'var(--color-surface)',
                boxShadow: 'var(--shadow-md)',
              }}
            >
              {addable.map((key) => (
                <span
                  key={key}
                  role="button"
                  tabIndex={0}
                  onClick={() => onAddMetric?.(key)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter' || event.key === ' ') {
                      event.preventDefault()
                      onAddMetric?.(key)
                    }
                  }}
                  style={{
                    padding: '6px 9px',
                    borderRadius: '5px',
                    color: POPUP_TEXT,
                    fontFamily: 'var(--font-mono)',
                    fontSize: '11.5px',
                    cursor: 'pointer',
                  }}
                >
                  {key}
                </span>
              ))}
            </span>
          ) : null}
        </div>
      ) : null}
    </div>
  )
}
