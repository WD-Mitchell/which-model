async page => {
 await page.evaluate(async()=>{
  const {getHost}=await import('/src/lib/host.ts')
  const host=getHost()
  host.administration.status=async()=>({native_keychain:true,use_keychain:true,policy:{managed:true,required:true,origin:'/Library/Application Support/which-model/company-policy.json',sha256:'a'.repeat(64),policy:{schema_version:1,name:'Company profile',allowed_providers:['codex','claude'],credential_sources:['keychain'],secure_store_only:true,identity_free:true,retention:{usage_snapshots_hours:24,launch_logs_days:7,pick_history_days:30,audit_records_days:30},integrations:{skill_installation:false,hook_installation:false,hook_use:true},executables:[{id:'codex',path:'/opt/company/codex',sha256:'b'.repeat(64),args:['-m','{model_id}']}],codexbar_installations:[{path:'/opt/company/codexbar',sha256:'c'.repeat(64),config:{path:'/opt/company/codexbar.json',sha256:'d'.repeat(64)}}],allow_custom_shell:false,allow_credential_migration:true}}})
  host.administration.migrate=async provider=>({provider,secure_store:'verified',legacy_copy:'recovery',recovery_file:'.which-model-migration-example',error:'Secure credential verified; legacy recovery copy could not be removed.'})
  host.administration.maintain=async operation=>({managed:true,operation,completed_at:'2026-09-14T11:00:00Z',categories:{pick_history:{retained_records:0,removed_records:3,scrubbed_records:0,deleted_files:1,failed_files:1}},error:'Company privacy pick_history: cleanup is incomplete.'})
 })
 await page.getByRole('button',{name:'General',exact:true}).click()
 await page.getByRole('button',{name:'Security & privacy',exact:true}).click()
 // Refresh via the query's normal interval, using focus instead of touching data.
 await page.evaluate(()=>window.dispatchEvent(new Event('visibilitychange')))
}
