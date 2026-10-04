import {test,expect} from 'bun:test';
import {existsSync,readFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {resolve} from 'node:path';
import {piclawSvgAdapter,patchMarkdownSvg,patchPostSvg,verifyPiclawSvg} from '../../../scripts/piclaw-svg-adapter.mjs';

const sha=(bytes:Buffer|string)=>createHash('sha256').update(bytes).digest('hex');

test('pinned SVG files match their provenance manifest without modifying supplied files',()=>{
 const base=verifyPiclawSvg();expect(base).toBe(resolve('web/piclaw-svg-3.2.5'));
 const manifest=JSON.parse(readFileSync(resolve(base,'MANIFEST.json'),'utf8'));
 for(const file of manifest.files){
  const pinned=readFileSync(file.target);expect(pinned.byteLength).toBe(file.bytes);
  expect(sha(pinned)).toBe(file.sha256);
 }
});

// Source comparison needs the pinned 3.2.5 release; the manifest's source-map
// hash rejects any other. Asset integrity always runs above.
const defaultRoot='/opt/piclaw/releases/piclaw-3.2.5-linux-x64-baseline/app/runtime/web/static/classic';
const historicalRoot=process.env.PICLAW_325_STATIC_ROOT||(existsSync(defaultRoot)?defaultRoot:undefined);
(historicalRoot?test:test.skip)('pinned SVG files match the explicit Piclaw 3.2.5 source map',()=>{
 const manifest=JSON.parse(readFileSync('web/piclaw-svg-3.2.5/MANIFEST.json','utf8'));
 const map=readFileSync(resolve(historicalRoot!,'dist/app.bundle.js.map'));
 expect(sha(map)).toBe(manifest.sourceMapSha256);
 const sourceMap=JSON.parse(map.toString());
 for(const file of manifest.files){
  const pinned=readFileSync(file.target);
  if(file.source.startsWith('../../../')){
   const index=sourceMap.sources.indexOf(file.source);expect(index).toBeGreaterThanOrEqual(0);
   expect(sourceMap.sourcesContent[index]).toBe(pinned.toString());
  }else{
   const css=readFileSync(resolve(historicalRoot!,'dist/app.bundle.css'));
   expect(css.subarray(...file.sourceRange).toString()).toBe(pinned.toString());
  }
 }
});

test('Markdown adapter wraps only the unchanged renderer with pinned SVG processing',()=>{
 const source=readFileSync('web/src/markdown.ts','utf8');
 const adapted=patchMarkdownSvg(source);
 expect(adapted).toContain("from '../piclaw-svg-3.2.5/utils/svg-images.js'");
 expect(adapted).toContain('return renderSvgFences(text, part => renderMarkdownBody(part, onHashtagClick, options)');
 expect(adapted).toContain('source => `<pre><code class="language-svg" data-svg-source=');
 expect(adapted).toContain('export function renderThinkingMarkdown(text)');
 expect(source).not.toContain('renderSvgFences');
 expect(()=>patchMarkdownSvg(adapted)).toThrow('anchor changed');
});

test('post adapter scopes preview surface controls and cleans up both listeners',()=>{
 const source=readFileSync('web/src/components/post.ts','utf8');
 const adapted=patchPostSvg(source);
 expect(adapted).toContain("from '../../piclaw-svg-3.2.5/utils/svg-images.js'");
 expect(adapted).toContain('const unbindSvg = bindSvgImageThemes(contentRef.current)');
 expect(adapted).toContain('return () => { unbindSvg(); unbindCopy(); };');
 expect(source).not.toContain('bindSvgImageThemes');
 expect(()=>patchPostSvg(adapted)).toThrow('anchor changed');
 const handlers:any[]=[];piclawSvgAdapter().setup({onResolve:(_filter:any,handler:any)=>handlers.push(handler)});
 expect(handlers[0]({importer:resolve('web/piclaw-svg-3.2.5/utils/svg-images.ts'),path:'../ui/svg-theme.js'}).path).toBe(resolve('web/piclaw-svg-3.2.5/ui/svg-theme.ts'));
 expect(handlers[0]({importer:resolve('web/src/markdown.ts'),path:'../ui/svg-theme.js'})).toBeUndefined();
});
