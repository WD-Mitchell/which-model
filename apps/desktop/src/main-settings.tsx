// settings.html entry — U07: settings window root.
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createRoot } from 'react-dom/client'
import { ToastProvider } from '@which-model/ui'
import { getHost } from './lib/host'
import { useEngineEvents } from './lib/invalidate'
import { SettingsApp } from './settings/SettingsApp'
// Inter Variable, not the static cuts: the Nocturne canvas renders Google
// Fonts' Inter, which is the variable face. Its optical-size axis makes 9-13px
// text — most of this UI — visibly looser and taller than the static 400/500,
// which is why the app read as "nearly right" against the design.
import '@fontsource-variable/inter'
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