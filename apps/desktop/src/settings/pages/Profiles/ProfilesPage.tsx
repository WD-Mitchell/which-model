import { useProfiles, useSettings, useUserProfiles } from '../../../lib/queries'
import { ProfileSelector } from '../../../popover/ProfileSelector'
import '../../../popover/LandingView.css'
import { DetailHeader } from '../../DetailHeader'
import { PAGE_META } from '../../pages'
import styles from './ProfilesPage.module.css'

export default function ProfilesPage() {
  const { data: profiles, isError } = useUserProfiles()
  const { data: useCases } = useProfiles()
  const { data: settings } = useSettings()
  return (
    <div className={styles.page}>
      <DetailHeader title={PAGE_META.Profiles[0]} blurb={PAGE_META.Profiles[1]} />
      <ProfileSelector />
      <p className={styles.note}>A profile chooses your starting use cases. Each use case has its own ranking weights, and you can always browse the full list.</p>
      {isError && <p role="alert">Could not load profiles.</p>}
      {!profiles && !isError && <p>Loading profiles…</p>}
      {profiles?.map((p) => (
        <section key={p.slug} className={styles.card} aria-label={p.name}>
          <h2>{p.name} {settings?.user_profile === p.slug && <span className={styles.active}>Selected</span>}</h2>
          <p>{p.description}</p>
          <ul>{p.use_case_slugs.map((slug) => <li key={slug}>
            {useCases?.find((u) => u.slug === slug)?.name ?? slug.replaceAll('_', ' ')}
            {p.default_use_case === slug && <span className={styles.default}> · default</span>}
          </li>)}</ul>
        </section>
      ))}
    </div>
  )
}
