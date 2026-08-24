import { fireEvent, render, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { BalanceSlider } from '../BalanceSlider'
import { ComplexityScale } from '../ComplexityScale'
import { WeightEditor } from '../WeightEditor'
import { WeightRow } from '../WeightRow'

function rect(width = 100): DOMRect {
  return {
    left: 0,
    top: 0,
    right: width,
    bottom: 20,
    width,
    height: 20,
    x: 0,
    y: 0,
    toJSON: () => ({}),
  } as DOMRect
}

function mockRect(element: HTMLElement, width = 100) {
  element.getBoundingClientRect = () => rect(width)
}

function drag(element: HTMLElement, ...clientX: number[]) {
  mockRect(element)
  fireEvent.pointerDown(element, { clientX: clientX[0] ?? 0, clientY: 5 })
  clientX.slice(1).forEach((x) => fireEvent.pointerMove(window, { clientX: x, clientY: 5 }))
  fireEvent.pointerUp(window)
}

describe('WeightRow', () => {
  it.each(['step', 'bar', 'slider'] as const)('maps %s drag fractions and renders the value', (variant) => {
    const onChange = vi.fn()
    const { getByRole, getByTestId, getAllByTestId } = render(<WeightRow variant={variant} label="cost" value={2} onChange={onChange} />)
    const control = getByRole('slider')
    mockRect(control)
    fireEvent.pointerDown(control, { clientX: 50, clientY: 5 })
    fireEvent.pointerMove(window, { clientX: 90, clientY: 5 })
    fireEvent.pointerUp(window)
    // usePointerFraction fires on pointerdown and on each pointermove, and the
    // onFraction handler emits on every distinct mapped value (see U02 §2.4).
    expect(onChange).toHaveBeenCalledTimes(2)
    expect(onChange).toHaveBeenNthCalledWith(1, 3)
    expect(onChange).toHaveBeenLastCalledWith(5)
    expect(control).toHaveAttribute('aria-valuenow', '2')

    if (variant === 'step') expect(getAllByTestId('weight-step')).toHaveLength(5)
    if (variant === 'bar') expect(getByTestId('weight-bar-fill')).toHaveStyle({ width: '40%' })
    if (variant === 'slider') {
      expect(getByTestId('weight-slider-fill')).toHaveStyle({ width: '40%' })
      expect(getByTestId('weight-slider-knob')).toHaveStyle({ left: 'calc(40% - 6px)' })
    }
  })

  // min={1} is what the popover's editor and the core axes pass: the engine
  // rejects a weight outside (0, 5], so 0 is not "off" there, it is a profile
  // that cannot be ranked. Dropping a metric is the row's × button instead.
  it('floors drags and arrow keys at min instead of reaching 0', () => {
    const onChange = vi.fn()
    const { getByRole } = render(
      <WeightRow variant="slider" label="cost" value={2} min={1} onChange={onChange} />,
    )
    const control = getByRole('slider')
    expect(control).toHaveAttribute('aria-valuemin', '1')

    mockRect(control)
    fireEvent.pointerDown(control, { clientX: 0, clientY: 5 })
    fireEvent.pointerUp(window)
    expect(onChange).toHaveBeenLastCalledWith(1)

    onChange.mockClear()
    fireEvent.keyDown(control, { key: 'ArrowLeft' })
    expect(onChange).toHaveBeenCalledWith(1)
  })

  // The floor sits at the track's left END, not a fifth of the way in: with
  // min=1 the five values spread across the whole track, so 1 is 0% and 5 is
  // 100%. Anything else leaves a stretch of track that cannot be reached.
  it('spreads a 1..5 row across the whole track', () => {
    const { getByTestId, rerender, getAllByTestId } = render(
      <WeightRow variant="slider" label="cost" value={1} min={1} />,
    )
    expect(getByTestId('weight-slider-fill')).toHaveStyle({ width: '0%' })
    expect(getByTestId('weight-slider-knob')).toHaveStyle({ left: 'calc(0% - 6px)' })

    rerender(<WeightRow variant="slider" label="cost" value={3} min={1} />)
    expect(getByTestId('weight-slider-fill')).toHaveStyle({ width: '50%' })

    rerender(<WeightRow variant="slider" label="cost" value={5} min={1} />)
    expect(getByTestId('weight-slider-fill')).toHaveStyle({ width: '100%' })

    // The step track spends its segments on the same range: four of them.
    rerender(<WeightRow variant="step" label="cost" value={1} min={1} />)
    expect(getAllByTestId('weight-step')).toHaveLength(4)
  })

  // Halfway along a 1..5 track is 3, not the 2.5 a 0..5 mapping would round.
  it('maps drag fractions onto [min, 5]', () => {
    const onChange = vi.fn()
    const { getByRole } = render(
      <WeightRow variant="slider" label="cost" value={1} min={1} onChange={onChange} />,
    )
    const control = getByRole('slider')
    mockRect(control)
    fireEvent.pointerDown(control, { clientX: 50, clientY: 5 })
    fireEvent.pointerUp(window)
    expect(onChange).toHaveBeenLastCalledWith(3)
  })

  // A row already at the floor emits nothing rather than re-emitting it, so a
  // drag along the dead zone cannot spam the store.
  it('emits nothing when already at min', () => {
    const onChange = vi.fn()
    const { getByRole } = render(
      <WeightRow variant="bar" label="cost" value={1} min={1} onChange={onChange} />,
    )
    const control = getByRole('slider')
    mockRect(control)
    fireEvent.pointerDown(control, { clientX: 2, clientY: 5 })
    fireEvent.pointerUp(window)
    expect(onChange).not.toHaveBeenCalled()
  })

  // Default stays 0: the settings profile editor has no × button, so dropping a
  // TASK benchmark to "ignored" is the only way to remove one there.
  it('still allows 0 when min is not set', () => {
    const onChange = vi.fn()
    const { getByRole } = render(
      <WeightRow variant="bar" label="ui_visual" value={2} valueStyle="verbose" onChange={onChange} />,
    )
    const control = getByRole('slider')
    expect(control).toHaveAttribute('aria-valuemin', '0')
    mockRect(control)
    fireEvent.pointerDown(control, { clientX: 0, clientY: 5 })
    fireEvent.pointerUp(window)
    expect(onChange).toHaveBeenLastCalledWith(0)
  })

  it('suppresses repeated mapped values and removes pointer listeners after pointerup', () => {
    const onChange = vi.fn()
    const { getByRole } = render(<WeightRow variant="bar" label="speed" value={3} onChange={onChange} />)
    const control = getByRole('slider')
    mockRect(control)
    fireEvent.pointerDown(control, { clientX: 60, clientY: 5 })
    fireEvent.pointerMove(window, { clientX: 61, clientY: 5 })
    expect(onChange).not.toHaveBeenCalled()
    fireEvent.pointerUp(window)
    fireEvent.pointerMove(window, { clientX: 90, clientY: 5 })
    expect(onChange).not.toHaveBeenCalled()
  })

  it('supports keyboard changes and read-only controls', () => {
    const onChange = vi.fn()
    const { rerender, getByRole } = render(<WeightRow variant="slider" label="quality" value={5} onChange={onChange} />)
    const control = getByRole('slider')
    fireEvent.keyDown(control, { key: 'ArrowRight' })
    expect(onChange).not.toHaveBeenCalled()
    rerender(<WeightRow variant="slider" label="quality" value={5} onChange={onChange} />)
    fireEvent.keyDown(getByRole('slider'), { key: 'ArrowLeft' })
    expect(onChange).toHaveBeenCalledWith(4)

    rerender(<WeightRow variant="slider" label="quality" value={2} readOnly onChange={onChange} />)
    const readOnlyControl = getByRole('slider')
    expect(readOnlyControl).not.toHaveAttribute('tabindex')
    expect(readOnlyControl).not.toHaveAttribute('onpointerdown')
    fireEvent.keyDown(readOnlyControl, { key: 'ArrowRight' })
    expect(onChange).toHaveBeenCalledTimes(1)
  })
})

describe('BalanceSlider', () => {
  it('maps pointer positions to the clamped five-point balance scale', () => {
    const onChange = vi.fn()
    const { getByRole, getByTestId } = render(<BalanceSlider core={60} onChange={onChange} showRatio />)
    const control = getByRole('slider')
    mockRect(control)
    fireEvent.pointerDown(control, { clientX: 50, clientY: 5 })
    fireEvent.pointerMove(window, { clientX: 0, clientY: 5 })
    fireEvent.pointerMove(window, { clientX: 100, clientY: 5 })
    fireEvent.pointerMove(window, { clientX: 51, clientY: 5 })
    fireEvent.pointerUp(window)
    // Long list: pointerdown at 50 fires immediately, then each pointermove.
    expect(onChange.mock.calls.map(([value]) => value)).toEqual([50, 10, 90, 50])
    expect(getByTestId('balance-core-bar')).toHaveStyle({ flex: '60' })
    expect(getByTestId('balance-task-bar')).toHaveStyle({ flex: '40' })
    expect(getByTestId('balance-slider')).toHaveTextContent('60 / 40')
  })

  it('supports keyboard changes and read-only mode', () => {
    const onChange = vi.fn()
    const { rerender, getByRole } = render(<BalanceSlider core={90} onChange={onChange} />)
    const control = getByRole('slider')
    fireEvent.keyDown(control, { key: 'ArrowRight' })
    fireEvent.keyDown(control, { key: 'ArrowLeft' })
    expect(onChange).toHaveBeenCalledWith(85)

    rerender(<BalanceSlider core={60} readOnly onChange={onChange} />)
    const readOnlyControl = getByRole('slider')
    expect(readOnlyControl).not.toHaveAttribute('tabindex')
    fireEvent.keyDown(readOnlyControl, { key: 'ArrowRight' })
    expect(onChange).toHaveBeenCalledTimes(1)
  })
})

describe('ComplexityScale', () => {
  it('renders all stops and maps pointer positions', () => {
    const onStop = vi.fn()
    const { getByRole, getAllByTestId, getByTestId } = render(<ComplexityScale stop={1} onStop={onStop} profileName="Balanced" />)
    const control = getByRole('slider')
    mockRect(control)
    fireEvent.pointerDown(control, { clientX: 50, clientY: 5 })
    fireEvent.pointerMove(window, { clientX: 70, clientY: 5 })
    fireEvent.pointerMove(window, { clientX: 100, clientY: 5 })
    fireEvent.pointerUp(window)
    // pointerdown at 50 fires stop 2 immediately, then each pointermove.
    expect(onStop.mock.calls.map(([value]) => value)).toEqual([2, 3, 4])
    expect(getAllByTestId('complexity-tick')).toHaveLength(5)
    expect(getByTestId('complexity-knob')).toHaveStyle({ left: 'calc(25% - 7px)' })
  })

  it('clamps keyboard changes and omits focusability when read-only', () => {
    const onStop = vi.fn()
    const { rerender, getByRole } = render(<ComplexityScale stop={4} onStop={onStop} />)
    const control = getByRole('slider')
    fireEvent.keyDown(control, { key: 'ArrowRight' })
    fireEvent.keyDown(control, { key: 'ArrowLeft' })
    expect(onStop).toHaveBeenCalledWith(3)

    rerender(<ComplexityScale stop={2} readOnly onStop={onStop} />)
    const readOnlyControl = getByRole('slider')
    expect(readOnlyControl).not.toHaveAttribute('tabindex')
  })
})

describe('WeightEditor', () => {
  const rows = {
    coreRows: [
      { key: 'intelligence', value: 3 },
      { key: 'cost', value: 2, accent: true },
      { key: 'speed', value: 0 },
    ],
    taskRows: [
      { key: 'mmlu', value: 3 },
      { key: 'gpqa', value: 1 },
    ],
  }

  it('composes popover rows, section percentages, remove buttons, and actions', () => {
    const onChangeWeight = vi.fn()
    const onRemoveWeight = vi.fn()
    const onAddMetric = vi.fn()
    const onToggleAdd = vi.fn()
    const onRevert = vi.fn()
    const { getByText, getByRole, getAllByRole, getByTestId } = render(
      <WeightEditor
        variant="popover"
        sliderStyle="step"
        {...rows}
        sectionPcts={{ core: '60%', task: '40%' }}
        addOpen
        addable={['mmlu', 'gpqa']}
        onChangeWeight={onChangeWeight}
        onRemoveWeight={onRemoveWeight}
        onAddMetric={onAddMetric}
        onToggleAdd={onToggleAdd}
        onRevert={onRevert}
      />,
    )
    expect(getByText('core benchmarks')).toBeInTheDocument()
    expect(getByText('(higher = better, cheaper, faster)')).toBeInTheDocument()
    expect(getByText('60%')).toBeInTheDocument()
    expect(getByText('40%')).toBeInTheDocument()
    expect(getAllByRole('button', { name: /remove/i })).toHaveLength(2)
    fireEvent.click(getAllByRole('button', { name: /remove/i })[0])
    expect(onRemoveWeight).toHaveBeenCalledWith('mmlu')
    // 'mmlu' is both a task row label and an addable option in the '+ Add metric'
    // popup; scope the click to the option inside the add popup.
    fireEvent.click(within(getByTestId('weight-editor-add-popup')).getByText('mmlu'))
    expect(onAddMetric).toHaveBeenCalledWith('mmlu')
    fireEvent.click(getByRole('button', { name: '+ Add metric' }))
    fireEvent.click(getByRole('button', { name: 'Revert' }))
    expect(onToggleAdd).toHaveBeenCalledTimes(1)
    expect(onRevert).toHaveBeenCalledTimes(1)
    expect(getByTestId('weight-editor')).toBeInTheDocument()
  })

  it('renders profile detail values without trailing remove controls', () => {
    const onChangeWeight = vi.fn()
    const { getByText, getAllByText, queryByRole, getAllByRole } = render(
      <WeightEditor
        variant="profile-detail"
        sliderStyle="bar"
        coreRows={rows.coreRows}
        taskRows={rows.taskRows}
        sectionPcts={{ core: '60%', task: '40%' }}
        readOnly
        onChangeWeight={onChangeWeight}
        onRemoveWeight={vi.fn()}
      />,
    )
    // verbose value style renders `${current} / 5` per row; intelligence and
    // mmlu both have value 3, so '3 / 5' appears twice (core + task) and 'ignored'
    // once (speed = 0).
    expect(getAllByText('3 / 5')).toHaveLength(2)
    expect(getByText('ignored')).toBeInTheDocument()
    expect(queryByRole('button', { name: /remove/i })).not.toBeInTheDocument()
    expect(getAllByRole('slider')).toHaveLength(5)
    getAllByRole('slider').forEach((control) => expect(control).not.toHaveAttribute('tabindex'))
  })
})
