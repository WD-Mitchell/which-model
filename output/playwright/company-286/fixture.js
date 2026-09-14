async (page) => {
  await page.setViewportSize({width: 500, height: 620})
  await page.evaluate(async () => {
    const {getHost} = await import('/src/lib/host.ts')
    const host = getHost()
    const rank = host.pick.rank.bind(host.pick)
    host.pick.rank = async (request) => {
      const result = await rank(request)
      return {...result, recommendation_mode: 'score_only', candidates: result.candidates.map(candidate => ({
        ...candidate, quota_evidence: {state: 'missing', message: 'Quota evidence is missing; allowance is unconfirmed.'}
      }))}
    }
    host.harnesses.launch = async () => ({copied: false, command: 'approved model', advisories: [
      'Quota evidence is missing; allowance is unconfirmed.',
      'Post-launch audit was not recorded; the process started.'
    ]})
    await host.settings.set({...await host.settings.get(), close_popover_after_launch: true})
  })
}
