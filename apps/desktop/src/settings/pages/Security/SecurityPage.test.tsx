import { afterEach, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createMockEngineHost } from '@which-model/core/mock'
import { resetHost } from '../../../lib/host'
import SecurityPage from './SecurityPage'

afterEach(() => {cleanup();resetHost()})
function setup(configure?: (host: ReturnType<typeof createMockEngineHost>) => void) {
 const host=createMockEngineHost();configure?.(host);resetHost(host)
 const client=new QueryClient({defaultOptions:{queries:{retry:false}}})
 render(<QueryClientProvider client={client}><SecurityPage/></QueryClientProvider>)
 return host
}
it('requires a category and explicit confirmation before deleting records',async()=>{
 const host=setup();const purge=vi.spyOn(host.administration,'maintain')
 const button=await screen.findByRole('button',{name:'Delete selected records'})
 expect((button as HTMLButtonElement).disabled).toBe(true)
 fireEvent.click(screen.getByLabelText('Pick history'))
 expect((button as HTMLButtonElement).disabled).toBe(true)
 fireEvent.click(screen.getByLabelText('I confirm deletion of the selected product records'))
 fireEvent.click(button)
 await waitFor(()=>expect(purge).toHaveBeenCalledWith('purge',['pick_history'],'',true))
 await waitFor(()=>expect((button as HTMLButtonElement).disabled).toBe(true))
})
it('keeps secure verification and residual-copy failures visible',async()=>{
 const host=setup();vi.spyOn(host.administration,'migrate').mockResolvedValue({provider:'codex',secure_store:'verified',legacy_copy:'recovery',recovery_file:'.which-model-migration-test',error:'Source removal incomplete'})
 fireEvent.click(await screen.findByRole('button',{name:'Migrate credential'}))
 expect(await screen.findByText('OS store: verified. Legacy copy: recovery.')).toBeDefined()
 expect(screen.getByText('Source removal incomplete')).toBeDefined()
 expect(screen.getByText(/Recovery file beside/)).toBeDefined()
})
it('persists native OS storage selection through the administration API',async()=>{
 const host=setup();const set=vi.spyOn(host.administration,'setNativeStore')
 fireEvent.click(await screen.findByLabelText('Use native OS credential storage'))
 await waitFor(()=>expect(set).toHaveBeenCalledWith(true))
})

it('renders deny-all company policy lists and keeps native storage administrator-controlled', async () => {
 setup(host => {
  vi.spyOn(host.administration, 'status').mockResolvedValue({
   native_keychain: true, use_keychain: true,
   policy: {managed: true, required: true, origin: 'protected-policy', sha256: 'a'.repeat(64), policy: {
    schema_version: 1, name: 'Deny-all company policy',
    // Go nil slices are encoded as null by the policy inspection API.
    allowed_providers: null, credential_sources: null, executables: null, codexbar_installations: null,
    secure_store_only: true, identity_free: true, allow_custom_shell: false, allow_credential_migration: false,
    retention: {usage_snapshots_hours: 24, launch_logs_days: 7, pick_history_days: 30, audit_records_days: 30},
    integrations: {skill_installation: false, hook_installation: false, hook_use: false},
   }},
  } as unknown as Awaited<ReturnType<typeof host.administration.status>>)
 })
 expect(await screen.findByText('Deny-all company policy')).toBeDefined()
 expect(screen.getByText('No harness executable is approved.')).toBeDefined()
 expect(screen.getByText('No CodexBar installation is approved.')).toBeDefined()
 expect(screen.queryByLabelText('Use native OS credential storage')).toBeNull()
 expect((screen.getByRole('button', {name: 'Migrate credential'}) as HTMLButtonElement).disabled).toBe(true)
})
