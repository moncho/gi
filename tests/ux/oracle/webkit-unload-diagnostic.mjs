// Minimal navigation control: no Piclaw/Gi assets, routing, credentials or backend.
import {createServer} from 'node:http';
import {chromium,webkit} from 'playwright';
import fs from 'node:fs/promises';
const evidence=[];
for(const [name,type]of Object.entries({chromium,webkit})){
 const streams=new Set(),requests=[];
 const server=createServer((req,res)=>{
  requests.push({method:req.method,url:req.url,at:Date.now()});req.resume();
  if(req.url==='/events'){res.writeHead(200,{'Content-Type':'text/event-stream'});res.write('event: connected\ndata: {}\n\n');streams.add(res);res.on('close',()=>streams.delete(res));return;}
  if(req.method==='POST'&&req.url==='/presence'){res.writeHead(200,{'Content-Type':'application/json'});res.end('{"ok":true}');return;}
  res.writeHead(200,{'Content-Type':'text/html'});res.end(`<textarea></textarea><script>
const events=new EventSource('/events');let connected=false;events.addEventListener('connected',()=>{connected=true;document.body.dataset.ready='yes'});
const presence=()=>fetch('/presence',{method:'POST',headers:{'Content-Type':'application/json'},body:'{}'}).catch(()=>{});
presence();document.addEventListener('visibilitychange',presence);window.addEventListener('pageshow',presence);
window.addEventListener('pagehide',()=>navigator.sendBeacon('/presence',new Blob(['{}'],{type:'application/json'})));
window.addEventListener('beforeunload',()=>navigator.sendBeacon('/presence',new Blob(['{}'],{type:'application/json'})));
</script>`);
 });await new Promise(r=>server.listen(0,'127.0.0.1',r));
 const browser=await type.launch({headless:true}),page=await browser.newPage({serviceWorkers:'block'});const errors=[],failures=[];
 page.on('pageerror',e=>errors.push(e.message));page.on('requestfailed',r=>failures.push({url:r.url(),error:r.failure()?.errorText}));
 try{await page.goto(`http://127.0.0.1:${server.address().port}`);await page.waitForSelector('body[data-ready=yes]');await page.reload();await page.waitForSelector('body[data-ready=yes]');await page.waitForTimeout(200);evidence.push({browser:name,errors,failures,requests,replacementConnected:true});}
 finally{await browser.close();for(const s of streams)s.end();server.closeAllConnections();await new Promise(r=>server.close(r));}
}
const output=process.env.ORACLE_OUTPUT||'test-results/ux-oracle/webkit-unload-control.json';await fs.mkdir(output.slice(0,output.lastIndexOf('/')),{recursive:true});await fs.writeFile(output,JSON.stringify(evidence,null,2));console.log(JSON.stringify({output,evidence},null,2));
