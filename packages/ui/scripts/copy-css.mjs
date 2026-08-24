import { readdir, mkdir, copyFile } from 'node:fs/promises'
import { join, relative, dirname } from 'node:path'

// Copies every `.css` under `src/` into `dist/` preserving the relative path,
// so the emitted JS (which imports CSS modules and the theme stylesheets) can
// resolve them at the same relative locations.
const src = 'src'
const out = 'dist'

async function collect(dir) {
  const entries = await readdir(dir, { withFileTypes: true })
  const files = []
  for (const e of entries) {
    const p = join(dir, e.name)
    if (e.isDirectory()) files.push(...(await collect(p)))
    else if (e.name.endsWith('.css')) files.push(p)
  }
  return files
}

const cssFiles = await collect(src)
for (const file of cssFiles) {
  const rel = relative(src, file)
  const dest = join(out, rel)
  await mkdir(dirname(dest), { recursive: true })
  await copyFile(file, dest)
}
console.log(`copy-css: copied ${cssFiles.length} css file(s) to ${out}`)