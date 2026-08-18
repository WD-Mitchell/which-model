import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import styles from './Toast.module.css'

export interface ToastHandle {
  show: (message: string) => void // 2.6s, single, replace
}

export const TOAST_DURATION_MS = 2600

const ToastContext = createContext<ToastHandle | null>(null)

export function ToastProvider(props: { children: ReactNode }) {
  const [message, setMessage] = useState<string | null>(null)
  const timerRef = useRef<number | null>(null)

  useEffect(() => {
    return () => {
      if (timerRef.current !== null) window.clearTimeout(timerRef.current)
    }
  }, [])

  const show = useCallback((next: string) => {
    setMessage(next)
    if (timerRef.current !== null) window.clearTimeout(timerRef.current)
    timerRef.current = window.setTimeout(() => {
      setMessage(null)
      timerRef.current = null
    }, TOAST_DURATION_MS)
  }, [])

  const value = useMemo(() => ({ show }), [show])

  return (
    <ToastContext.Provider value={value}>
      {props.children}
      {message !== null && <div className={styles.toast}>{message}</div>}
    </ToastContext.Provider>
  )
}

export function useToast(): ToastHandle {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast requires ToastProvider')
  return ctx
}