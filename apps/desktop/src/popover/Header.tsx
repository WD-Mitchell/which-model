import './Header.css'

/**
 * Popover header: the app's strapline and its catalogue line.
 *
 * The hamburger that used to sit here is gone — the app menu is the tray's
 * right-click menu now. What remains sits ABOVE the tab strip, because it
 * describes the app rather than either tab.
 */
export interface PopoverHeaderProps {
  /** "412 models · 3 providers on · 4 harnesses", or "—" while it loads. */
  catalogLine: string
}

export function PopoverHeader({ catalogLine }: PopoverHeaderProps) {
  return (
    <div className="ph-hero">
      {/* The mockup's strapline with the product name spliced into its head:
          "Which Model" is the app, so it carries the title's size and weight,
          and the rest of the sentence trails it at strapline scale. */}
      <h1 className="ph-title">
        <span className="ph-titleBrand">Which Model</span> for the task at hand
      </h1>
      <div className="mono ph-catalog">{catalogLine}</div>
    </div>
  )
}
