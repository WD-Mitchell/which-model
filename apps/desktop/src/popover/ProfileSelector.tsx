import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useToast } from '@which-model/ui'
import { getHost } from '../lib/host'
import { useSettings, useUserProfiles } from '../lib/queries'

/** Shared persisted selection. Keep the previous value until the write succeeds. */
export function ProfileSelector() {
  const { data: settings } = useSettings()
  const { data: profiles } = useUserProfiles()
  const [saving, setSaving] = useState(false)
  const client = useQueryClient()
  const toast = useToast()
  return (
    <label className="work-profile-selector">
      <span>Profile</span>
      <select
        aria-label="Profile"
        value={settings?.user_profile ?? 'software_engineering'}
        disabled={!settings || !profiles || saving}
        onChange={async (e) => {
          if (!settings) return
          const next = { ...settings, user_profile: e.target.value }
          setSaving(true)
          try {
            await getHost().settings.set(next)
            client.setQueryData(['settings'], next)
          } catch (err) {
            toast.show((err as { message?: string }).message ?? 'Could not save profile')
          } finally { setSaving(false) }
        }}
      >
        {(profiles ?? []).map((p) => <option key={p.slug} value={p.slug}>{p.name}</option>)}
      </select>
    </label>
  )
}
