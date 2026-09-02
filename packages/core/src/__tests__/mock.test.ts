import { describe, expect, it } from 'vitest'
import { isEngineError } from '../errors.js'
import type { GUISettings, ProfileDetail, RankedModel } from '../types.js'
import { createMockEngineHost, MOCK_NOW } from '../mock.js'

async function expectCode(p: Promise<unknown>, code: string): Promise<void> {
  let caught: unknown
  try {
    await p
  } catch (e) {
    caught = e
  }
  expect(caught).toBeDefined()
  expect(isEngineError(caught)).toBe(true)
  if (isEngineError(caught)) expect(caught.code).toBe(code)
}

const customProfile: ProfileDetail = {
  slug: 'my_custom',
  name: 'My Custom',
  builtin: false,
  core_share: 50,
  tier1_weights: { intelligence: 5 },
  tier2_weights: { reasoning: 5 },
  picks: 0,
  last_used: '',
}

describe('createMockEngineHost — CRUD round-trips', () => {
  it('profiles.save (new custom) then list contains it', async () => {
    const host = createMockEngineHost()
    await host.profiles.save(customProfile)
    const list = await host.profiles.list()
    expect(list.map((p) => p.slug)).toContain('my_custom')
    const got = await host.profiles.get('my_custom')
    expect(got).toEqual(customProfile)
  })

  it('duplicate appends _copy, then _copy_2', async () => {
    const host = createMockEngineHost()
    const first = await host.profiles.duplicate('research')
    expect(first.slug).toBe('research_copy')
    expect(first.builtin).toBe(false)
    const second = await host.profiles.duplicate('research')
    expect(second.slug).toBe('research_copy_2')
    const slugs = (await host.profiles.list()).map((p) => p.slug)
    expect(slugs).toContain('research_copy')
    expect(slugs).toContain('research_copy_2')
  })

  it('delete removes a custom profile', async () => {
    const host = createMockEngineHost()
    await host.profiles.save(customProfile)
    await host.profiles.delete('my_custom')
    const slugs = (await host.profiles.list()).map((p) => p.slug)
    expect(slugs).not.toContain('my_custom')
  })

  it('complexityScale returns the five complexity slugs in order', async () => {
    const host = createMockEngineHost()
    expect(await host.profiles.complexityScale()).toEqual([
      'simple_action_execution',
      'simple_implementation',
      'balanced_implementation',
      'research',
      'planning',
    ])
  })

  it('resolved values are deep copies — mutating results does not corrupt data', async () => {
    const host = createMockEngineHost()
    const list = await host.profiles.list()
    list[0]!.name = 'clobbered'
    list[0]!.tier1_weights['intelligence'] = 0
    const again = await host.profiles.list()
    expect(again[0]!.name).not.toBe('clobbered')
  })
})

describe('createMockEngineHost — EngineError codes', () => {
  it('save/delete builtin profile → builtin_readonly', async () => {
    const host = createMockEngineHost()
    const builtin = await host.profiles.get('research')
    await expectCode(host.profiles.save({ ...builtin, name: 'x' }), 'builtin_readonly')
    await expectCode(host.profiles.delete('research'), 'builtin_readonly')
  })

  it("get('nope') → not_found", async () => {
    await expectCode(createMockEngineHost().profiles.get('nope'), 'not_found')
  })

  it('saveGroup renameTo onto an existing slug → conflict', async () => {
    const host = createMockEngineHost()
    const dup = await host.catalog.duplicateGroup('reasoning')
    expect(dup.slug).toBe('reasoning_copy')
    await expectCode(
      host.catalog.saveGroup('reasoning_copy', ['AIME'], 'knowledge'),
      'conflict',
    )
  })

  it('builtin group mutation → builtin_readonly', async () => {
    const host = createMockEngineHost()
    await expectCode(host.catalog.saveGroup('reasoning', ['AIME']), 'builtin_readonly')
    await expectCode(host.catalog.deleteGroup('reasoning'), 'builtin_readonly')
  })

  it("favourites.pin('bad key') → validation_failed", async () => {
    await expectCode(createMockEngineHost().favourites.pin('bad key'), 'validation_failed')
  })

  it('builtin harness save/delete → builtin_readonly; unknown → not_found', async () => {
    const host = createMockEngineHost()
    const claude = (await host.harnesses.list()).find((h) => h.slug === 'claude')!
    await expectCode(host.harnesses.save({ ...claude, name: 'x' }), 'builtin_readonly')
    await expectCode(host.harnesses.delete('claude'), 'builtin_readonly')
    await expectCode(host.harnesses.delete('nope'), 'not_found')
  })

  it('malformed route key to launch → validation_failed', async () => {
    const host = createMockEngineHost()
    await expectCode(
      host.harnesses.launch('claude', 'not-a-route-key', 'research'),
      'validation_failed',
    )
  })
})

describe('createMockEngineHost — event firing', () => {
  it('each mutation fires its mapped event exactly once, synchronously', async () => {
    const host = createMockEngineHost()
    const fired: { event: string; payload: unknown }[] = []
    host.on('config:changed', (payload) => fired.push({ event: 'config:changed', payload }))
    host.on('catalog:changed', (payload) => fired.push({ event: 'catalog:changed', payload }))
    host.on('usage:updated', (payload) => fired.push({ event: 'usage:updated', payload }))
    host.on('settings:changed', (payload) => fired.push({ event: 'settings:changed', payload }))
    host.on('pick:recorded', (payload) => fired.push({ event: 'pick:recorded', payload }))

    const take = () => {
      const got = fired.splice(0)
      expect(got.length).toBe(1)
      return got[0]!
    }

    // profiles.save / delete / duplicate → config:changed {section:'profiles'}
    await host.profiles.save(customProfile)
    expect(take()).toEqual({ event: 'config:changed', payload: { section: 'profiles' } })
    await host.profiles.duplicate('my_custom')
    expect(take()).toEqual({ event: 'config:changed', payload: { section: 'profiles' } })
    await host.profiles.delete('my_custom_copy')
    expect(take()).toEqual({ event: 'config:changed', payload: { section: 'profiles' } })

    // catalog mutations → catalog:changed {}
    await host.catalog.duplicateGroup('reasoning')
    expect(take()).toEqual({ event: 'catalog:changed', payload: {} })
    await host.catalog.saveGroup('reasoning_copy', ['AIME'])
    expect(take()).toEqual({ event: 'catalog:changed', payload: {} })
    await host.catalog.deleteGroup('reasoning_copy')
    expect(take()).toEqual({ event: 'catalog:changed', payload: {} })

    // provider mutations → config:changed {section:'providers'}
    await host.providers.setEnabled('cursor', true)
    expect(take()).toEqual({ event: 'config:changed', payload: { section: 'providers' } })
    await host.providers.reorder(['codex', 'claude', 'copilot', 'cursor', 'google', 'mistral', 'xai'])
    expect(take()).toEqual({ event: 'config:changed', payload: { section: 'providers' } })
    await host.providers.setRouteEnabled('claude', 'claude-opus-5', 'max', false)
    expect(take()).toEqual({ event: 'config:changed', payload: { section: 'providers' } })
    await host.providers.setAllRoutes('claude', true)
    expect(take()).toEqual({ event: 'config:changed', payload: { section: 'providers' } })

    // harness mutations → config:changed {section:'harnesses'}
    await host.harnesses.setProvider('claude', 'cursor', true)
    expect(take()).toEqual({ event: 'config:changed', payload: { section: 'harnesses' } })
    await host.harnesses.setAllProviders('claude', true)
    expect(take()).toEqual({ event: 'config:changed', payload: { section: 'harnesses' } })

    // launch / recordPick → pick:recorded
    await host.pick.recordPick('research', 'claude/claude-opus-5@max')
    expect(take()).toEqual({
      event: 'pick:recorded',
      payload: { profile_slug: 'research', route_key: 'claude/claude-opus-5@max' },
    })
    await host.harnesses.launch('claude', 'claude/claude-opus-5@max', 'research')
    expect(take()).toEqual({
      event: 'pick:recorded',
      payload: { profile_slug: 'research', route_key: 'claude/claude-opus-5@max' },
    })

    // favourites → config:changed {section:'favourites'}; idempotent no-ops still fire
    await host.favourites.pin('claude/claude-opus-5@max')
    expect(take()).toEqual({ event: 'config:changed', payload: { section: 'favourites' } })
    await host.favourites.unpin('claude/claude-opus-5@max')
    expect(take()).toEqual({ event: 'config:changed', payload: { section: 'favourites' } })
    await host.favourites.unpin('claude/claude-opus-5@max') // absent — still fires
    expect(take()).toEqual({ event: 'config:changed', payload: { section: 'favourites' } })

    // settings.set → settings:changed (new GUISettings)
    const settings: GUISettings = { ...(await host.settings.get()), holds: 3 }
    fired.splice(0)
    await host.settings.set(settings)
    const settingsEvt = take()
    expect(settingsEvt.event).toBe('settings:changed')
    expect(settingsEvt.payload).toEqual(settings)

    // usage → usage:updated {}
    await host.usage.snapshots(true)
    expect(take()).toEqual({ event: 'usage:updated', payload: {} })
    await host.usage.setMode('on')
    expect(take()).toEqual({ event: 'usage:updated', payload: {} })
    await host.usage.setBackend('native')
    expect(take()).toEqual({ event: 'usage:updated', payload: {} })

    // reads and window.* fire nothing
    await host.profiles.list()
    await host.usage.snapshots(false)
    await host.window.openSettings()
    await host.window.hidePopover()
    expect(fired).toEqual([])
  })

  it('dispatch happens before the promise resolves', async () => {
    const host = createMockEngineHost()
    let seenBeforeResolve = false
    host.on('config:changed', () => {
      seenBeforeResolve = true
    })
    const p = host.profiles.save(customProfile).then(() => seenBeforeResolve)
    expect(await p).toBe(true)
  })

  it('unsubscribe stops delivery; unsubscribing during dispatch is safe', async () => {
    const host = createMockEngineHost()
    let a = 0
    let b = 0
    let offA = () => {}
    offA = host.on('config:changed', () => {
      a++
      offA() // unsubscribe self during dispatch
    })
    const offB = host.on('config:changed', () => {
      b++
    })
    await host.profiles.save(customProfile)
    await host.profiles.delete('my_custom')
    expect(a).toBe(1)
    expect(b).toBe(2)
    offB()
    await host.profiles.save(customProfile)
    expect(b).toBe(2)
  })
})

describe('createMockEngineHost — rank', () => {
  const golden: RankedModel[] = [
    { rank: 1, model_id: 'claude-sonnet-5.2', model_name: 'Claude Sonnet 5.2', provider: 'claude', reasoning: 'high', score: 88.42, route_key: 'claude/claude-sonnet-5.2@high', intelligence: 4.2, cost: 4.4, speed: 4.6 },
    { rank: 2, model_id: 'grok-5-fast', model_name: 'Grok 5 Fast', provider: 'copilot', reasoning: 'medium', score: 85.77, route_key: 'copilot/grok-5-fast@medium', intelligence: 3.8, cost: 4.7, speed: 5 },
    { rank: 3, model_id: 'gpt-5.6-sol', model_name: 'GPT-5.6 Sol', provider: 'codex', reasoning: 'high', score: 85.42, route_key: 'codex/gpt-5.6-sol@high', intelligence: 4.4, cost: 4, speed: 4.4 },
    { rank: 4, model_id: 'gpt-5.6-luna', model_name: 'GPT-5.6 Luna', provider: 'codex', reasoning: 'max', score: 83.85, route_key: 'codex/gpt-5.6-luna@max', intelligence: 5, cost: 3, speed: 3.5 },
    { rank: 5, model_id: 'claude-opus-5', model_name: 'Claude Opus 5', provider: 'claude', reasoning: 'max', score: 80.5, route_key: 'claude/claude-opus-5@max', intelligence: 4.9, cost: 2.6, speed: 3.2 },
  ]

  it('is deterministic and matches the golden fixture (2dp scores)', async () => {
    const host = createMockEngineHost()
    const first = await host.pick.rank({ profile_slug: 'balanced_implementation', holds: 5 })
    const second = await host.pick.rank({ profile_slug: 'balanced_implementation', holds: 5 })
    expect(first).toEqual(second)
    expect(first.total).toBe(6)
    expect(first.candidates).toEqual(golden)
  })

  it('disabling provider codex reroutes gpt-5.6-luna to copilot', async () => {
    const host = createMockEngineHost()
    await host.providers.setEnabled('codex', false)
    const res = await host.pick.rank({ profile_slug: 'balanced_implementation', holds: 5 })
    const luna = res.candidates.find((c) => c.model_id === 'gpt-5.6-luna')!
    expect(luna.provider).toBe('copilot')
    expect(luna.route_key).toBe('copilot/gpt-5.6-luna@max')
  })

  it("disabling a model's only route drops it and decrements total", async () => {
    const host = createMockEngineHost()
    const before = await host.pick.rank({ profile_slug: 'balanced_implementation', holds: 5 })
    await host.providers.setRouteEnabled('claude', 'claude-opus-5', 'max', false)
    const after = await host.pick.rank({ profile_slug: 'balanced_implementation', holds: 5 })
    expect(after.total).toBe(before.total - 1)
    expect(after.candidates.map((c) => c.model_id)).not.toContain('claude-opus-5')
  })

  it('holds 0 uses settings.holds; rank fires no event', async () => {
    const host = createMockEngineHost()
    const fired: string[] = []
    host.on('config:changed', () => fired.push('config:changed'))
    host.on('pick:recorded', () => fired.push('pick:recorded'))
    const res = await host.pick.rank({ profile_slug: 'balanced_implementation', holds: 0 })
    expect(res.candidates.length).toBe(5) // default settings.holds = 5
    expect(fired).toEqual([])
  })

  it('rejects holds values the engine rejects', async () => {
    const host = createMockEngineHost()
    await expect(
      host.pick.rank({ profile_slug: 'balanced_implementation', holds: 50 }),
    ).rejects.toMatchObject({
      code: 'validation_failed',
      message: 'holds 50 must be 1, 3 or 5',
    })
  })

  it('rank with overrides ignores the named profile weights and fires no event', async () => {
    const host = createMockEngineHost()
    const fired: string[] = []
    host.on('config:changed', () => fired.push('x'))
    host.on('pick:recorded', () => fired.push('x'))
    const overrides: ProfileDetail = {
      slug: 'balanced_implementation',
      name: 'Balanced Implementation',
      builtin: true,
      core_share: 90,
      tier1_weights: { cost: 5 },
      tier2_weights: {},
      picks: 0,
      last_used: '',
    }
    const res = await host.pick.rank({
      profile_slug: 'balanced_implementation',
      overrides,
      holds: 5,
    })
    // cost-only ranking puts llama-5-405b (cost 5.0) first, not the golden order.
    expect(res.candidates[0]!.model_id).toBe('llama-5-405b')
    expect(fired).toEqual([])
  })
})

describe('createMockEngineHost — clock determinism and overrides merge', () => {
  it('launch sets picks +1 and last_used === MOCK_NOW', async () => {
    const host = createMockEngineHost()
    const before = await host.profiles.get('research')
    const result = await host.harnesses.launch('claude', 'claude/claude-opus-5@max', 'research')
    expect(result.copied).toBe(false) // copy_command_instead default false
    expect(result.command).toBe('claude --model claude-opus-5 --reasoning max')
    const after = await host.profiles.get('research')
    expect(after.picks).toBe(before.picks + 1)
    expect(after.last_used).toBe(MOCK_NOW)
  })

  it('two fresh hosts have identical fixture data', () => {
    const a = createMockEngineHost()
    const b = createMockEngineHost()
    expect(a.data).toEqual(b.data)
  })

  it('MOCK_NOW is the fixed base clock', () => {
    expect(MOCK_NOW).toBe('2026-01-01T12:00:00Z')
  })

  it('createMockEngineHost({settings}) honours the shallow merge', async () => {
    const base = createMockEngineHost()
    const settings: GUISettings = { ...base.data.settings, holds: 3, layout: 'list' }
    const host = createMockEngineHost({ settings })
    expect((await host.settings.get()).holds).toBe(3)
    expect((await host.settings.get()).layout).toBe('list')
    // other top-level keys untouched
    expect(host.data.profiles).toEqual(base.data.profiles)
    const res = await host.pick.rank({ profile_slug: 'balanced_implementation', holds: 0 })
    expect(res.candidates.length).toBe(3)
  })
})

describe('createMockEngineHost — remaining surfaces', () => {
  it('catalogLine reflects live data', async () => {
    const host = createMockEngineHost()
    expect(await host.pick.catalogLine()).toEqual({ models: 8, providers_on: 3, harnesses: 7 })
    await host.providers.setEnabled('cursor', true)
    expect((await host.pick.catalogLine()).providers_on).toBe(4)
  })

  it('usage.snapshots excludes disabled providers', async () => {
    const host = createMockEngineHost()
    const snaps = await host.usage.snapshots(false)
    expect(snaps.map((s) => s.provider)).toEqual(['claude', 'codex', 'copilot'])
    expect(snaps[0]!.failure).toBe('')
  })

  it('groups and benchmark detail follow the fixture rules', async () => {
    const host = createMockEngineHost()
    const groups = await host.catalog.groups()
    expect(groups.length).toBe(11)
    const se = groups.find((g) => g.slug === 'software_engineering')!
    expect(se.builtin).toBe(true)
    expect(se.benchmark_count).toBe(24)

    const detail = await host.catalog.groupDetail('reasoning')
    expect(detail.benchmarks.length).toBe(86) // full catalogue, on marks membership
    const on = detail.benchmarks.filter((b) => b.on).map((b) => b.name)
    expect(on.sort()).toEqual(['GPQA Diamond', 'FrontierMath', 'ARC-AGI-2', 'AIME', 'HMMT'].sort())
    expect(detail.benchmarks.every((b) => b.covered === 8 && b.coverage_total === 8)).toBe(true)

    const bench = await host.catalog.benchmarkDetail('MMMU Pro')
    expect(bench.note).toBe('')
    expect(bench.groups).toEqual(['knowledge', 'evidence_capture', 'ui_visual'])
    expect(bench.rows.length).toBe(8)
    const norms = bench.rows.map((r) => r.norm)
    expect([...norms].sort((a, b) => b - a)).toEqual(norms)
    expect(Math.max(...norms)).toBe(100)

    const model = await host.catalog.modelDetail('Claude Opus 5', 'max')
    expect(model.model).toBe('Claude Opus 5')
    expect(model.reasoning).toBe('max')
    expect(model.rows.length).toBeGreaterThan(0)
    expect(model.rows.some((r) => r.name === 'MMMU Pro')).toBe(true)
    expect(Math.max(...model.rows.map((r) => r.norm))).toBe(100)
    const missing = await host.catalog.modelDetail('No Such Model', 'max')
    expect(missing.rows).toEqual([])
  })

  it('provider detail lists catalogue models and every effort level', async () => {
    const host = createMockEngineHost()
    const claude = await host.providers.detail('claude')
    expect(claude.models.map((m) => m.model_id)).toEqual([
      'claude-haiku-4',
      'claude-opus-5',
      'claude-sonnet-5.2',
    ])
    const opus = claude.models.find((m) => m.model_id === 'claude-opus-5')!
    expect(opus.levels.map((l) => l.reasoning)).toEqual(['low', 'high', 'max'])
    const haiku = claude.models.find((m) => m.model_id === 'claude-haiku-4')!
    expect(haiku.levels.map((l) => l.reasoning)).toEqual(['low', 'medium', 'high'])
    const untested = await host.catalog.modelDetail('Claude Haiku 4', 'low')
    expect(untested.rows).toEqual([])
  })

  it('favourites round-trip with route labels', async () => {
    const host = createMockEngineHost()
    await host.favourites.pin('claude/claude-opus-5@max')
    const favs = await host.favourites.list()
    expect(favs).toEqual([
      {
        route_key: 'claude/claude-opus-5@max',
        model_name: 'Claude Opus 5',
        route_label: 'claude · max',
        in_range: true,
      },
    ])
    await host.providers.setEnabled('claude', false)
    expect((await host.favourites.list())[0]!.in_range).toBe(false)
  })

  it('window methods no-op and usage.mode round-trips', async () => {
    const host = createMockEngineHost()
    await expect(host.window.quit()).resolves.toBeUndefined()
    await expect(host.window.copyToClipboard('x')).resolves.toBeUndefined()
    expect(await host.usage.mode()).toEqual({ mode: 'auto', backend: 'native' })
    await host.usage.setMode('off')
    await host.usage.setBackend('codexbar')
    expect(await host.usage.mode()).toEqual({ mode: 'off', backend: 'codexbar' })
  })
})
