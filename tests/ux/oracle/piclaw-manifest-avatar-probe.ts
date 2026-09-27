// Installed Piclaw 3.2.4 manifest handler with a disposable avatar-cache dependency.
// No production avatar file, HTTP listener, live chat or home-screen installation.
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
const url=pathToFileURL(path.join(oracleRoot,'app/runtime/src/channels/web/manifest.ts')).href;
const {handleManifestRequest}=await import(url);
const req=(method:string)=>new Request('http://fixture/manifest.json',{method});
const cacheCalls:any[]=[];
const ctx=(avatar:string|null,meta:any)=>({assistantName:'Oracle Agent',assistantAvatar:avatar,
 ensureAvatarCache:async(kind:string,source:string)=>{cacheCalls.push({kind,source});return meta;}});
const read=async(response:Response)=>({status:response.status,headers:Object.fromEntries(response.headers),body:await response.text()});
const staticResponse=await read(await handleManifestRequest(req('GET'),ctx(null,null)));
assert.equal(staticResponse.status,200);
const staticManifest=JSON.parse(staticResponse.body);
assert.deepEqual(staticManifest.icons.map((icon:any)=>icon.src),[
 '/static/icon-192.png','/static/icon-512.png','/static/icon-192.png','/static/icon-512.png']);
assert.equal(staticManifest.piclaw_avatar,null);
assert.deepEqual(cacheCalls,[]);
const prepared={revision:'rev-7',updatedAt:'2026-09-27T12:00:00.000Z'};
const avatarResponse=await read(await handleManifestRequest(req('GET'),ctx('/tmp/disposable-avatar.png',prepared)));
assert.equal(avatarResponse.status,200);
const avatarManifest=JSON.parse(avatarResponse.body);
assert.deepEqual(cacheCalls,[{kind:'agent',source:'/tmp/disposable-avatar.png'}]);
assert.equal(avatarManifest.icons.length,4);
for(const icon of avatarManifest.icons){
 const parsed=new URL(icon.src,'http://fixture');
 assert.equal(parsed.pathname,'/avatar/agent');
 assert.equal(parsed.searchParams.get('format'),'png');
 assert.equal(parsed.searchParams.get('v'),'rev-7');
 assert.ok(['192','512'].includes(parsed.searchParams.get('size')));
}
assert.equal(avatarManifest.piclaw_avatar,avatarManifest.icons[0].src);
const head=await read(await handleManifestRequest(req('HEAD'),ctx('/tmp/disposable-avatar.png',prepared)));
assert.equal(head.status,200);assert.equal(head.body,'');
assert.equal(head.headers['cache-control'],'no-store');
const fallback=await read(await handleManifestRequest(req('GET'),ctx('/tmp/disposable-avatar.png',null)));
assert.deepEqual(JSON.parse(fallback.body).icons.map((icon:any)=>icon.src),staticManifest.icons.map((icon:any)=>icon.src));
console.log(JSON.stringify({release:reference.release,mapSha256:reference.map.sha256,
 scope:'installed manifest handler with injected avatar metadata; no avatar file, HTTP listener or live writes',
 staticIcons:staticManifest.icons.map((icon:any)=>icon.src),avatarIcons:avatarManifest.icons.map((icon:any)=>icon.src),
 fallbackIcons:JSON.parse(fallback.body).icons.map((icon:any)=>icon.src),head:{status:head.status,bodyBytes:head.body.length}},null,2));
