// Installed Piclaw 3.2.4 manifest and shell-icon helpers with disposable collaborators.
// No live web channel, actual avatar cache, home-screen install or OS icon selection.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
assert.equal((await fs.readFile(path.join(root,'VERSION'),'utf8')).trim(),'3.2.4');
const {handleManifestRequest}=await import(path.join(root,'app/runtime/src/channels/web/manifest.ts'));
const {handleShellRoutes}=await import(path.join(root,'app/runtime/src/channels/web/http/dispatch-shell.ts'));
const {getRouteFlags}=await import(path.join(root,'app/runtime/src/channels/web/http/route-flags.ts'));
const origin='http://127.0.0.1:19242',avatar='fixture-avatar',manifest=async(cache,method='GET',source=avatar)=>{
 const reads=[];const req=new Request(origin+'/manifest.json',{method});
 const response=await handleManifestRequest(req,{assistantName:'Fixture Agent',assistantAvatar:source,ensureAvatarCache:async(kind,value)=>{reads.push([kind,value]);if(cache instanceof Error)throw cache;return cache;}});
 return {response,body:method==='HEAD'?null:await response.json(),reads};
};
const fallback=await manifest(null);assert.equal(fallback.response.status,200);assert.deepEqual(fallback.reads,[['agent',avatar]]);
assert.equal(fallback.body.piclaw_avatar,null);assert.equal(fallback.body.icons.length,4);
assert.deepEqual(fallback.body.icons.map(icon=>icon.src),['/static/icon-192.png','/static/icon-512.png','/static/icon-192.png','/static/icon-512.png']);
const absent=await manifest(null,'GET',null);assert.deepEqual(absent.reads,[]);assert.deepEqual(absent.body.icons,fallback.body.icons);
const rejected=await manifest(new Error('Disposable cache failure'));assert.deepEqual(rejected.body.icons,fallback.body.icons);
const avatarA=await manifest({revision:'version A'}),avatarB=await manifest({revision:'version B'});
assert.equal(avatarA.response.headers.get('cache-control'),'no-store');assert.equal(avatarA.body.icons.length,4);
for(const [size,purpose] of [[192,'any'],[192,'maskable'],[512,'any'],[512,'maskable']]){
 const icon=avatarA.body.icons.find(value=>value.sizes===`${size}x${size}`&&value.purpose===purpose);
 assert.ok(icon);const url=new URL(icon.src,origin);assert.equal(url.pathname,'/avatar/agent');
 assert.equal(url.searchParams.get('format'),'png');assert.equal(url.searchParams.get('size'),String(size));
 assert.equal(url.searchParams.get('v'),'version A');assert.equal(url.searchParams.get('purpose'),purpose==='maskable'?'maskable':null);
 assert.equal(avatarB.body.icons.find(value=>value.sizes===icon.sizes&&value.purpose===purpose).src.replace('version%20B','version%20A'),icon.src);
}
assert.equal(avatarA.body.piclaw_avatar,avatarA.body.icons[0].src);
assert.notDeepEqual(avatarA.body.icons.map(x=>x.src),avatarB.body.icons.map(x=>x.src));
const head=await manifest({revision:'version A'},'HEAD');assert.equal(head.body,null);
assert.equal(head.response.headers.get('content-length'),avatarA.response.headers.get('content-length'));
const shellCases=[];
for(const [pathname,size] of [['/favicon.ico','48'],['/apple-touch-icon-180x180.png','180'],['/apple-touch-icon-167x167.png','167'],['/apple-touch-icon-152x152.png','152'],['/apple-touch-icon.png','180']]){
 for(const mode of ['png','non-png','error']){
  const calls=[],req=new Request(origin+pathname),flags=getRouteFlags(req,pathname);
  const avatarResponse=mode==='png'?new Response('avatar-bytes',{status:200,headers:{'Content-Type':'image/png'}})
   :mode==='non-png'?new Response('wrong-type',{status:200,headers:{'Content-Type':'image/webp'}})
   :new Response('unavailable',{status:404,headers:{'Content-Type':'image/png'}});
  const channel={handleAvatar:async(kind,r)=>{calls.push({kind,url:r.url});return avatarResponse;}};
  const staticAsset=async(_r,relative)=>{calls.push({static:relative});return new Response('static-bytes',{status:200,headers:{'Content-Type':pathname.endsWith('.ico')?'image/x-icon':'image/png'}})};
  const response=await handleShellRoutes(channel,req,pathname,flags,staticAsset);
  assert.equal(response.status,200);assert.equal(await response.text(),mode==='png'?'avatar-bytes':'static-bytes');
  assert.equal(calls[0].kind,'agent');const u=new URL(calls[0].url);
  assert.equal(u.pathname,pathname);assert.equal(u.searchParams.get('format'),'png');assert.equal(u.searchParams.get('size'),size);
  assert.equal(calls.length,mode==='png'?1:2);if(mode!=='png')assert.equal(calls[1].static,pathname==='/favicon.ico'?'favicon.ico':pathname.slice(1));
  shellCases.push({pathname,size,mode,selected:mode==='png'?'avatar':'static'});
 }
}
console.log(JSON.stringify({scope:'Installed Piclaw 3.2.4 server helpers with disposable avatar/cache/static callbacks; no live avatar storage, authenticated routing, generated image bytes, home-screen install or physical OS',manifest:{fallback:4,avatar:4,versions:['version A','version B'],head:true},shellCases},null,2));
