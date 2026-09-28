/** Real terminal regression for screenshot 3139: inline path order before tool panels. */
import {spawnSync} from 'node:child_process';
import {mkdtempSync,mkdirSync,writeFileSync,readFileSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join,resolve} from 'node:path';
import {generateMessages} from '@cucumber/gherkin';
import {IdGenerator,SourceMediaType} from '@cucumber/messages';
const binary=process.env.GI_TUI_BIN||resolve('bin/gi');
const out=resolve(process.env.GI_TUI_PROSE_OUTPUT||`test-results/tui-inline-prose/run-${Date.now()}`);mkdirSync(out,{recursive:true});
const source='I added these tests under `internal/tui` and the suite passed. I left the other uncommitted changes untouched.';
const expected=source.replaceAll('`','');
const cases=generateMessages(readFileSync('features/gi/tui/inline-prose-layout.feature','utf8'),'inline-prose-layout.feature',SourceMediaType.TEXT_X_CUCUMBER_GHERKIN_PLAIN,{newId:IdGenerator.incrementing(),includePickles:true,includeGherkinDocument:true});
if(cases.some(x=>x.parseError))throw Error(JSON.stringify(cases.filter(x=>x.parseError)));
const scenarios=cases.filter(x=>x.pickle).map(x=>x.pickle);if(scenarios.length!==6)throw Error('Expected six inline-prose scenarios');
const run=(cmd,args)=>{const r=spawnSync(cmd,args,{encoding:'utf8',timeout:15000});if(r.status!==0)throw Error(`${cmd}: ${r.stderr}`);return r.stdout;};
const sleep=ms=>new Promise(r=>setTimeout(r,ms));
const wait=async(f,label)=>{const end=Date.now()+12000;while(Date.now()<end){if(f())return;await sleep(70)}throw Error('Timed out: '+label)};
const quote=s=>`'${s.replaceAll("'","''")}'`;
const results=[];
for(const scenario of scenarios){
 const [,mode,size]=scenario.name.match(/at (fullscreen|regular) (\d+x\d+)$/)||[];if(!mode)throw Error('Unsupported scenario '+scenario.name);
 const expectedSteps=['an assistant reply has prose before and after the inline path "internal/tui"','the next two stored messages are shell tool results',`I open the stored transcript in ${mode} at ${size}`,'the assistant sentence stays in reading order with width-dependent wrapping','the two tool results immediately follow in stored order','the inline path appears once without a lost word boundary','assistant and tool output are not surrounded by decorative boxes','resizing preserves that reading order and an unsent draft'];
 if(JSON.stringify(scenario.steps.map(s=>s.text))!==JSON.stringify(expectedSteps))throw Error('Unsupported Gherkin clauses: '+scenario.name);
 const [width,height]=size.split('x').map(Number),dir=mkdtempSync(join(tmpdir(),'gi-inline-')),db=join(dir,'state.db'),socket=`gi-inline-${process.pid}-${mode}-${width}`,pane='proof:0.0';
 const tm=(...a)=>run('tmux',['-L',socket,...a]);
 const cap=(ansi=false)=>tm('capture-pane','-p',...(ansi?['-e']:[]),'-t',pane).replaceAll('\u00a0',' ');
 const sql=q=>run('sqlite3',['-cmd','.timeout 5000',db,q]).trim();
 const launch=()=>{tm('new-session','-d','-s','proof','-x',String(width),'-y',String(height),`cd '${dir}' && HOME='${dir}' TERM=xterm-256color COLORTERM=truecolor '${binary}' -tui -tui-mode ${mode} -db '${db}' -workspace '${dir}' -model test-model 2>'${dir}/runtime.log'; echo PROSE_EXITED; sleep 30`);tm('set-option','-t','proof','status','off');};
 const shot=label=>{writeFileSync(join(out,`${mode}-${size}-${label}.txt`),cap());writeFileSync(join(out,`${mode}-${size}-${label}.ansi`),cap(true));};
 const check=()=>{
  const screen=cap(),start=screen.indexOf('I added'),end=screen.indexOf('shell',start);
  if(start<0||end<=start)throw Error('Assistant/tool boundaries missing');
  const part=screen.slice(start,end);
  const joined=part.trim().split('\n').map(s=>s.trim()).join(' ').replace(/\s+/g,' ').trim();
  if(joined!==expected)throw Error('Assistant reading order differs: '+JSON.stringify(joined));
  if((part.match(/internal\/tui/g)||[]).length!==1)throw Error('Path missing/duplicated');
  const assistantRows=part.trim().split('\n').filter(s=>s.trim());
  if(width===60&&assistantRows.length<2)throw Error('Narrow viewport did not wrap');
  if(width===140&&assistantRows.length!==1)throw Error('Wide viewport wrapped unexpectedly');
  if(/[╭╮╰╯│]/.test(screen))throw Error('Decorative output box visible');
  const rows=screen.split('\n').map(s=>s.trim()).filter(Boolean),first=rows.findIndex(s=>s.includes('I added'));
  const tail=rows.slice(first+assistantRows.length,first+assistantRows.length+4);
  if(JSON.stringify(tail)!==JSON.stringify(['shell','features/example.feature','shell','second tool output']))throw Error('Tool blocks out of order: '+JSON.stringify(tail));
  if(rows.filter(s=>s==='shell').length!==2||screen.indexOf('shell')!==end)throw Error('Unexpected tool before/around assistant prose');
 };
 try{
  mkdirSync(join(dir,'.pi'));writeFileSync(join(dir,'.pi/settings.json'),JSON.stringify({model:'test-model',enabledModels:['test-model']}));launch();await wait(()=>cap().includes('m0/t0'),'ready');
  const id=sql('select id from sessions limit 1;');tm('send-keys','-t',pane,'C-d');await wait(()=>cap().includes('PROSE_EXITED'),'close');tm('kill-session','-t','proof');
  sql(`insert into messages(id,session_id,role,content,payload_json,created_at) values('u',${quote(id)},'user','Please store these tests','{}','2026-01-01'),('a',${quote(id)},'assistant',${quote(source)},'{}','2026-01-02'),('t1',${quote(id)},'tool_result','features/example.feature','{"tool_name":"shell"}','2026-01-03'),('t2',${quote(id)},'tool_result','second tool output','{"tool_name":"shell"}','2026-01-04');`);
  launch();await wait(()=>cap().includes('m4/t0'),'loaded');shot('initial');check();
  tm('send-keys','-t',pane,'-l','unsent prose draft');await wait(()=>cap().includes('unsent prose draft'),'draft');
  tm('resize-window','-t','proof','-x',String(width+7),'-y',String(height+2));await sleep(150);tm('resize-window','-t','proof','-x',String(width),'-y',String(height));await wait(()=>cap().includes('unsent prose draft'),'resize');check();shot('resized');
  if(sql('select count(*) from messages;')!=='4')throw Error('Rendering submitted a prompt');
  results.push({scenario:scenario.name,result:'pass'});
 }catch(error){results.push({scenario:scenario.name,result:'fail',error:String(error)});try{shot('failure')}catch{}}
 finally{try{tm('kill-server')}catch{}rmSync(dir,{recursive:true,force:true});}
}
writeFileSync(join(out,'results.json'),JSON.stringify({binary,scope:'Stored synthetic transcript; real tmux layout, resize and draft, no provider calls',results},null,2));console.log(JSON.stringify({out,results},null,2));if(results.some(r=>r.result==='fail'))process.exitCode=1;
