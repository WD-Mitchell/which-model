import { useMemo } from 'react'
import { useQuery, type UseQueryResult } from '@tanstack/react-query'
import type {
  BenchmarkDetail,
  CatalogSummary,
  Favourite,
  GroupDetail,
  GroupSummary,
  GUISettings,
  HarnessInfo,
  ProfileDetail,
  ProfileSummary,
  ProviderDetail,
  ProviderInfo,
  RankResponse,
  ShellSnippets,
  UsageDTO,
} from '@which-model/core'
import { getHost } from './host'
import { useOverridesStore } from './overrides'

// U05 §2.11 — one typed TanStack Query hook per U00 CONTRACTS §6 canonical
// key. Thin wrappers over getHost().

export function useProfiles(): UseQueryResult<ProfileSummary[]> {
  return useQuery({ queryKey: ['profiles'], queryFn: () => getHost().profiles.list() })
}

export function useProfile(slug: string): UseQueryResult<ProfileDetail> {
  return useQuery({
    queryKey: ['profile', slug],
    queryFn: () => getHost().profiles.get(slug),
    enabled: Boolean(slug),
  })
}

export function useComplexityScale(): UseQueryResult<string[]> {
  return useQuery({ queryKey: ['complexity-scale'], queryFn: () => getHost().profiles.complexityScale() })
}

// Stable stringify: sorted object keys so key order never changes a hash.
export function stableStringify(value: unknown): string {
  if (value === null || typeof value !== 'object') return JSON.stringify(value)
  if (Array.isArray(value)) return `[${value.map(stableStringify).join(',')}]`
  const obj = value as Record<string, unknown>
  const keys = Object.keys(obj).sort()
  return `{${keys.map((k) => `${JSON.stringify(k)}:${stableStringify(obj[k])}`).join(',')}}`
}

// U05 CONTRACTS §2 — the canonical overrides hash: stable JSON of the
// overrides DTO, or 'none' when absent.
export function overridesHashOf(o: ProfileDetail | null): string {
  return o === null ? 'none' : stableStringify(o)
}

function mapEqual(a: Record<string, number>, b: Record<string, number>): boolean {
  const ak = Object.keys(a)
  const bk = Object.keys(b)
  if (ak.length !== bk.length) return false
  for (const k of ak) if (a[k] !== b[k]) return false
  return true
}

// Reassemble the overrides DTO from the U06 store (U06 §2.3) when it is
// dirty for the active slug; null (⇒ hash 'none', overrides omitted) when
// clean or not seeded.
function useReassembledOverrides(slug: string, base: ProfileDetail | undefined): ProfileDetail | null {
  const baseSlug = useOverridesStore((s) => s.baseSlug)
  const coreShare = useOverridesStore((s) => s.coreShare)
  const tier1 = useOverridesStore((s) => s.tier1)
  const tier2 = useOverridesStore((s) => s.tier2)
  return useMemo(() => {
    if (!base || baseSlug !== slug) return null
    const dirty =
      base.core_share !== coreShare ||
      !mapEqual(base.tier1_weights, tier1) ||
      !mapEqual(base.tier2_weights, tier2)
    if (!dirty) return null
    return { ...base, core_share: coreShare, tier1_weights: tier1, tier2_weights: tier2 }
  }, [base, baseSlug, slug, coreShare, tier1, tier2])
}

/** Current overrides hash for a profile slug ('none' when clean). */
export function useOverridesHash(slug: string): string {
  const { data: base } = useProfile(slug)
  const overrides = useReassembledOverrides(slug, base)
  return overridesHashOf(overrides)
}

export function useRank(
  slug: string,
  overridesHash: string,
  holds: number,
): UseQueryResult<RankResponse> {
  const { data: base } = useProfile(slug)
  const overrides = useReassembledOverrides(slug, base)
  return useQuery({
    queryKey: ['rank', slug, overridesHash, holds],
    queryFn: () =>
      getHost().pick.rank({
        profile_slug: slug,
        overrides: overrides ?? undefined,
        holds,
      }),
    enabled: Boolean(slug),
  })
}

export function useCatalogLine(): UseQueryResult<CatalogSummary> {
  return useQuery({ queryKey: ['catalog-line'], queryFn: () => getHost().pick.catalogLine() })
}

export function useGroups(): UseQueryResult<GroupSummary[]> {
  return useQuery({ queryKey: ['groups'], queryFn: () => getHost().catalog.groups() })
}

export function useGroupDetail(slug: string): UseQueryResult<GroupDetail> {
  return useQuery({
    queryKey: ['group', slug],
    queryFn: () => getHost().catalog.groupDetail(slug),
    enabled: Boolean(slug),
  })
}

export function useBenchmarks(): UseQueryResult<string[]> {
  return useQuery({ queryKey: ['benchmarks'], queryFn: () => getHost().catalog.benchmarks() })
}

export function useBenchmarkDetail(name: string): UseQueryResult<BenchmarkDetail> {
  return useQuery({
    queryKey: ['benchmark', name],
    queryFn: () => getHost().catalog.benchmarkDetail(name),
    enabled: Boolean(name),
  })
}

export function useProviders(): UseQueryResult<ProviderInfo[]> {
  return useQuery({ queryKey: ['providers'], queryFn: () => getHost().providers.list() })
}

export function useProviderDetail(id: string): UseQueryResult<ProviderDetail> {
  return useQuery({
    queryKey: ['provider', id],
    queryFn: () => getHost().providers.detail(id),
    enabled: Boolean(id),
  })
}

export function useHarnesses(): UseQueryResult<HarnessInfo[]> {
  return useQuery({ queryKey: ['harnesses'], queryFn: () => getHost().harnesses.list() })
}

export function useUsage(force: boolean): UseQueryResult<UsageDTO[]> {
  return useQuery({ queryKey: ['usage', force], queryFn: () => getHost().usage.snapshots(force) })
}

export function useFavourites(): UseQueryResult<Favourite[]> {
  return useQuery({ queryKey: ['favourites'], queryFn: () => getHost().favourites.list() })
}

export function useSettings(): UseQueryResult<GUISettings> {
  return useQuery({ queryKey: ['settings'], queryFn: () => getHost().settings.get() })
}

export function useSnippets(): UseQueryResult<ShellSnippets> {
  return useQuery({ queryKey: ['snippets'], queryFn: () => getHost().settings.shellSnippets() })
}