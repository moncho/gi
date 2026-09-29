/** Disposable PTY + SQLite proof of Gi's read-only pending panel. */
import {spawnSync} from 'node:child_process';
import {mkdtempSync,mkdirSync,writeFileSync,readFileSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join,resolve} from 'node:path';
const binary=process.env.GI_TUI_BIN||resolve('bin/gi'),shell=resolve('tests/ux/shell'),out=resolve(`test-results/tui-queue-display/run-${Date.now()}`);mkdirSync(out,{recursive:true});
const run=(cmd,args)=>{const result=spawnSync(cmd,args,{encoding:'utf8',timeout:12000});if(result.status!==0)throw Error(`${cmd}: ${result.stderr}`);return result.stdout};
const sleep=ms=>new Promise(r=>setTimeout(r,ms));const wait=async(fn,label)=>{const end=Date.now()+15000;while(Date.now()<end){if(fn())return;await sleep(80)}throw Error(`Timed out ${label}`)};const results=[];
for(const mode of ['fullscreen','regular']){
 const dir=mkdtempSync(join(tmpdir(),'gi-queue-display-')),db=join(dir,'gi.db'),gates=join(dir,'gates'),socket=`gi-queue-display-${process.pid}-${mode}`,pane='proof:0.0';mkdirSync(gates);mkdirSync(join(dir,'.pi'));writeFileSync(join(dir,'.pi/settings.json'),JSON.stringify({model:'test-model',enabledModels:['test-model']}));
 const tm=(...args)=>run('tmux',['-L',socket,...args]);const cap=()=>tm('capture-pane','-p','-t',pane).replaceAll('\u00a0',' ');const sql=query=>run('sqlite3',['-cmd','.timeout 5000',db,query]).trim();const send=(text,key='Enter')=>{tm('send-keys','-t',pane,'-l',text);tm('send-keys','-t',pane,key)};const altEnter=()=>tm('send-keys','-t',pane,'-H','1b','5b','31','33','3b','33','75');
 try{
  tm('new-session','-d','-s','proof','-x','100','-y','32',`cd '${dir}' && HOME='${dir}' PATH='${shell}':"$PATH" GI_UX_QUEUE_GATES='${gates}' '${binary}' -tui -tui-mode ${mode} -db '${db}' -workspace '${dir}' -model test-model 2>'${dir}/runtime.log'; sleep 20`);tm('set-option','-t','proof','status','off');await wait(()=>cap().includes('m0/t0'),'startup');
  send('UX queue gate:held');await wait(()=>sql("select count(*) from turn_events where event_type='tool.started';")==='1','held shell');
  send('steer at boundary');await wait(()=>sql("select count(*) from steering_queue where content='steer at boundary' and status='queued';")==='1','steer');
  tm('send-keys','-t',pane,'-l','follow after completion');altEnter();await wait(()=>sql("select count(*) from turns where prompt='follow after completion' and status='queued';")==='1','follow-up');
  await wait(()=>{const text=cap();return text.includes('Steering: steer at boundary')&&text.includes('Follow-up: follow after completion')&&text.includes('Alt+Up restores text-only queue')},'visible pending labels/hint');
  let text=cap(),steering=text.indexOf('Steering: steer at boundary'),followUp=text.indexOf('Follow-up: follow after completion');
  if(steering<0||followUp<=steering)throw Error('pending ordering reversed');
  writeFileSync(join(out,mode+'-pending.txt'),text);
  tm('send-keys','-t',pane,'-l','unsubmitted draft');await wait(()=>sql("select count(*) from kv_store where namespace='tui_text_draft_v1' and value like '%unsubmitted draft%';")==='1','durable draft');
  if(!cap().includes('Follow-up: follow after completion'))throw Error('typing draft hid queue');
  tm('resize-window','-t','proof','-x','64','-y','12');await wait(()=>cap().includes('queue: 2 pending; /queue to inspect'),'compact pending summary');
  writeFileSync(join(out,mode+'-compact.txt'),cap());
  tm('resize-window','-t','proof','-x','100','-y','32');await wait(()=>{const screen=cap();return screen.includes('Steering: steer at boundary')&&screen.includes('Follow-up: follow after completion')},'pending after resize');
  writeFileSync(join(gates,'held'),'go');await wait(()=>sql("select count(*) from turns where status in ('queued','running','steering');")==='0','queue drain');
  await wait(()=>!cap().includes('Steering: steer at boundary')&&!cap().includes('Follow-up: follow after completion'),'panel clear');
  if(sql("select count(*) from messages where role='user' and content in ('steer at boundary','follow after completion');")!=='2')throw Error('pending display disrupted delivery');
  if(sql("select count(*) from kv_store where namespace='tui_text_draft_v1' and value like '%unsubmitted draft%';")!=='1')throw Error('pending display changed draft');
  results.push({mode,result:'pass',steeringBeforeFollowUp:true,draftPreserved:true,clearedAfterDelivery:true});
 }catch(e){results.push({mode,result:'fail',error:String(e)});try{writeFileSync(join(out,mode+'-failure.txt'),cap());writeFileSync(join(out,mode+'-runtime.log'),readFileSync(join(dir,'runtime.log')))}catch{}}
 finally{writeFileSync(join(gates,'held'),'go');try{tm('kill-server')}catch{}rmSync(dir,{recursive:true,force:true})}
}
writeFileSync(join(out,'results.json'),JSON.stringify({scope:'Gi disposable PTY/SQLite pending display, no Pi PTY or live provider',results},null,2));console.log(JSON.stringify({out,results},null,2));if(results.some(r=>r.result==='fail'))process.exitCode=1;
