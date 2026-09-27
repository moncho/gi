/** Real tmux/PTY mouse scrollbar acceptance; make test-tui-scrollbar. */
import {spawnSync} from 'node:child_process';
import {mkdtempSync,mkdirSync,writeFileSync,readFileSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join,resolve} from 'node:path';
import {generateMessages} from '@cucumber/gherkin';
import {IdGenerator,SourceMediaType} from '@cucumber/messages';
const bin=process.env.GI_TUI_BIN||resolve('bin/gi'),out=resolve('test-results/tui-scrollbar');mkdirSync(out,{recursive:true});
const run=(cmd,args)=>{const r=spawnSync(cmd,args,{encoding:'utf8',timeout:15000});if(r.status!==0)throw Error(`${cmd} ${args.join(' ')}: ${r.stderr}`);return r.stdout;};
const sleep=ms=>new Promise(r=>setTimeout(r,ms)),assert=(ok,msg)=>{if(!ok)throw Error(msg);};
async function wait(fn,label){const end=Date.now()+15000;while(Date.now()<end){if(fn())return;await sleep(80);}throw Error('Timed out: '+label);}
const feature=resolve('features/tui/scrollbar.feature');
const envelopes=generateMessages(readFileSync(feature,'utf8'),feature,SourceMediaType.TEXT_X_CUCUMBER_GHERKIN_PLAIN,{newId:IdGenerator.incrementing(),includePickles:true});
assert(!envelopes.some(e=>e.parseError),'Invalid scrollbar Gherkin');
const scenarios=envelopes.filter(e=>e.pickle).map(e=>e.pickle);
assert(scenarios.length===3,`Expected 3 scenarios; got ${scenarios.length}`);
const results=[],failures=[];
for(const scenario of scenarios){
 const steps=scenario.steps.map(s=>s.text),size=steps[0]?.match(/^a long stored transcript in a (60x18|100x22|140x36) fullscreen tmux pane$/)?.[1];
 assert(size && JSON.stringify(steps.slice(1))===JSON.stringify([
  'I click the scrollbar track','the transcript jumps toward the middle',
  'I drag the scrollbar thumb above the top of the pane','the transcript shows its first marker',
  'I drag the scrollbar thumb below the bottom of the pane','the transcript shows its last marker',
  'my unsent draft is unchanged']),`Unsupported Gherkin steps: ${steps.join(' | ')}`);
 const [width,height]=size.split('x').map(Number),dir=mkdtempSync(join(tmpdir(),'gi-scrollbar-'));
 const socket=`gi-scrollbar-${process.pid}-${width}`,db=join(dir,'state.db'),pane='proof:0.0';
 const tm=(...a)=>run('tmux',['-L',socket,...a]),cap=()=>tm('capture-pane','-p','-t',pane),keys=(...a)=>tm('send-keys','-t',pane,...a);
 const sql=q=>run('sqlite3',['-cmd','.timeout 5000',db,q]).trim();
 const sgr=(code,x,y,end='M')=>keys('-H',...Buffer.from(`\x1b[<${code};${x+1};${y+1}${end}`).toString('hex').match(/../g));
 const markers=()=>[...cap().matchAll(/SCROLL MARKER (\d{3})/g)].map(m=>Number(m[1]));
 const shot=label=>{writeFileSync(join(out,`${size}-${label}.txt`),cap());writeFileSync(join(out,`${size}-${label}.ansi`),tm('capture-pane','-p','-e','-t',pane));};
 const launch=()=>{tm('new-session','-d','-s','proof','-x',String(width),'-y',String(height),`cd '${dir}' && HOME='${dir}' TERM=xterm-256color COLORTERM=truecolor '${bin}' -tui -tui-mode fullscreen -db '${db}' -workspace '${dir}' -model test-model 2>'${dir}/runtime.log'; echo SCROLL_EXITED; sleep 30`);tm('set-option','-t','proof','status','off');};
 try{
  mkdirSync(join(dir,'.pi'));writeFileSync(join(dir,'.pi/settings.json'),JSON.stringify({model:'test-model',enabledModels:['test-model']}));
  launch();await wait(()=>cap().includes('m0/t0'),'bootstrap');
  const session=sql('select id from sessions limit 1;');assert(session,'missing session');
  keys('C-d');await wait(()=>cap().includes('SCROLL_EXITED'),'seed shutdown');tm('kill-session','-t','proof');
  const values=Array.from({length:100},(_,i)=>`('scroll-${i}','${session}','system','SCROLL MARKER ${String(i).padStart(3,'0')}','{}','2026-01-01T00:${String(Math.floor(i/60)).padStart(2,'0')}:${String(i%60).padStart(2,'0')}Z')`).join(',');
  sql(`insert into messages(id,session_id,role,content,payload_json,created_at) values ${values};`);
  launch();await wait(()=>cap().includes('SCROLL MARKER 099'),'stored transcript');
  keys('-l','unsent scrollbar draft');await wait(()=>cap().includes('unsent scrollbar draft'),'draft');
  const baseline=markers();assert(baseline.includes(99) && !baseline.includes(0),'initial position not at bottom');shot('bottom');
  // Identify the actual painted gutter, not the terminal edge (which may have padding).
  const rows=cap().split('\n');let x=-1,ys=[];
  for(let col=width-1;col>=width-4;col--){
   const found=rows.flatMap((row,y)=>row[col]==='│'||row[col]==='█'?[y]:[]);
   if(found.length>=4){x=col;ys=found;break;}
  }
  assert(x>=0,`no visible scrollbar gutter in ${size}`);
  const top=ys[0],bottom=ys.at(-1),mid=Math.floor((top+bottom)/2);
  sgr(0,x,mid);sgr(0,x,mid,'m');
  await wait(()=>{const m=markers();return m.length>0 && !m.includes(99) && !m.includes(0);},'track click jumps to middle');shot('track');
  // Grab the thumb at its painted cell, then drive it outside both ends.
  const thumb=()=>{const lines=cap().split('\n');return ys.find(y=>lines[y]?.[x]==='█');};
  let y=thumb();assert(y!==undefined,'thumb missing after track click');
  sgr(0,x,y);sgr(32,x,0);sgr(0,x,0,'m');
  await wait(()=>markers().includes(0),'thumb drag to top');shot('top');
  y=thumb();assert(y!==undefined,'thumb missing at top');
  sgr(0,x,y);sgr(32,x,height-1);sgr(0,x,height-1,'m');
  await wait(()=>markers().includes(99),'thumb drag to bottom');shot('drag-bottom');
  assert(cap().replaceAll('▌','').includes('unsent scrollbar draft'),'draft changed while scrolling');
  assert(sql('select count(*) from turns;')==='0','mouse input submitted a turn');
  results.push(`${scenario.name}: track, thumb to top/bottom, draft preserved`);
 }catch(e){failures.push(`${scenario.name}: ${e.message}`);try{shot('failure');writeFileSync(join(out,`${size}-runtime.log`),readFileSync(join(dir,'runtime.log')));}catch{}}
 finally{try{tm('kill-server');}catch{}rmSync(dir,{recursive:true,force:true});}
}
writeFileSync(join(out,'summary.txt'),[...results,...failures].join('\n')+'\n');console.log(results.join('\n'));if(failures.length){console.error(failures.join('\n'));process.exitCode=1;}
