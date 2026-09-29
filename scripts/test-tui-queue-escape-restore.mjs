/** Disposable PTY/SQLite proof of active text-only Escape abort and restoration. */
import {spawnSync} from 'node:child_process';
import {mkdtempSync,mkdirSync,writeFileSync,readFileSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join,resolve} from 'node:path';
const binary=process.env.GI_TUI_BIN||resolve('bin/gi'),shell=resolve('tests/ux/shell'),out=resolve(`test-results/tui-queue-escape-restore/run-${Date.now()}`);mkdirSync(out,{recursive:true});
const run=(c,a)=>{const r=spawnSync(c,a,{encoding:'utf8',timeout:12000});if(r.status!==0)throw Error(`${c}: ${r.stderr}`);return r.stdout};
const sleep=ms=>new Promise(r=>setTimeout(r,ms));const wait=async(f,label)=>{const end=Date.now()+12000;while(Date.now()<end){if(f())return;await sleep(80)}throw Error(`Timed out ${label}`)};
const quote=s=>`'${s.replaceAll("'","''")}'`;const results=[];
for(const mode of ['fullscreen','regular']){
 const dir=mkdtempSync(join(tmpdir(),'gi-queue-escape-')),db=join(dir,'gi.db'),gates=join(dir,'gates'),socket=`gi-queue-escape-${process.pid}-${mode}`,pane='proof:0.0';mkdirSync(gates);mkdirSync(join(dir,'.pi'));writeFileSync(join(dir,'.pi/settings.json'),JSON.stringify({model:'test-model',enabledModels:['test-model']}));
 const tm=(...a)=>run('tmux',['-L',socket,...a]);const cap=()=>tm('capture-pane','-p','-t',pane).replaceAll('\u00a0',' ');const sql=q=>run('sqlite3',['-cmd','.timeout 5000',db,q]).trim();const send=(text,key='Enter')=>{tm('send-keys','-t',pane,'-l',text);tm('send-keys','-t',pane,key)};const altEnter=()=>tm('send-keys','-t',pane,'-H','1b','5b','31','33','3b','33','75');
 try{
  tm('new-session','-d','-s','proof','-x','100','-y','32',`cd '${dir}' && HOME='${dir}' PATH='${shell}':"$PATH" GI_UX_QUEUE_GATES='${gates}' '${binary}' -tui -tui-mode ${mode} -db '${db}' -workspace '${dir}' -model test-model 2>'${dir}/runtime.log'; sleep 20`);tm('set-option','-t','proof','status','off');await wait(()=>cap().includes('%/'),'startup');
  send('UX queue gate:held');await wait(()=>sql("select count(*) from turn_events where event_type='tool.started';")==='1','held tool');const sid=sql('select id from sessions limit 1');const active=sql("select id from turns where status='running' limit 1");
  for(const text of ['escape first','escape second']){tm('send-keys','-t',pane,'-l',text);altEnter();await wait(()=>sql(`select count(*) from turns where prompt=${quote(text)} and status='queued';`)==='1',`queue ${text}`)}
  tm('send-keys','-t',pane,'-l','newer draft');await wait(()=>sql(`select json_extract(value,'$.text') from kv_store where namespace='tui_text_draft_v1' and key=${quote(sid)};`)==='newer draft','draft');
  tm('send-keys','-t',pane,'Escape');
  await wait(()=>sql(`select json_extract(value,'$.text') from kv_store where namespace='tui_text_draft_v1' and key=${quote(sid)};`)==='escape first\n\nescape second\n\nnewer draft','queued text restored to journal');
  tm('send-keys','-t',pane,'-l',' still editing');
  await wait(()=>sql(`select json_extract(value,'$.text') from kv_store where namespace='tui_text_draft_v1' and key=${quote(sid)};`)==='escape first\n\nescape second\n\nnewer draft still editing','Escape retains composer focus and restored draft');
  const activeStatus=sql(`select status from turns where id=${quote(active)};`);const queued=sql("select count(*) from turns where prompt in ('escape first','escape second') and status='queued';");const restored=sql("select count(*) from turns where prompt in ('escape first','escape second') and status='cancelled';");const draft=sql(`select json_extract(value,'$.text') from kv_store where namespace='tui_text_draft_v1' and key=${quote(sid)};`);
  if(!['cancelling','cancelled'].includes(activeStatus)||queued!=='0'||restored!=='2'||draft!=='escape first\n\nescape second\n\nnewer draft still editing')throw Error(`unexpected Escape state: active=${activeStatus} queued=${queued} restored=${restored} draft=${JSON.stringify(draft)}`);
  writeFileSync(join(out,mode+'-escape.txt'),cap());
  writeFileSync(join(gates,'held'),'go');await wait(()=>sql(`select status from turns where id=${quote(active)};`)==='cancelled'&&sql(`select count(*) from session_active_turns where session_id=${quote(sid)};`)==='0','cancelled run cleanup');
  if(sql("select count(*) from messages where role='user' and content in ('escape first','escape second');")!=='0')throw Error('restored queued messages were redelivered');
  results.push({mode,result:'pass',scope:'text-only abort+restore',activeStatus,queued,restored,draft});
 }catch(e){results.push({mode,result:'fail',error:String(e)});try{writeFileSync(join(out,mode+'-failure.txt'),cap());writeFileSync(join(out,mode+'-runtime.log'),readFileSync(join(dir,'runtime.log')))}catch{}}
 finally{writeFileSync(join(gates,'held'),'go');try{tm('kill-server')}catch{}rmSync(dir,{recursive:true,force:true})}
}
writeFileSync(join(out,'results.json'),JSON.stringify({scope:'Gi disposable tmux/SQLite Escape restore, shell fixture, not Pi PTY or live provider',results},null,2));console.log(JSON.stringify({out,results},null,2));if(results.some(r=>r.result==='fail'))process.exitCode=1;
