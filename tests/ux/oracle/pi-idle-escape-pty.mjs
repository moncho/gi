/** Installed Pi 0.87.1 disposable tmux oracle: idle Escape keeps the editor focused. */
import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {mkdtempSync,mkdirSync,readFileSync,writeFileSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join,resolve} from 'node:path';
const root=process.env.PICLAW_ORACLE_ROOT||'/opt/piclaw/current';
const pkg=JSON.parse(readFileSync(join(root,'app/node_modules/@earendil-works/pi-coding-agent/package.json'),'utf8'));
assert.equal(pkg.version,'0.87.1');assert.equal(readFileSync(join(root,'VERSION'),'utf8').trim(),'3.2.4');
const bin=join(root,'app/node_modules/@earendil-works/pi-coding-agent/dist/bundle/cli.js');
const out=resolve(`test-results/ux-oracle/pi-idle-escape/run-${Date.now()}`);mkdirSync(out,{recursive:true});
const dir=mkdtempSync(join(tmpdir(),'pi-idle-escape-')),sock=`pi-idle-escape-${process.pid}`,pane='proof:0.0';mkdirSync(join(dir,'agent'));
const run=(cmd,args)=>{const r=spawnSync(cmd,args,{encoding:'utf8',timeout:12000});if(r.status!==0)throw Error(`${cmd}: ${r.stderr}`);return r.stdout};
const tm=(...args)=>run('tmux',['-L',sock,...args]);const cap=()=>tm('capture-pane','-p','-t',pane).replaceAll('\u00a0',' ');
const sleep=ms=>new Promise(r=>setTimeout(r,ms));const wait=async(fn,label)=>{const until=Date.now()+10000;while(Date.now()<until){if(fn())return;await sleep(80)}throw Error(`Timed out ${label}`)};
try{
 tm('new-session','-d','-s','proof','-x','100','-y','30',`cd '${dir}' && PI_CODING_AGENT_DIR='${join(dir,'agent')}' '${bin}' --no-session --tools '' 2>'${join(dir,'runtime.log')}'; sleep 20`);tm('set-option','-t','proof','status','off');
 await wait(()=>cap().includes('0.0%/0'),'startup');
 tm('send-keys','-t',pane,'Escape');await sleep(160);tm('send-keys','-t',pane,'-l','idle-after');await wait(()=>cap().includes('idle-after'),'empty-editor Escape retains input');
 tm('send-keys','-t',pane,'C-u');tm('send-keys','-t',pane,'-l','probe-');await wait(()=>cap().includes('probe-'),'draft');
 tm('send-keys','-t',pane,'Escape');await sleep(160);tm('send-keys','-t',pane,'-l','after');await wait(()=>cap().includes('probe-after'),'draft Escape retains input');
 const screen=cap();writeFileSync(join(out,'pi-idle-escape.txt'),screen);
 if(!screen.includes('probe-after'))throw Error('draft not retained');
 const result={pi:pkg.version,piclaw:'3.2.4',result:'pass',emptyEscapeInput:true,draftEscapeInput:true,scope:'Installed Pi disposable tmux, no session/model/provider; not a physical terminal or live inference'};
 writeFileSync(join(out,'result.json'),JSON.stringify(result,null,2));console.log(JSON.stringify({out,...result},null,2));
}catch(error){try{writeFileSync(join(out,'failure.txt'),cap());writeFileSync(join(out,'runtime.log'),readFileSync(join(dir,'runtime.log')))}catch{}throw error}
finally{try{tm('kill-server')}catch{}rmSync(dir,{recursive:true,force:true})}
