// Use the pinned installed renderer without editing either protected source tree.
import { resolve, dirname } from 'node:path';
import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { patchPinnedStatusResize } from './patch-pinned-status-resize.mjs';
export function verifyPiclawStatus(root = resolve('.')) {
  const base = resolve(root, 'web/piclaw-status-3.2.4');
  const manifest = JSON.parse(readFileSync(resolve(base, 'manifest.json'), 'utf8'));
  for (const [name, hash] of Object.entries(manifest.files)) {
    if (createHash('sha256').update(readFileSync(resolve(base, name))).digest('hex') !== hash) throw Error(`Pinned Piclaw status source changed: ${name}`);
  }
  return base;
}
export function piclawStatusAdapter(root = resolve('.')) {
  const base = verifyPiclawStatus(root), src = resolve(root, 'web/src');
  return {name:'piclaw-3.2.4-status', setup(build) {
    build.onLoad({filter:/piclaw-status-3\.2\.4\/components\/status\.ts$/}, args=>({contents:patchPinnedStatusResize(readFileSync(args.path,'utf8')),loader:'ts'}));
    build.onResolve({filter: /(?:^|\/)status\.js$/}, args => {
      if (resolve(dirname(args.importer), args.path) === resolve(src, 'components/status.js')) return {path:resolve(base,'components/status.ts')};
    });
    build.onResolve({filter: /^\.\.?\//}, args => {
      if (!args.importer.startsWith(base + '/')) return;
      const target = resolve(dirname(args.importer), args.path);
      // Backend API, Markdown pipeline and the shared Preact instance stay native.
      for (const name of ['api.js','markdown.js','gi-preview-overflow.js','vendor/preact-htm.js']) {
        if (target === resolve(base,name)) return {path:resolve(src,name.startsWith('vendor/') ? name : name.replace(/\.js$/,'.ts'))};
      }
    });
  }};
}
