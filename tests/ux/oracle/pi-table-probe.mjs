// Installed Pi terminal Markdown renderer: independent table geometry oracle.
import fs from 'node:fs/promises';
import {pathToFileURL} from 'node:url';
import assert from 'node:assert/strict';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
const base=root+'/app/node_modules/@earendil-works/pi-tui';
const pkg=JSON.parse(await fs.readFile(base+'/package.json','utf8'));assert.equal(pkg.version,'0.87.1');
const {Markdown}=await import(pathToFileURL(base+'/dist/components/markdown.js'));
const {visibleWidth}=await import(pathToFileURL(base+'/dist/utils.js'));
const theme=Object.fromEntries(['heading','link','linkUrl','code','codeBlock','codeBlockBorder','quote','quoteBorder','hr','listBullet','bold','italic','strikethrough','underline'].map(k=>[k,s=>s]));
const markdown='| Command | Result |\n| --- | --- |\n| `go test ./internal/tui` | Tests pass with terminal columns preserved |\n| 界面 🙂 | é and wide glyphs stay aligned |\n';
const cases=[];
for(const width of [30,56,96,136]){
 const renderer=new Markdown(markdown,0,0,theme);const lines=renderer.render(width).filter(l=>l.trim());
 assert(lines[0].startsWith('┌'));assert(lines.every(l=>visibleWidth(l)<=width));assert(lines.every(l=>visibleWidth(l)===visibleWidth(lines[0])));
 cases.push({width,lines});
}
const stream=[];for(const end of [20,38,70,110,markdown.length]){const lines=new Markdown(markdown.slice(0,end),0,0,theme).render(56);assert(lines.every(l=>visibleWidth(l)<=56));stream.push({end,lines});}
const out=process.env.PI_TABLE_ORACLE_OUTPUT||'test-results/tui-tables/pi-oracle.json';await fs.mkdir(out.slice(0,out.lastIndexOf('/')),{recursive:true});await fs.writeFile(out,JSON.stringify({version:pkg.version,scope:'Installed Pi Markdown render and partial-text geometry, not physical terminal acceptance.',markdown,cases,stream},null,2));console.log(JSON.stringify({result:'pass',out,widths:cases.map(c=>c.width)}));
