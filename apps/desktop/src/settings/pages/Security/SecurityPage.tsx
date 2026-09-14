import { useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import type { DelegationCheck, MaintenanceResult, MigrationResult, PrivacyCategory } from '@which-model/core'
import { getHost } from '../../../lib/host'
import { DetailHeader } from '../../DetailHeader'
import styles from './SecurityPage.module.css'

const categories: Array<[PrivacyCategory, string]> = [['usage_snapshots', 'Usage snapshots'], ['pick_history', 'Pick history'], ['audit_records', 'Audit records'], ['launch_logs', 'Launch logs']]
const message = (e: unknown) => e && typeof e === 'object' && 'message' in e ? String(e.message) : 'The operation could not be completed.'

export default function SecurityPage() {
  const host = getHost()
  const status = useQuery({queryKey: ['administration'], queryFn: () => host.administration.status(), refetchInterval: 60_000})
  const providers = useQuery({queryKey: ['providers'], queryFn: () => host.providers.list()})
  const inFlight = useRef(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [provider, setProvider] = useState('codex')
  const [remove, setRemove] = useState(false)
  const [replace, setReplace] = useState(false)
  const [migration, setMigration] = useState<MigrationResult>()
  const [maintenance, setMaintenance] = useState<MaintenanceResult>()
  const [checks, setChecks] = useState<DelegationCheck[]>()
  const [selected, setSelected] = useState<PrivacyCategory[]>([])
  const [project, setProject] = useState('')
  const [confirmed, setConfirmed] = useState(false)
  const state = status.data
  const policy = state?.policy.policy
  const managed = state?.policy.managed ?? false
  const canMigrate = !managed || Boolean(policy?.allow_credential_migration && policy.credential_sources?.includes('keychain') && policy.allowed_providers?.includes(provider))
  const run = async (action: () => Promise<void>) => {
    if (inFlight.current) return
    inFlight.current = true
    setBusy(true); setError('')
    try { await action(); await status.refetch() } catch (e) { setError(message(e)) } finally { inFlight.current = false; setBusy(false) }
  }
  const maintain = (operation: 'cleanup' | 'purge') => run(async () => {
    setMaintenance(await host.administration.maintain(operation, selected, project.trim(), operation === 'purge' && confirmed))
    setConfirmed(false)
  })
  const last = maintenance ?? state?.last_maintenance
  return <div className={styles.page}>
    <DetailHeader title="Security & privacy" blurb="Inspect company controls and manage data stored by which-model." />
    {status.isPending && <p>Loading settings…</p>}
    {(error || status.error) && <p role="alert" className={styles.error}>{error || message(status.error)}</p>}
    {state && <>
      <section>
        <h2>{managed ? policy?.name ?? 'Company-managed installation' : 'Personal installation'}</h2>
        <p>{managed ? 'These controls are set by your administrator. Desktop preferences cannot grant permissions.' : 'No company policy is active. Personal settings and explicit data removal are available below.'}</p>
        {managed && policy && <>
          <dl><dt>Policy source</dt><dd>{state.policy.origin}</dd><dt>Policy digest</dt><dd className={styles.digest}>{state.policy.sha256}</dd><dt>Enrollment required</dt><dd>{state.policy.required ? 'Yes' : 'No'}</dd><dt>Allowed providers</dt><dd>{policy.allowed_providers?.join(', ') || 'None'}</dd><dt>Credential sources</dt><dd>{policy.credential_sources?.join(', ') || 'None'}</dd><dt>Secure storage only</dt><dd>{policy.secure_store_only ? 'Yes' : 'No'}</dd><dt>Identity-free records</dt><dd>{policy.identity_free ? 'Yes' : 'No'}</dd></dl>
          <dl><dt>Skill installation</dt><dd>{policy.integrations.skill_installation ? 'Allowed' : 'Not allowed'}</dd><dt>Hook installation / use</dt><dd>{policy.integrations.hook_installation ? 'Allowed' : 'Not allowed'} / {policy.integrations.hook_use ? 'Allowed' : 'Not allowed'}</dd><dt>Custom execution</dt><dd>{policy.allow_custom_shell ? 'Allowed with an approved image' : 'Not allowed'}</dd><dt>Credential migration</dt><dd>{policy.allow_credential_migration ? 'Allowed for permitted sources/providers' : 'Not allowed'}</dd></dl>
          <h3>Approved executables</h3>
          {!policy.executables?.length ? <p>No harness executable is approved.</p> : policy.executables.map(exe => <details key={exe.id}><summary>{exe.id}</summary><p>{exe.path}</p><code>{exe.sha256}</code><pre>{JSON.stringify(exe.args, null, 2)}</pre>{exe.inputs?.map(input => <p key={input.path}>{input.path}<br/><code>{input.sha256}</code></p>)}</details>)}
          <h3>CodexBar approval</h3>
          {!policy.codexbar_installations?.length ? <p>No CodexBar installation is approved.</p> : <>
            {policy.codexbar_installations.map(entry => <details key={entry.path}><summary>{entry.path}</summary><p>Image digest</p><code>{entry.sha256}</code><p>Configuration: {entry.config.path}</p><code>{entry.config.sha256}</code></details>)}
            <button disabled={busy} onClick={() => run(async () => setChecks(await host.administration.verifyDelegation()))}>Verify approved files</button>
            <p>This checks protected files without launching CodexBar. Files are checked again before use.</p>
            {checks?.map(check => <p key={check.path} role="status">{check.path}: {check.verified ? 'Image and configuration verified' : check.error}</p>)}
          </>}
        </>}
      </section>
      <section>
        <h2>Credential storage</h2>
        {managed ? <p>Native OS storage is administrator-controlled.</p> : <label className={styles.option}><input type="checkbox" checked={state.native_keychain && state.use_keychain} disabled={busy} onChange={e => run(() => host.administration.setNativeStore(e.target.checked))}/>Use native OS credential storage</label>}
        <p>Migration copies only which-model-owned legacy credentials. Provider-owned credentials are unchanged. A different OS credential is preserved unless replacement is selected.</p>
        <label>Provider<select aria-label="Migration provider" value={provider} disabled={busy} onChange={e => {setProvider(e.target.value); setMigration(undefined)}}>{Array.from(new Set(['codex', ...(providers.data?.map(p => p.id) ?? []), ...(managed ? ['artificial-analysis'] : [])])).map(id => <option key={id}>{id}</option>)}</select></label>
        <label className={styles.option}><input type="checkbox" checked={remove} disabled={busy} onChange={e => setRemove(e.target.checked)}/>Remove the legacy source after verifying the OS copy</label>
        <label className={styles.option}><input type="checkbox" checked={replace} disabled={busy} onChange={e => setReplace(e.target.checked)}/>Replace a different existing which-model OS credential</label>
        {!canMigrate && <p>Migration for this provider is not permitted by company policy.</p>}
        <button disabled={busy || !canMigrate} onClick={() => run(async () => setMigration(await host.administration.migrate(provider, remove, replace)))}>Migrate credential</button>
        {migration && <div role="status"><p>OS store: {migration.secure_store}. Legacy copy: {migration.legacy_copy}.</p>{migration.recovery_file && <p>Recovery file beside the source: {migration.recovery_file}</p>}{migration.error && <p className={styles.error}>{migration.error}</p>}</div>}
      </section>
      <section>
        <h2>Retention & cleanup</h2>
        {policy && managed ? <dl><dt>Usage snapshots</dt><dd>{policy.retention.usage_snapshots_hours} hours</dd><dt>Launch logs</dt><dd>{policy.retention.launch_logs_days} days</dd><dt>Pick history</dt><dd>{policy.retention.pick_history_days} days</dd><dt>Audit records</dt><dd>{policy.retention.audit_records_days} days</dd></dl> : <p>Automatic company retention is inactive. You can explicitly delete selected product records.</p>}
        <p>Cleanup removes expired records and applies company minimization. Purge deletes all owned records in the selected categories. Credentials, provider files, configuration and catalogs are excluded. Enabled collection can create new records afterward.</p>
        <fieldset disabled={busy}><legend>Record categories</legend>{categories.map(([id, label]) => <label key={id} className={styles.option}><input type="checkbox" checked={selected.includes(id)} onChange={e => {setSelected(old => e.target.checked ? [...old,id] : old.filter(c => c !== id));setConfirmed(false)}}/>{label}</label>)}</fieldset>
        <label>Previous project (optional absolute path)<input aria-label="Previous project" value={project} disabled={busy} onChange={e => {setProject(e.target.value);setConfirmed(false)}}/></label>
        <p>Only the two owned legacy audit files in the selected project are considered. No project scan is performed.</p>
        <button disabled={busy || !managed} onClick={() => maintain('cleanup')}>Clean up expired records</button>
        <p>With no categories selected, cleanup covers all categories. Automatic maintenance runs every minute while the app is running; stopped apps need scheduled CLI cleanup.</p>
        <label className={styles.option}><input type="checkbox" checked={confirmed} disabled={busy || !selected.length} onChange={e => setConfirmed(e.target.checked)}/>I confirm deletion of the selected product records</label>
        <button disabled={busy || !confirmed || !selected.length} onClick={() => maintain('purge')}>Delete selected records</button>
        {last && <div role="status"><h3>{last.error ? 'Maintenance incomplete' : 'Maintenance completed'}</h3><p>{last.operation} · {last.completed_at}</p>{last.error && <p className={styles.error}>{last.error}</p>}<div className={styles.table}><table><thead><tr><th>Category</th><th>Retained</th><th>Removed</th><th>Scrubbed</th><th>Files deleted</th><th>Failures</th></tr></thead><tbody>{Object.entries(last.categories).map(([key,r]) => <tr key={key}><th>{categories.find(([id]) => id===key)?.[1] ?? key}</th><td>{r.retained_records}</td><td>{r.removed_records}</td><td>{r.scrubbed_records}</td><td>{r.deleted_files}</td><td>{r.failed_files}</td></tr>)}</tbody></table></div></div>}
      </section>
    </>}
  </div>
}
