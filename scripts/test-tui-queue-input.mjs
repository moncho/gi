/** PTY proof that Enter steers and Alt+Enter creates a separate durable follow-up. */
import{spawnSync}from'node:child_process';import{mkdtempSync,mkdirSync,writeFileSync,readFileSync,rmSync}from'node:fs';import{tmpdir}from'node:os';import{join,resolve}from'node:path';
const binary=process.env.GI_TUI_BIN||resolve('bin/gi'),shell=resolve('tests/ux/shell'),out=resolve(`test-results/tui-queue-input/run-${Date.now()}`);mkdirSync(out,{recursive:true});
const run=(c,a)=>{const r=spawnSync(c,a,{encoding:'utf8',timeout:12000});if(r.status!==0)throw Error(`${c}: ${r.stderr}`);return r.stdout};const sleep=ms=>new Promise(r=>setTimeout(r,ms));const wait=async(f,label)=>{let end=Date.now()+15000;while(Date.now()<end){if(f())return;await sleep(70)}throw Error('Timed out '+label)};const quote=s=>`'${s.replaceAll("'","''")}'`;const results=[];
for(const mode of ['fullscreen','regular']){
 const dir=mkdtempSync(join(tmpdir(),'gi-queue-input-')),db=join(dir,'gi.db'),gates=join(dir,'gates'),socket=`gi-queue-input-${process.pid}-${mode}`,pane='proof:0.0';mkdirSync(gates);mkdirSync(join(dir,'.pi'));writeFileSync(join(dir,'.pi/settings.json'),JSON.stringify({model:'test-model',enabledModels:['test-model']}));
 const tm=(...a)=>run('tmux',['-L',socket,...a]);const cap=()=>tm('capture-pane','-p','-t',pane).replaceAll('\u00a0',' ');const sql=q=>run('sqlite3',['-cmd','.timeout 5000',db,q]).trim();const send=(text,key='Enter')=>{tm('send-keys','-t',pane,'-l',text);tm('send-keys','-t',pane,key)};
 try{
  tm('new-session','-d','-s','proof','-x','100','-y','32',`cd '${dir}' && HOME='${dir}' PATH='${shell}':"$PATH" GI_UX_QUEUE_GATES='${gates}' '${binary}' -tui -tui-mode ${mode} -db '${db}' -workspace '${dir}' -model test-model 2>'${dir}/runtime.log'; sleep 20`);tm('set-option','-t','proof','status','off');await wait(()=>cap().includes('%/'),'startup');
  send('UX queue gate:held');await wait(()=>sql("select count(*) from turns where status='running';")==='1','active turn');const sid=sql('select id from sessions limit 1'),active=sql("select id from turns where status='running' limit 1");
  // Admission readiness belongs to the current captured turn, not a sleep.
  await wait(()=>sql("select count(*) from turn_events where event_type='tool.started';")!=='0','held tool started');
  send('steer at next boundary');await wait(()=>sql("select count(*) from steering_queue where content='steer at next boundary';")==='1','Enter steering');
  if(sql('select count(*) from turns;')!=='1')throw Error('Enter created follow-up turn');
  tm('send-keys','-t',pane,'-l','follow after completion');
  // Explicit CSI-u Alt+Enter, independent of terminal's Alt fullscreen binding.
  tm('send-keys','-t',pane,'-H','1b','5b','31','33','3b','33','75');
  await wait(()=>sql("select count(*) from turns where prompt='follow after completion' and status='queued';")==='1','Alt+Enter follow-up');
  if(sql("select count(*) from steering_queue where content='follow after completion';")!=='0')throw Error('Alt+Enter incorrectly steered');
  if(sql(`select count(*) from turns where prompt='follow after completion' and json_extract(metadata_json,'$.intent')='queue' and session_id=${quote(sid)};`)!=='1')throw Error('follow-up lost session/intent');
  if(sql(`select status from turns where id=${quote(active)};`)!=='running')throw Error('test failed to hold active turn');
  writeFileSync(join(out,mode+'-queued.txt'),cap());writeFileSync(join(out,mode+'-queued.ansi'),tm('capture-pane','-p','-e','-t',pane));
  writeFileSync(join(gates,'held'),'go');await wait(()=>sql("select count(*) from turns where status in ('running','queued');")==='0','queue drain');
  tm('send-keys','-t',pane,'-l','idle Alt+Enter');tm('send-keys','-t',pane,'-H','1b','5b','31','33','3b','33','75');await wait(()=>sql("select count(*) from turns where prompt='idle Alt+Enter' and status='completed';")==='1','idle Alt+Enter');
  if(sql("select count(*) from messages where role='user' and content='follow after completion';")!=='1')throw Error('follow-up duplicate/lost user message');
  results.push({mode,result:'pass',active});
 }catch(e){results.push({mode,result:'fail',error:String(e)});try{writeFileSync(join(out,mode+'-failure.txt'),cap());writeFileSync(join(out,mode+'-runtime.log'),readFileSync(join(dir,'runtime.log')))}catch{}}
 finally{writeFileSync(join(gates,'held'),'go');try{tm('kill-server')}catch{}rmSync(dir,{recursive:true,force:true})}
}
writeFileSync(join(out,'results.json'),JSON.stringify({scope:'Actual tmux input and native durable admission; shell fixture, no remote provider',results},null,2));console.log(JSON.stringify({out,results},null,2));if(results.some(r=>r.result==='fail'))process.exitCode=1;
