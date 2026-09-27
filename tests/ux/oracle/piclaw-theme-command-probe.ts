// Pinned installed Piclaw 3.2.4 command parser only. No database, HTTP, chat or browser writes.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {readFile} from 'node:fs/promises';
import path from 'node:path';
import {pathToFileURL} from 'node:url';

const oracleRoot=path.resolve(process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current');
assert.equal((await readFile(path.join(oracleRoot,'VERSION'),'utf8')).trim(),'3.2.4');
const reference=JSON.parse(await readFile(new URL('./piclaw-3.2.4-reference.json',import.meta.url),'utf8'));
const map=await readFile(path.join(oracleRoot,'app/runtime/web/static/classic/dist/app.bundle.js.map'));
assert.equal(createHash('sha256').update(map).digest('hex'),reference.map.sha256);
const moduleUrl=pathToFileURL(path.join(oracleRoot,'app/runtime/src/channels/web/theming/ui-theme-commands.ts')).href;
const {handleUiThemeCommand}=await import(moduleUrl);
const cases=[
 ['/theme','success',null,/Available themes/],
 ['/theme ristretto','success',{theme:'ristretto',tint:null},/Theme set to/],
 ['/theme default','success',{theme:'default',tint:null},/Theme set to/],
 ['/theme dark','error',null,/Unknown theme/],
 ['/tint','error',null,/Usage/],
 ['/tint #e11d48','success',{theme:'default',tint:'#e11d48'},/Tint set to/],
 ['/tint orange','success',{theme:'default',tint:'orange'},/Tint set to/],
 ['/tint off','success',{theme:'default',tint:null},/Tint cleared/],
 ['/tint $$notacolor','error',null,/Invalid tint/],
] as const;
const results=[];
for(const [input,status,payload,message] of cases){
 const result=handleUiThemeCommand(input);
 assert.equal(result?.status,status,input);
 assert.deepEqual(result?.payload??null,payload,input);
 assert.match(result?.message??'',message,input);
 results.push({input,status,payload:result?.payload??null});
}
assert.equal(handleUiThemeCommand('hello'),null);
console.log(JSON.stringify({version:'3.2.4',mapSha256:reference.map.sha256,
 scope:'installed command parser function only; no browser, timeline, database, HTTP or live chat',results},null,2));
