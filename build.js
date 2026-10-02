import { resolve, dirname } from 'path';
import { mkdirSync, readFileSync, writeFileSync, renameSync, existsSync, rmSync, copyFileSync, cpSync } from 'fs';
import { fileURLToPath } from 'url';
import { piclawStatusAdapter } from './scripts/piclaw-status-adapter.mjs';
import {piclawSvgAdapter, patchMarkdownSvg, patchPostSvg, verifyPiclawSvg} from './scripts/piclaw-svg-adapter.mjs';
import { patchTimelineMenu } from './scripts/patch-timeline-menu.mjs';
import { patchWorkspaceReadonly } from './scripts/patch-workspace-readonly.mjs';
import { patchTabReadonly } from './scripts/patch-tab-readonly.mjs';
import { patchQuickActionKeys, patchComposePopupKeys } from './scripts/patch-popup-keys.mjs';
import { patchModelPicker } from './scripts/patch-model-picker.mjs';
import { patchPickerGeometry } from './scripts/patch-picker-geometry.mjs';
import { patchComposeCommands } from './scripts/patch-compose-commands.mjs';
import { patchComposeEscape } from './scripts/patch-compose-escape.mjs';
import { patchComposeRandomId } from './scripts/patch-compose-random-id.mjs';
import { patchComposeCaptureToken } from './scripts/patch-compose-capture-token.mjs';
import { patchMessageReferenceLabels } from './scripts/patch-message-reference-labels.mjs';
import { patchComposeSurface } from './scripts/patch-compose-surface.mjs';
import { patchModelPanel } from './scripts/patch-model-panel.mjs';
import { patchModelThinking } from './scripts/patch-model-thinking.mjs';
import { patchSessionPanel } from './scripts/patch-session-panel.mjs';
import { patchAccentContrast } from './scripts/patch-accent-contrast.mjs';
import { patchThemeTextContrast } from './scripts/patch-theme-text-contrast.mjs';
import { patchVoiceInput } from './scripts/patch-voice-input.mjs';
import { patchModelAccessibility } from './scripts/patch-model-accessibility.mjs';
import { patchUploadCancel } from './scripts/patch-upload-cancel.mjs';
import { patchComposePrefillFocus } from './scripts/patch-compose-prefill-focus.mjs';
import { patchPostSpeech } from './scripts/patch-post-speech.mjs';
import { patchPostOutcomes } from './scripts/patch-post-outcomes.mjs';
import { patchPostRecoveryControl } from './scripts/patch-post-recovery-control.mjs';

import { readdirSync, statSync } from 'fs';
import { gzipSync, brotliCompressSync, constants as zlibConstants } from 'zlib';

const __dirname = dirname(fileURLToPath(import.meta.url));
// Source maps are opt-in (GI_SOURCEMAPS=1): they are ~10 MB and would be
// embedded in the binary. See issue #10.
const SOURCEMAPS = process.env.GI_SOURCEMAPS === '1';
process.chdir(__dirname);

const webSrc = 'web/src';
const distDir = 'internal/web/static/dist';
const vendorDir = 'internal/web/static/js/vendor';
const jsDir = 'internal/web/static/js';
const editorVendorDir = 'internal/web/static/editor-vendor';

mkdirSync(distDir, { recursive: true });
mkdirSync(vendorDir, { recursive: true });
mkdirSync(jsDir, { recursive: true });
mkdirSync(editorVendorDir, { recursive: true });

function run(cmd) {
  const proc = Bun.spawnSync(cmd, { stdout: 'inherit', stderr: 'inherit' });
  if (proc.exitCode !== 0) process.exit(proc.exitCode ?? 1);
}

function move(src, dst) {
  if (existsSync(dst)) rmSync(dst);
  if (existsSync(src)) renameSync(src, dst);
}

// Build a vendor entry. Leaves a copy in web/src/vendor/<outputName> for
// app.ts import resolution, then moves the canonical output to static/.
function buildVendor(entryFile, finalDir, finalName) {
  const entry = `${webSrc}/vendor/${entryFile}`;
  const base = entryFile.replace(/\.ts$/, '');
  const srcJs = `${webSrc}/vendor/${base}.js`;
  run(['bun', 'build', entry, '--target=browser', '--format=esm', '--minify', `--outfile=${srcJs}`, ...(SOURCEMAPS ? ['--sourcemap=linked'] : [])]);
  const srcMap = `${webSrc}/vendor/${base}.js.map`;
  // Leave a copy at the exact path components expect (e.g. preact-htm.js)
  const vendorAlias = `${webSrc}/vendor/${finalName}`;
  if (existsSync(srcJs) && vendorAlias !== srcJs) {
    copyFileSync(srcJs, vendorAlias);
  }
  // Move canonical build output to static/
  move(srcJs, `${finalDir}/${finalName}`);
  if (SOURCEMAPS) move(srcMap, `${finalDir}/${finalName}.map`);
  else for (const stale of [srcMap, `${finalDir}/${finalName}.map`]) if (existsSync(stale)) rmSync(stale);
}

// ── Vendor bundles ────────────────────────────────────────────────────────
buildVendor('preact-htm-entry.ts', vendorDir, 'preact-htm.js');
buildVendor('marked-entry.ts',     jsDir,      'marked.min.js');
buildVendor('katex-entry.ts',      vendorDir,  'katex.min.js');
// Keep the renderer CSS/fonts on the same version as its JavaScript bundle.
mkdirSync('internal/web/static/fonts/katex', { recursive: true });
// Only the woff2 faces ship: every supported browser loads woff2, and the
// woff/ttf fallbacks would add ~0.9 MB to the binary.
rmSync('internal/web/static/fonts/katex', { recursive: true, force: true });
mkdirSync('internal/web/static/fonts/katex', { recursive: true });
for (const font of readdirSync('node_modules/katex/dist/fonts')) {
  if (font.endsWith('.woff2')) copyFileSync(`node_modules/katex/dist/fonts/${font}`, `internal/web/static/fonts/katex/${font}`);
}
copyFileSync('node_modules/katex/LICENSE', 'internal/web/static/fonts/katex/LICENSE');
const katexCSS = readFileSync('node_modules/katex/dist/katex.min.css', 'utf8')
  .replace(/,url\(fonts\/[^)]+\.woff\) format\("woff"\)/g, '')
  .replace(/,url\(fonts\/[^)]+\.ttf\) format\("truetype"\)/g, '');
writeFileSync('internal/web/static/css/katex.min.css', katexCSS.replaceAll('url(fonts/', 'url(/fonts/katex/'));
buildVendor('mermaid-entry.ts',    vendorDir,  'beautiful-mermaid.js');
buildVendor('codemirror-entry.ts', editorVendorDir, 'codemirror.js');

// ── App bundle ────────────────────────────────────────────────────────────
const appBuild = await Bun.build({
  entrypoints: [`${webSrc}/gi-bootstrap.ts`], outdir: distDir,
  target: 'browser', format: 'esm', sourcemap: SOURCEMAPS ? 'linked' : 'none', splitting: true, modulePreload: false,
  naming: { entry: 'app.bundle.[ext]', chunk: 'chunks/[name]-[hash].[ext]', asset: 'assets/[name]-[hash].[ext]' },
  external: ['/editor-vendor/codemirror.js'],
  plugins: [piclawStatusAdapter(__dirname), piclawSvgAdapter(__dirname), { name: 'gi-post-speech', setup(build) {
    build.onLoad({ filter: /[\\/]components[\\/]post\.ts$/ }, async args => ({
      contents: patchPostSvg(patchPostRecoveryControl(patchPostOutcomes(patchPostSpeech(await Bun.file(args.path).text())))), loader: 'ts',
    }));
    build.onLoad({filter:/[\\/]src[\\/]markdown\.ts$/},async args=>({contents:patchMarkdownSvg(await Bun.file(args.path).text()),loader:'ts'}));
  } }, { name: 'gi-popup-key-ownership', setup(build) {
    build.onLoad({ filter: /[\\/]components[\\/]timeline-quick-actions\.ts$/ }, async args => ({
      contents: patchQuickActionKeys(await Bun.file(args.path).text()), loader: 'ts',
    }));
    build.onLoad({ filter: /[\\/]components[\\/]compose-box\.ts$/ }, async args => ({
      contents: patchMessageReferenceLabels(patchComposeCaptureToken(patchComposeRandomId(patchModelAccessibility(patchModelThinking(patchVoiceInput(patchSessionPanel(patchModelPanel(patchComposeSurface(patchComposeCommands(patchPickerGeometry(patchComposePrefillFocus(patchUploadCancel(patchModelPicker(patchComposePopupKeys(patchComposeEscape(await Bun.file(args.path).text())))))))))))))))), loader: 'ts',
    }));
  } }, { name: 'gi-workspace-readonly', setup(build) {
    build.onLoad({ filter: /[\\/]components[\\/]tab-strip\.ts$/ }, async args => ({
      contents: patchTabReadonly(await Bun.file(args.path).text()), loader: 'ts',
    }));
    build.onLoad({ filter: /[\\/]components[\\/]workspace-explorer\.ts$/ }, async args => ({
      contents: patchWorkspaceReadonly(await Bun.file(args.path).text()), loader: 'ts',
    }));
  } }, { name: 'gi-timeline-menu-dismissal', setup(build) {
    build.onLoad({ filter: /[\\/]components[\\/]timeline-menu\.ts$/ }, async args => ({
      contents: patchTimelineMenu(await Bun.file(args.path).text()), loader: 'ts',
    }));
  } }, { name: 'gi-appearance-renderer', setup(build) {
    // Adapt accent contrast at build time; keep the supplied source bytes unchanged.
    build.onLoad({ filter: /[\\/]ui[\\/]theme\.ts$/ }, async args => ({
      contents: patchThemeTextContrast(patchAccentContrast(await Bun.file(args.path).text())) + '\nexport { THEME_PRESETS as giThemePresets, applyThemeState as giApplyThemeState };\n',
      loader: 'ts',
    }));
  } }, { name: 'gi-clipboard-safety', setup(build) {
    build.onResolve({filter: /^\.\/post-runtime-safety\.js$/}, args => {
      if (args.importer === resolve(webSrc, 'components/post.ts')) {
        return { path: resolve(webSrc, 'gi-clipboard-safety.ts') };
      }
    });
  } }],
});
if (!appBuild.success) { console.error(appBuild.logs); process.exit(1); }
// Retain only this build's hashed chunks. Old open clients receive a pane-load
// error after deployment rather than executing a mismatched module.
const outputPaths = new Set(appBuild.outputs.map(output => resolve(output.path)));
for (const entry of new Bun.Glob('chunks/**/*').scanSync({ cwd: distDir, onlyFiles: true })) {
  const path = resolve(distDir, entry);
  if (!outputPaths.has(path)) rmSync(path);
}

// No post-processing needed — vendor scripts are loaded as modules.

// Now clean up preact-htm alias
const phtmAlias = `${webSrc}/vendor/preact-htm.js`;
if (existsSync(phtmAlias)) rmSync(phtmAlias);

// Clean up vendor alias copies EXCEPT preact-htm.js (kept for app externals resolution)
['marked.min.js', 'katex.min.js', 'beautiful-mermaid.js', 'codemirror.js'].forEach((f) => {
  const p = `${webSrc}/vendor/${f}`;
  if (existsSync(p)) rmSync(p);
});

// Pinned renderer stylesheet; original supplied stylesheet is left untouched.
copyFileSync('web/piclaw-status-3.2.4/css/agent.css', 'internal/web/static/css/piclaw-status-3.2.4.css');
verifyPiclawSvg(__dirname);
copyFileSync('web/piclaw-svg-3.2.4/css/svg-fences.css','internal/web/static/css/piclaw-svg-3.2.4.css');

// ── CSS bundle ────────────────────────────────────────────────────────────
// CSS bundle — all Piclaw CSS is served from /css/styles.css (with @import partials).
// app.bundle.css is kept minimal — only Gi-specific overrides go here.
writeFileSync(`${distDir}/app.bundle.css`, '/* Gi app overrides */\n', 'utf-8');

// ── Static asset post-processing (issue #10) ──────────────────────────────
function walk(dir, out = []) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = `${dir}/${entry.name}`;
    if (entry.isDirectory()) walk(path, out); else out.push(path);
  }
  return out;
}
const staticRoot = 'internal/web/static';
if (!SOURCEMAPS) for (const f of walk(staticRoot)) if (f.endsWith('.map')) rmSync(f);

// Pre-compress text assets: deterministic .br/.gz siblings, kept only when
// they save at least 10%. The server serves them with Content-Encoding.
// index.html is templated per request and is never pre-compressed.
const compressible = /\.(js|mjs|css|svg|json|txt|webmanifest|map|xml|html)$/;
let variants = 0, rawBytes = 0, brBytes = 0;
for (const f of walk(staticRoot)) {
  if (f.endsWith('.gz') || f.endsWith('.br')) {
    const source = f.slice(0, -3);
    if (!existsSync(source)) rmSync(f); // stale variant
    continue;
  }
  const eligible = compressible.test(f) && f !== `${staticRoot}/index.html` && statSync(f).size >= 1024;
  const data = eligible ? readFileSync(f) : null;
  for (const [suffix, encode] of [['.br', d => brotliCompressSync(d, { params: { [zlibConstants.BROTLI_PARAM_QUALITY]: 11, [zlibConstants.BROTLI_PARAM_SIZE_HINT]: d.length } })], ['.gz', d => gzipSync(d, { level: 9 })]]) {
    const target = f + suffix;
    const encoded = data ? encode(data) : null;
    if (encoded && encoded.length <= data.length * 0.9) {
      if (!existsSync(target) || !readFileSync(target).equals(encoded)) writeFileSync(target, encoded);
      variants++;
      if (suffix === '.br') { rawBytes += data.length; brBytes += encoded.length; }
    } else if (existsSync(target)) rmSync(target);
  }
}
console.log(`Pre-compressed ${variants} variants (${(rawBytes / 1e6).toFixed(1)} MB → ${(brBytes / 1e6).toFixed(1)} MB brotli).`);

console.log('Gi web build complete.');
