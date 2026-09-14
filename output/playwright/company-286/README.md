# Company advisory UI evidence

Captured 2026-09-11 (Europe/London), Chromium, browser-mode mock host.
The screenshots exercise the real PopoverApp with synthetic missing-quota and
post-start audit-failure responses; no credentials or native processes are used.

- `advisory-dark.png`: Quick tab, 500 × 620, process-started notice visible.
- `advisory-advanced.png`: Advanced tab, 500 × 620, same notice.
- `advisory-400-light-preference.png`: Advanced, 400 × 620 with browser light
  preference. The app ships its Nocturne theme and does not switch palettes.

Manual inspection found overlapping recommendation and notices in the first
capture. The final company content region scrolls within the existing height
ceiling; recommendation, notices, dismiss control and footer no longer overlap.
The launch notice scrolls into view and stays until dismissed; automatic close is
suppressed. At 400px the footer label wraps within its button. The content above
the notice remains reachable by scrolling. The dark screenshots retain the normal
short command toast; the narrower capture was made after it expired.

Reproduce after `pnpm build` (avoid rebuilding workspace packages during capture):

1. `pnpm --filter desktop dev:browser --host 127.0.0.1 --port 5186`
2. Open `http://127.0.0.1:5186/` with Playwright CLI.
3. Run the function in `fixture.js` using the CLI's `run-code` command. It changes
   only the page's in-memory mock host and sets the 500 × 620 viewport.
4. Snapshot, click **Launch in Claude Code**, capture Quick, then click Advanced.
5. Set viewport 400 × 620 and emulate `colorScheme: 'light'`; capture again.

Observed accessible text: `Score-only recommendation`, `Quota evidence is missing;
allowance is unconfirmed.`, status `Process started`, `Post-launch audit was not
recorded; the process started.`, button `Dismiss launch notice`.

Unit tests additionally verify personal close behavior, company non-close behavior,
dismissal and refreshed company reports after usage events. The Wails host surface
check and desktop TypeScript build pass.
