import { useEffect, type FormEvent, type MouseEvent, type ReactNode } from 'react'
import { cx } from '../../utils/cx'
import styles from './settings.module.css'

export interface SettingsModalProps {
  /** Visibility is controlled; omitted defaults to visible for simple confirmation usage. */
  open?: boolean
  /** Alias for `open`. */
  isOpen?: boolean
  title?: ReactNode
  description?: ReactNode
  children?: ReactNode
  /** Render custom footer controls instead of the convenience confirm/cancel controls. */
  actions?: ReactNode
  onClose?: () => void
  /** Optional form submit callback for create/edit forms. */
  onSubmit?: () => void
  onConfirm?: () => void
  confirmLabel?: string
  cancelLabel?: string
  confirmDisabled?: boolean
  closeOnBackdrop?: boolean
  className?: string
}

export function SettingsModal({
  open,
  isOpen,
  title,
  description,
  children,
  actions,
  onClose,
  onSubmit,
  onConfirm,
  confirmLabel = 'Save',
  cancelLabel = 'Cancel',
  confirmDisabled = false,
  closeOnBackdrop = true,
  className,
}: SettingsModalProps) {
  const visible = isOpen ?? open ?? true

  useEffect(() => {
    if (!visible || !onClose) return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [onClose, visible])

  if (!visible) return null

  const hasConvenienceActions = onConfirm !== undefined || onSubmit !== undefined || onClose !== undefined
  const submit = onSubmit ?? onConfirm

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    submit?.()
  }

  function handleBackdropClick(event: MouseEvent<HTMLDivElement>) {
    if (closeOnBackdrop && event.target === event.currentTarget) onClose?.()
  }

  return (
    <div className={styles.modalBackdrop} onMouseDown={handleBackdropClick}>
      <div
        className={cx(styles.modal, className)}
        role="dialog"
        aria-modal="true"
        aria-labelledby={title !== undefined ? 'settings-modal-title' : undefined}
      >
        {(title !== undefined || onClose !== undefined) && (
          <header className={styles.modalHeader}>
            {title !== undefined && (
              <h2 className={styles.modalTitle} id="settings-modal-title">
                {title}
              </h2>
            )}
            {onClose !== undefined && (
              <button type="button" className={styles.modalClose} aria-label="Close" onClick={onClose}>
                ×
              </button>
            )}
          </header>
        )}
        {description !== undefined && <p className={styles.modalDescription}>{description}</p>}
        <form onSubmit={handleSubmit}>
          <div className={styles.modalBody}>{children}</div>
          {(actions !== undefined || hasConvenienceActions) && (
            <div className={styles.modalActions}>
              {actions}
              {actions === undefined && (
                <>
                  {onClose !== undefined && (
                    <button type="button" className={cx(styles.modalButton, styles.modalButtonSecondary)} onClick={onClose}>
                      {cancelLabel}
                    </button>
                  )}
                  {submit !== undefined && (
                    <button
                      type="submit"
                      className={cx(styles.modalButton, styles.modalButtonPrimary)}
                      disabled={confirmDisabled}
                    >
                      {confirmLabel}
                    </button>
                  )}
                </>
              )}
            </div>
          )}
        </form>
      </div>
    </div>
  )
}
