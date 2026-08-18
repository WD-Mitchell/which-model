import './Header.css'

export interface PopoverHeaderProps {
  onToggleMenu(): void
}

export function PopoverHeader({ onToggleMenu }: PopoverHeaderProps) {
  return (
    <div className="ph-row">
      <span className="ph-brand">which-model</span>
      <button
        type="button"
        className="ib ph-hamburger"
        aria-label="App menu"
        onClick={(e) => {
          e.stopPropagation()
          onToggleMenu()
        }}
      >
        <svg
          width="13"
          height="13"
          viewBox="0 0 16 16"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.5"
          strokeLinecap="round"
        >
          <path d="M1.8 4.2h12.4M1.8 8h12.4M1.8 11.8h12.4"></path>
          <circle cx="5.4" cy="4.2" r="1.7" fill="var(--color-bg)"></circle>
          <circle cx="10.6" cy="11.8" r="1.7" fill="var(--color-bg)"></circle>
        </svg>
      </button>
    </div>
  )
}