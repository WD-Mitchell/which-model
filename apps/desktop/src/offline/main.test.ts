import { beforeEach, expect, it, vi } from 'vitest'

const call = vi.hoisted(() => vi.fn())
vi.mock('@wailsio/runtime', () => ({ Call: { ByName: call } }))

beforeEach(() => {
  vi.resetModules()
  call.mockReset()
  document.body.innerHTML = '<select id="profile"></select><select id="top"><option>3</option><option>10</option></select><p id="error"></p><p id="summary"></p><ol id="results"></ol><pre id="capabilities"></pre>'
})

it('starts and changes rankings through the fully qualified Wails bindings', async () => {
  const prefix = 'github.com/WD-Mitchell/which-model/pkg/offlinedesktop.API.'
  call.mockImplementation(async (name: string) => {
    if (name === prefix + 'Profiles') return ['balanced_implementation', 'review']
    if (name === prefix + 'Capabilities') return { artifact: 'which-model-score-only-desktop' }
    if (name === prefix + 'Rank') return { ranking: { candidate_count: 1, recommendation: { model: 'Bundled model', reasoning: 'default', total_score: '0.9' }, alternatives: [] } }
    throw new Error('Unknown bound method: ' + name)
  })
  await import('./main')
  await vi.waitFor(() => expect(document.querySelector('#results h2')?.textContent).toBe('Bundled model'))
  expect(call).toHaveBeenCalledWith(prefix + 'Rank', 'balanced_implementation', 3)
  const profile = document.querySelector<HTMLSelectElement>('#profile')!
  profile.value = 'review'
  profile.dispatchEvent(new Event('change'))
  await vi.waitFor(() => expect(call).toHaveBeenCalledWith(prefix + 'Rank', 'review', 3))
  expect(document.querySelector('#error')?.textContent).toBe('')
})
