/** @description Durable Unicode draft/cursor crash recovery and held admission, six disposable PTYs. */
import{execFileSync}from'node:child_process';import{mkdtempSync,mkdirSync,writeFileSync,readFileSync,rmSync}from'node:fs';import{tmpdir}from'node:os';import{join,resolve}from'node:path';
const bin=process.env.GI_TUI_BIN||resolve('bin/gi'),out=resolve('test-results/tui-durable-draft');mkdirSync(out,{recursive:true});
const run=(cmd,args)=>execFileSync(cmd,args,{encoding:'utf8',timeout:15000}),sleep=ms=>new Promise(r=>setTimeout(r,ms)),assert=(v,m)=>{if(!v)throw Error(m)};
async function wait(fn,label){for(let i=0;i<160;i++){if(fn())return;await sleep(60)}throw Error('timeout: '+label)}
const results=[];
for(const mode of ['fullscreen','regular'])for(const[width,height]of[[60,18],[100,22],[140,36]]){
 const dir=mkdtempSync(join(tmpdir(),'gi-durable-draft-')),db=join(dir,'state.db'),socket=`gi-draft-${process.pid}-${mode}-${width}`,pane='proof:0.0';mkdirSync(join(dir,'.pi'));const settings=JSON.stringify({defaultProvider:'test',defaultModel:'test-model',enabledModels:['test-model']});writeFileSync(join(dir,'.pi/settings.json'),settings);
 const tm=(...a)=>run('tmux',['-L',socket,...a]),keys=(...a)=>tm('send-keys','-t',pane,...a),type=s=>keys('-l',s),cap=()=>tm('capture-pane','-p','-t',pane),all=()=>tm('capture-pane','-p','-S','-','-t',pane),sql=q=>run('sqlite3',['-cmd','.timeout 5000',db,q]).trim(),shot=name=>writeFileSync(join(out,`${mode}-${width}-${name}.txt`),all());
 const launch=()=>tm('new-session','-d','-s','proof','-x',String(width),'-y',String(height),`cd '${dir}' && HOME='${dir}' '${bin}' -tui -tui-mode ${mode} -db '${db}' -workspace '${dir}'; echo EXIT; sleep 60`);
 const kill=()=>{const pid=Number(tm('display-message','-p','-t',pane,'#{pane_pid}').trim());const child=run('pgrep',['-P',String(pid)]).trim().split('\n').map(Number).find(Boolean);run('kill',['-KILL',String(child)]);tm('kill-session','-t','proof')};
 const command=async text=>{keys('C-e','C-u');type(text);keys('Enter');await sleep(180)};
 try{
  launch();await wait(()=>cap().includes('%/'),'ready');const id=sql('select id from sessions limit 1');
  const journal=()=>JSON.parse(sql(`select cast(value as text) from kv_store where namespace='tui_text_draft_v1' and key='${id}'`)||'{}');
  const draft='first 中文🙂\nsecond β';type('\x1b[200~'+draft+'\x1b[201~');keys('Left','Left');await wait(()=>journal().text===draft&&journal().cursor===Array.from(draft).length-2,'saved draft/cursor');shot('saved');kill();launch();await wait(()=>cap().includes('second'),'recovered');assert(journal().text===draft,'restart changed bytes');type('X');await wait(()=>journal().text==='first 中文🙂\nsecondX β','cursor restored');shot('restored');
  // Clear deliberately, then send once and crash after confirmed admission.
  keys('C-e','C-u','BSpace','C-u');type('echo durable once');keys('Enter');await wait(()=>sql("select count(*) from turns where status='completed'")==='1','one admission');await wait(()=>!journal().claim,'claim retired');kill();launch();await wait(()=>cap().includes('test-model'),'reopen accepted');assert(journal().text===''&&!journal().claim,'accepted prompt recovered unsent');assert(sql('select count(*) from turns')==='1','reopen resent');
  // External revision wins; local typing must remain visibly unsaved.
  type('local start');await wait(()=>journal().text==='local start','local saved');sql(`update kv_store set value=json_set(value,'$.text','other terminal','$.cursor',3,'$.revision',json_extract(value,'$.revision')+1) where namespace='tui_text_draft_v1' and key='${id}'`);type(' conflict');await wait(()=>all().includes('not saved'),'conflict visible');keys('Enter');await sleep(180);assert(sql('select count(*) from turns')==='1','conflict submitted');assert(journal().text==='other terminal','external overwritten');shot('conflict');await command('/draft reload');await wait(()=>cap().replaceAll('▌','').includes('other terminal'),'reload stored');
  // A paired dispatch lost before an admission remains held, never restored.
  kill();const current=journal(),token='held-crash-token',revision=current.revision+1;
  const held={text:'',cursor:0,revision,claim:{text:'unknown old prompt',cursor:4,token,revision,dispatched:true}};sql(`update kv_store set value='${JSON.stringify(held)}' where namespace='tui_text_draft_v1' and key='${id}'`);
  launch();await wait(()=>all().includes('held submission recovered'),'held restart');await command('/draft check');await wait(()=>all().includes('unresolved'),'unknown held');await command('/draft release '+token);await wait(()=>all().includes('refused'),'dispatched release refused');keys('C-e','C-u');type('newer while held');await wait(()=>journal().text==='newer while held','newer saved');keys('Enter');await sleep(180);assert(sql('select count(*) from turns')==='1','unknown replay');assert(journal().claim.token===token,'unknown claim dropped');shot('held');
  // Undispatched claims can be explicitly released, with a full production-
  // length token visible in the narrow regular transcript. No admission occurs.
  kill();const releaseToken='tui-text-'+('ab'.repeat(32)),nextRevision=journal().revision+1;
  const unstarted={text:'',cursor:0,revision:nextRevision,claim:{text:'released exact 中文🙂',cursor:4,token:releaseToken,revision:nextRevision}};
  sql(`update kv_store set value='${JSON.stringify(unstarted)}' where namespace='tui_text_draft_v1' and key='${id}'`);
  launch();await wait(()=>all().includes('held submission recovered'),'unstarted recovery');await command('/draft');
  // tmux capture includes the TUI's right-hand border between wrapped token rows.
  // Inspect only the held-token block, stripping layout glyphs and whitespace.
  const visibleToken=()=>{const screen=all(),start=screen.lastIndexOf('token (join wrapped lines):');if(start<0)return '';const end=screen.indexOf('draft: /draft',start);return screen.slice(start,end<0?undefined:end).replace(/[\s│█]/g,'');};
  await wait(()=>visibleToken().includes(releaseToken),'full token visible');
  await command('/draft release '+releaseToken);await wait(()=>!journal().claim&&journal().text==='released exact 中文🙂','explicit release restored');
  assert(journal().cursor===4,'release cursor changed');assert(sql('select count(*) from turns')==='1','release submitted');shot('release');
  assert(readFileSync(join(dir,'.pi/settings.json'),'utf8')===settings,'settings changed');if(mode==='regular')assert(tm('display-message','-p','-t',pane,'#{alternate_on} #{mouse_any_flag}').trim()==='0 0','regular ownership');
  const screen=cap().trimEnd().split('\n'),bars=screen.map((x,i)=>/─{10}/.test(x)?i:-1).filter(i=>i>=0);assert(bars.at(-1)-bars.at(-2)===2,'idle growth');
  results.push({mode,width,height,crashUnicodeCursor:true,acceptedNotRestored:true,noReplay:true,conflictProtected:true,heldRestart:true,newerTextSaved:true,fullTokenVisible:true,explicitReleaseNoSubmit:true,regularOwnership:true,zeroIdleGrowth:true});
 }catch(error){try{shot('failure')}catch{}throw error}finally{try{tm('kill-server')}catch{}rmSync(dir,{recursive:true,force:true})}
}
writeFileSync(join(out,'summary.json'),JSON.stringify(results,null,2));console.log(JSON.stringify(results,null,2));
