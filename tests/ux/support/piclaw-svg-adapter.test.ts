import {test,expect} from 'bun:test';
import {readFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {resolve} from 'node:path';
import {piclawSvgAdapter,patchMarkdownSvg,patchPostSvg,verifyPiclawSvg} from '../../../scripts/piclaw-svg-adapter.mjs';

const sha=(bytes:Buffer|string)=>createHash('sha256').update(bytes).digest('hex');

test('pinned SVG files match their provenance manifest without modifying supplied files',()=>{
 const base=verifyPiclawSvg();expect(base).toBe(resolve('web/piclaw-svg-3.2.4'));
 const manifest=JSON.parse(readFileSync(resolve(base,'MANIFEST.json'),'utf8'));
 for(const file of manifest.files){
  const pinned=readFileSync(file.target);expect(pinned.byteLength).toBe(file.bytes);
  expect(sha(pinned)).toBe(file.sha256);
 }
});

// Historical source comparison needs the historical release, never whichever
// Piclaw happens to be installed as current. Asset integrity always runs above.
const historicalRoot=process.env.PICLAW_324_STATIC_ROOT;
(historicalRoot?test:test.skip)('pinned SVG files match the explicit Piclaw 3.2.4 source map',()=>{
 const manifest=JSON.parse(readFileSync('web/piclaw-svg-3.2.4/MANIFEST.json','utf8'));
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
 expect(adapted).toContain("from '../piclaw-svg-3.2.4/utils/svg-images.js'");
 expect(adapted).toContain('return renderSvgFences(text, part => renderMarkdownBody(part, onHashtagClick, options)');
 expect(adapted).toContain('source => `<pre><code class="language-svg" data-svg-source=');
 expect(adapted).toContain('export function renderThinkingMarkdown(text)');
 expect(source).not.toContain('renderSvgFences');
 expect(()=>patchMarkdownSvg(adapted)).toThrow('anchor changed');
});

test('post adapter scopes preview surface controls and cleans up both listeners',()=>{
 const source=readFileSync('web/src/components/post.ts','utf8');
 const adapted=patchPostSvg(source);
 expect(adapted).toContain("from '../../piclaw-svg-3.2.4/utils/svg-images.js'");
 expect(adapted).toContain('const unbindSvg = bindSvgImageThemes(contentRef.current)');
 expect(adapted).toContain('return () => { unbindSvg(); unbindCopy(); };');
 expect(source).not.toContain('bindSvgImageThemes');
 expect(()=>patchPostSvg(adapted)).toThrow('anchor changed');
 const handlers:any[]=[];piclawSvgAdapter().setup({onResolve:(_filter:any,handler:any)=>handlers.push(handler)});
 expect(handlers[0]({importer:resolve('web/piclaw-svg-3.2.4/utils/svg-images.ts'),path:'../ui/svg-theme.js'}).path).toBe(resolve('web/piclaw-svg-3.2.4/ui/svg-theme.ts'));
 expect(handlers[0]({importer:resolve('web/src/markdown.ts'),path:'../ui/svg-theme.js'})).toBeUndefined();
});
