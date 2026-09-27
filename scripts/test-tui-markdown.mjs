/** Native tmux acceptance for stored assistant Markdown (not just projector unit output).
 * Run with: make test-tui-markdown. No network/model credentials required.
 */
import {spawnSync} from 'node:child_process';
import {mkdtempSync,mkdirSync,writeFileSync,readFileSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {resolve,join} from 'node:path';
import {generateMessages} from '@cucumber/gherkin';
import {IdGenerator,SourceMediaType} from '@cucumber/messages';

const binary=process.env.GI_TUI_BIN||resolve('bin/gi');
const artifacts=resolve('test-results/tui-markdown');mkdirSync(artifacts,{recursive:true});
const run=(cmd,args)=>{const r=spawnSync(cmd,args,{encoding:'utf8',timeout:15000});if(r.status!==0)throw Error(`${cmd} ${args.join(' ')}: ${r.stderr}`);return r.stdout;};
const sleep=ms=>new Promise(r=>setTimeout(r,ms));
const wait=async(fn,label)=>{const end=Date.now()+15000;while(Date.now()<end){if(fn())return;await sleep(80);}throw Error(`Timed out: ${label}`);};
const assert=(ok,label)=>{if(!ok)throw Error(label);};
const quote=s=>`'${s.replaceAll("'","''")}'`;
const fixtures=[
 {name:'plain-output',source:'Plain assistant output',lines:['Plain assistant output','shell','  if ready { return 42 }'],absent:['last hidden line']},
 {name:'heading-list',source:'# Rendered heading\n\nA **bold** and *italic* phrase.\n\n- alpha item\n- beta item',
  lines:['RENDERED HEADING','================','A bold and italic phrase.','• alpha item','• beta item'],absent:['**bold**','*italic*','# Rendered heading']},
 {name:'code-quote',source:'> Quoted words\n\nUse `inline()` now.\n\n```js\nconst answer = 42;\n  return answer;\n```',
  lines:['> Quoted words','Use inline() now.','[code:js] 2 lines','    const answer = 42;','      return answer;'],absent:['```js','`inline()`']},
 {name:'table-link',source:'| Name | Value |\n| --- | --- |\n| First | 世界 |\n\nVisit [docs](https://example.invalid/docs).',
  lines:['| Name | Value |','| First | 世界 |','docs (https://example.invalid/docs)'],absent:['[docs]','| --- |']},
];
const featurePath=resolve('features/tui-markdown/rendering.feature');
const envelopes=generateMessages(readFileSync(featurePath,'utf8'),featurePath,
 SourceMediaType.TEXT_X_CUCUMBER_GHERKIN_PLAIN,
 {newId:IdGenerator.incrementing(),includeGherkinDocument:true,includePickles:true});
const errors=envelopes.filter(e=>e.parseError);
if(errors.length)throw Error(`Invalid Gherkin: ${JSON.stringify(errors)}`);
const scenarios=envelopes.filter(e=>e.pickle).map(e=>e.pickle);
if(scenarios.length!==24)throw Error(`Expected 24 Markdown scenarios, found ${scenarios.length}`);
const results=[],failures=[];
for(const scenario of scenarios){
 const steps=scenario.steps.map(step=>step.text);
 const fixtureName=steps[0]?.match(/^a stored assistant Markdown fixture "([^"]+)"$/)?.[1];
 const [,mode,size]=steps[1]?.match(/^I open the transcript in (fullscreen|regular) at (60x18|100x22|140x36)$/)||[];
 const fixture=fixtures.find(f=>f.name===fixtureName);
 const expected={
  'plain-output':['assistant and tool output have no boxes or outcome backgrounds'],
  'heading-list':['headings emphasis and list bullets are projected without source markers'],
  'code-quote':['inline code has ANSI highlighting without splitting or losing words','fenced code retains its four-space indentation and quote text'],
  'table-link':['table cells and the link target remain readable without raw Markdown'],
 }[fixtureName];
 if(!fixture||!mode||!expected||JSON.stringify(steps.slice(2))!==JSON.stringify([...expected,'the Markdown remains readable after a resize with an unsent draft']))
  throw Error(`Unsupported Gherkin steps in ${scenario.name}: ${steps.join(' | ')}`);
 const [width,height]=size.split('x').map(Number);
 const dir=mkdtempSync(join(tmpdir(),'gi-markdown-')),socket=`gi-md-${process.pid}-${mode}-${width}-${fixture.name}`,pane='proof:0.0',db=join(dir,'state.db');
 const tm=(...a)=>run('tmux',['-L',socket,...a]);
 const cap=()=>tm('capture-pane','-p','-t',pane).replaceAll('\u00a0',' ');
 const ansi=()=>tm('capture-pane','-p','-e','-t',pane);
 const sql=q=>run('sqlite3',['-cmd','.timeout 5000',db,q]).trim();
 const launch=()=>{tm('new-session','-d','-s','proof','-x',String(width),'-y',String(height),`cd '${dir}' && HOME='${dir}' TERM=xterm-256color COLORTERM=truecolor '${binary}' -tui -tui-mode ${mode} -db '${db}' -workspace '${dir}' -model test-model 2>'${dir}/runtime.log'; echo MARKDOWN_EXITED; sleep 30`);tm('set-option','-t','proof','status','off');};
 const shot=label=>{writeFileSync(join(artifacts,`${mode}-${width}x${height}-${fixture.name}-${label}.txt`),cap());writeFileSync(join(artifacts,`${mode}-${width}x${height}-${fixture.name}-${label}.ansi`),ansi());};
 try{
  mkdirSync(join(dir,'.pi'));writeFileSync(join(dir,'.pi/settings.json'),JSON.stringify({model:'test-model',enabledModels:['test-model']}));
  launch();await wait(()=>cap().includes('m0/t0'),'initial TUI');
  const id=sql('select id from sessions limit 1;');assert(id,'missing session');
  tm('send-keys','-t',pane,'C-d');await wait(()=>cap().includes('MARKDOWN_EXITED'),'close before seeding');tm('kill-session','-t','proof');
  // Seed the real store; reopening exercises transcript loading, layout and terminal ANSI output.
  sql(`insert into messages(id,session_id,role,content,payload_json,created_at) values('user',${quote(id)},'user','Markdown check','{}','2026-01-01'),('assistant',${quote(id)},'assistant',${quote(fixture.source)},'{}','2026-01-02')${fixture.name==='plain-output'?`,('tool-output',${quote(id)},'tool_result','  if ready { return 42 }\nsecond line\nlast hidden line','{"tool_name":"shell"}','2026-01-03')`:''};`);
  launch();await wait(()=>cap().includes(fixture.name==='plain-output'?'m3/t0':'m2/t0'),'stored Markdown rendered');
  const check=label=>{
   const screen=cap(),rows=screen.split('\n');
   const problems=[];
   for(const line of fixture.lines){
    // Table columns carry alignment padding; compare cell contents while
    // preserving strict indentation assertions for code and tool output.
    const tableRow=fixture.name==='table-link'&&line.startsWith('|');
    const visible=row=>tableRow?row.replace(/ +/g,' '):row;
    if(!rows.some(row=>visible(row).includes(visible(line))))problems.push(`missing ${JSON.stringify(line)}`);
   }
   for(const literal of fixture.absent)if(!(fixture.name==='plain-output'&&mode==='regular')&&screen.includes(literal))problems.push(`raw Markdown leaked: ${literal}`);
   if(label==='initial'&&!rows.some(row=>row.includes('Markdown check')))problems.push('user message missing');
   if(screen.includes('\x00gi-code-'))problems.push('inline style marker leaked');
   if(/\b(?:you|Gi): /.test(screen))problems.push('speaker label leaked into transcript');
   if(fixture.name==='plain-output'){
    const transcript=screen.split(/^[ ─]{10,}$/m)[0];
    if(/[╭╮╰╯│]/.test(transcript))problems.push('old box border visible');
    if(mode==='fullscreen'&&!transcript.includes('more line(s)'))problems.push('tool not collapsed');
    if(mode==='regular'&&!transcript.includes('last hidden line'))problems.push('terminal scrollback lost full tool output');
   }
   if(fixture.name==='code-quote'){
    const label=rows.find(row=>row.includes('[code:js]'));
    const code=rows.find(row=>row.includes('const answer = 42;'));
    if(label&&code&&code.search(/\S/)<label.search(/\S/)+4)problems.push('fenced code lost four-space indentation');
    const indented=rows.find(row=>row.includes('return answer;'));
    if(label&&indented&&indented.search(/\S/)<label.search(/\S/)+6)problems.push('source indentation inside fenced code lost');
   }
   // These fixtures are deliberately short enough to be visible in a 60x18 viewport.
   const content=rows.slice(0,rows.findIndex(row=>/^\s*─{10,}\s*$/.test(row)));
   if(!content.every(row=>[...row].length<=width))problems.push('transcript overflow');
   assert(problems.length===0,`${label}: ${problems.join('; ')}`);
  };
  check('initial');
  const styled=ansi().replaceAll('\u00a0',' ');
  // tmux may repeat foreground/bold SGR between words. Keep background
  // boundaries while comparing visible text, rather than requiring contiguous
  // bytes that cease to exist when a heading is styled span by span.
  const backgrounds=styled.replace(/\x1b\[(?!48;2;|49m)[0-9;]*m/g,'');
  assert(/\x1b\[48;2;52;53;65m[\s\S]*Markdown[\s\S]*\x1b\[49m[\s\S]*(?:Plain assistant output|RENDERED HEADING|Use |Name)/.test(backgrounds),
   'user background band or assistant terminal-background reset missing');
  if(fixture.name==='plain-output'){
   const output=backgrounds.slice(backgrounds.indexOf('Plain assistant output'));
   assert(!/\x1b\[48;2;/.test(output), 'assistant/tool output has a colored background');
  }
  if(fixture.name==='code-quote'){
   assert(/Use \x1b\[2m\x1b\[90minline\(\)\x1b\[0m now\./.test(styled),
    'inline ANSI style split the sentence or leaked into following words');
  }
  if(fixture.name==='table-link'){
   const target='https://example.invalid/docs';
   assert(styled.includes(`\x1b]8;;${target}\x1b\\${target}\x1b]8;;\x1b\\`),
    'OSC 8 link target missing or unclosed');
  }
  shot('initial');
  tm('send-keys','-t',pane,'-l','unsent draft');await wait(()=>cap().includes('unsent draft'),'draft');
  tm('resize-window','-t','proof','-x',String(width+8),'-y',String(height+3));await wait(()=>cap().includes(fixture.name==='plain-output'?'m3/t0':'m2/t0'),'resized Markdown');
  tm('resize-window','-t','proof','-x',String(width),'-y',String(height));await wait(()=>cap().includes('unsent draft'),'resize round trip');
  check('resized');shot('resized');
  assert(sql('select count(*) from messages;')===(fixture.name==='plain-output'?'3':'2'),'rendering or draft submitted a message');
  results.push(`${scenario.name}: stored projection, ANSI, viewport, resize, draft`);
 }catch(error){failures.push(`${scenario.name}: ${error.message}`);try{shot('failure');writeFileSync(join(artifacts,`${mode}-${width}-${fixture.name}-runtime.log`),readFileSync(join(dir,'runtime.log')));}catch{}}
 finally{try{tm('kill-server');}catch{}rmSync(dir,{recursive:true,force:true});}
}
writeFileSync(join(artifacts,'summary.txt'),[...results,...failures].join('\n')+'\n');console.log(results.join('\n'));if(failures.length){console.error(failures.join('\n'));process.exitCode=1;}
