import '@fontsource/inter/400.css'
import '@fontsource/inter/500.css'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ToastProvider } from '@which-model/ui'
import { PopoverApp } from './popover'
import { useEngineEvents } from './lib'
import './app.css'
import '../../../packages/ui/dist/theme/nocturne.css'
import '../../../packages/ui/dist/theme/app.css'

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false } },
})

// Subscribes once to every host event and invalidates the U00 §5 map.
function EventsBridge() {
  useEngineEvents()
  return null
}

function Root() {
  return (
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        <EventsBridge />
        <PopoverApp />
      </ToastProvider>
    </QueryClientProvider>
  )
}

createRoot(document.getElementById('root')!).render(<Root />)