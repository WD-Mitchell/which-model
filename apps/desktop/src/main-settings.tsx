// settings.html entry — U07: settings window root.
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createRoot } from 'react-dom/client'
import { ToastProvider } from '@which-model/ui'
import { getHost } from './lib/host'
import { useEngineEvents } from './lib/invalidate'
import { SettingsApp } from './settings/SettingsApp'
import '@fontsource/inter/400.css'
import '@fontsource/inter/500.css'
import '@fontsource/inter/600.css'
import '../../../packages/ui/dist/theme/nocturne.css'
import '../../../packages/ui/dist/theme/app.css'
import './settings.css'

const queryClient = new QueryClient({
  defaultOptions: { queries: { staleTime: 30_000, retry: 1 } },
})

function SettingsRoot() {
  useEngineEvents()
  return <SettingsApp host={getHost()} />
}

createRoot(document.getElementById('root')!).render(
  <QueryClientProvider client={queryClient}>
    <ToastProvider>
      <SettingsRoot />
    </ToastProvider>
  </QueryClientProvider>,
)