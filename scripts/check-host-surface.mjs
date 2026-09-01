#!/usr/bin/env node
// S04 CONTRACTS §5 / SPEC §2.7 — `check:host`. Statically compares the
// EngineHost interface in packages/core/src/host.ts against the exported
// binding functions of the generated bindings modules. Exits 1 on any missing
// or extra method/group so the webview surface never drifts from the bindings.
import { readFileSync } from 'node:fs'
import { resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const hostPath = resolve(root, 'packages/core/src/host.ts')
const bindingsDir = resolve(
  root,
  'apps/desktop/src/bindings/github.com/WD-Mitchell/which-model/cmd/which-model-desktop',
)

const hostSrc = readFileSync(hostPath, 'utf8')

// EngineHost group -> generated binding module. The keys are the module
// export names (wails generates lowercase FILENAMES — pickapi.js etc. — but
// PascalCase export symbols), so the filename is the lowercased module name
// + ".js" (issue #33: reading the PascalCase name ENOENTs on
// case-sensitive filesystems, i.e. Linux CI).
const GROUP_MODULE = {
  profiles: 'ProfilesAPI',
  pick: 'PickAPI',
  catalog: 'CatalogAPI',
  providers: 'ProvidersAPI',
  harnesses: 'HarnessesAPI',
  usage: 'UsageAPI',
  favourites: 'FavouritesAPI',
  settings: 'SettingsAPI',
  window: 'WindowService',
}

function exportedFunctions(moduleName) {
  // wails3 generate bindings emits all-lowercase filenames (index.js
  // imports "./pickapi.js" etc.); only the export symbols are PascalCase.
  const src = readFileSync(`${bindingsDir}/${moduleName.toLowerCase()}.js`, 'utf8')
  return [...src.matchAll(/^export function ([A-Za-z_$][\w$]*)/gm)].map((m) => m[1])
}

// Parse EngineHost interface methods: group property blocks with 4-space methods.
const errors = []
const groupRe = /^\s{2}([a-z_]+): \{/gm
const methodRe = /^\s{4}([a-zA-Z_$][\w$]*)(?=\()/gm
let m

while ((m = groupRe.exec(hostSrc))) {
  const group = m[1]
  if (group === 'on') continue // runtime, not a binding
  const start = m.index + m[0].length
  const end = hostSrc.indexOf('\n  }', start)
  const block = hostSrc.slice(start, end === -1 ? undefined : end)
  const methods = new Set()
  let mm
  while ((mm = methodRe.exec(block))) methods.add(mm[1])

  const moduleName = GROUP_MODULE[group]
  if (!moduleName) {
    errors.push(`host group "${group}" is not bound (no module)`)
    continue
  }
  const exported = new Set(exportedFunctions(moduleName).map((n) => n.toLowerCase()))
  for (const method of methods)
    if (!exported.has(method.toLowerCase())) errors.push(`bindings: missing ${group}.${method}`)
  for (const fn of exported)
    if (![...methods].map((x) => x.toLowerCase()).includes(fn)) errors.push(`bindings: extra ${group}.${fn}`)
}

if (errors.length) {
  console.error('check:host — EngineHost ↔ bindings surface mismatch:')
  for (const e of errors) console.error('  ✗ ' + e)
  process.exit(1)
}
console.log('check:host — EngineHost surface matches bindings')