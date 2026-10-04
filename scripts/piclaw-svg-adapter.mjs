// Pin the installed Piclaw SVG sanitizer and adapt only the build graph.
import {createHash} from 'node:crypto';
import {readFileSync} from 'node:fs';
import {dirname,resolve} from 'node:path';
const hash=data=>createHash('sha256').update(data).digest('hex');
const replace=(source,from,to)=>{
 if(source.split(from).length!==2)throw Error('Piclaw SVG adapter anchor changed: '+from.slice(0,90));
 return source.replace(from,to);
};
export function verifyPiclawSvg(root=resolve('.')){
 const base=resolve(root,'web/piclaw-svg-3.2.5');
 const manifest=JSON.parse(readFileSync(resolve(base,'MANIFEST.json'),'utf8'));
 if(manifest.release!=='piclaw-3.2.5-linux-x64-baseline'||manifest.sourceMapSha256!=='6fa35edad38ab67f75b9defd53699b02bb6fa0c330aa353327456bef25ede769')throw Error('Piclaw SVG provenance changed');
 for(const file of manifest.files){const target=resolve(root,file.target);if(!target.startsWith(base+'/')||hash(readFileSync(target))!==file.sha256)throw Error('Pinned Piclaw SVG source changed: '+file.target)}
 return base;
}
export function patchMarkdownSvg(source){
 if(source.includes('renderSvgFences')||source.includes('renderMarkdownBody'))throw Error('Piclaw SVG adapter anchor changed: markdown already adapted');
 source=replace(source,"export function renderMarkdown(text, onHashtagClick, options = {}) {","function renderMarkdownBody(text, onHashtagClick, options = {}) {");
 const anchor='/**\n * Render thinking panels with markdown while keeping tags/quotes intact.\n */';
 const wrapper=`export function renderMarkdown(text, onHashtagClick, options = {}) {
    if (!text) return '';
    return renderSvgFences(text, part => renderMarkdownBody(part, onHashtagClick, options),
        source => \`<pre><code class="language-svg" data-svg-source="\${encodeSvgSource(source)}">\${escapeSvgSource(source)}</code></pre>\`,
        { sanitize: typeof window === 'undefined' || window.__PICLAW_SANITIZE_SVG_FENCES__ !== false });
}

`;
 source=replace(source,anchor,wrapper+anchor);
 return "import {renderSvgFences, escapeSvgSource, encodeSvgSource} from '../piclaw-svg-3.2.5/utils/svg-images.js';\n"+source;
}
export function patchPostSvg(source){
 source=replace(source,`        renderMermaidDiagrams(contentRef.current);
        return enhanceCodeBlocks(contentRef.current);`,`        renderMermaidDiagrams(contentRef.current);
        const unbindSvg = bindSvgImageThemes(contentRef.current);
        const unbindCopy = enhanceCodeBlocks(contentRef.current);
        return () => { unbindSvg(); unbindCopy(); };`);
 return "import {bindSvgImageThemes} from '../../piclaw-svg-3.2.5/utils/svg-images.js';\n"+source;
}
export function piclawSvgAdapter(root=resolve('.')){
 const base=verifyPiclawSvg(root);
 return {name:'piclaw-3.2.5-svg',setup(build){
  build.onResolve({filter:/^\.\.\/ui\/svg-theme\.js$/},args=>{
   if(args.importer===resolve(base,'utils/svg-images.ts'))return{path:resolve(base,'ui/svg-theme.ts')};
  });
 }};
}
