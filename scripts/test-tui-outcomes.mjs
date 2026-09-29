/**
 * @script Pi-style outcome bands and rendered transcript scrollback in a real PTY.
 * @description Native shell results, ANSI color evidence, expand/collapse, Home/End, pages and editor preservation at three sizes.
 */
import {spawnSync} from 'node:child_process';
import {mkdtempSync,mkdirSync,writeFileSync,readFileSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {resolve,join} from 'node:path';
const root=process.cwd(),artifacts=resolve('test-results/tui-outcomes');mkdirSync(artifacts,{recursive:true});
const run=(cmd,args)=>{const r=spawnSync(cmd,args,{encoding:'utf8',timeout:15000});if(r.status!==0)throw Error(`${cmd} ${args.join(' ')}\n${r.stderr}`);return r.stdout;};
const tmux=(...args)=>run('tmux',args),sleep=ms=>new Promise(r=>setTimeout(r,ms));
const wait=async(fn,label)=>{const end=Date.now()+15000;while(Date.now()<end){if(await fn())return;await sleep(80)}throw Error('Timed out: '+label);};
const assert=(v,label)=>{if(!v)throw Error(label)};
const results=[];
for(const [width,height] of [[60,18],[100,22],[140,36]]){
 const dir=mkdtempSync(join(tmpdir(),'gi-outcomes-')),session=`gi-outcomes-${process.pid}-${width}`,pane=session+':0.0',db=join(dir,'gi.db');
 mkdirSync(join(dir,'.pi'));writeFileSync(join(dir,'.pi/settings.json'),JSON.stringify({model:'test-model',enabledModels:['test-model','bootstrap']}));
 const sql=q=>run('sqlite3',['-cmd','.timeout 5000',db,q]).trim();
 const capture=()=>tmux('capture-pane','-p','-t',pane).replace(/\s+$/,'');
 const ansi=()=>tmux('capture-pane','-e','-p','-t',pane);
 const keys=(...args)=>tmux('send-keys','-t',pane,...args),type=text=>keys('-l',text);
 const shot=name=>{const text=capture();writeFileSync(join(artifacts,`${width}x${height}-${name}.txt`),text+'\n');writeFileSync(join(artifacts,`${width}x${height}-${name}.ansi`),ansi());return text;};
 // The editor's top/bottom borders are the last two full rules; Pi's `!!`
 // command blocks draw their own rules in the transcript above.
 const bars=text=>text.split('\n').map((line,i)=>/^\s*─{10,}\s*$/.test(line)?i:-1).filter(i=>i>=0).slice(-2);
 const color=rgb=>ansi().includes(`48;2;${rgb}`);
 const idle=()=>sql('select count(*) from session_active_turns;')==='0';
 try{
  tmux('new-session','-d','-s',session,'-x',String(width),'-y',String(height),`cd '${dir}' && PATH='${root}/tests/ux/shell':"$PATH" GI_UX_QUEUE_GATES='${dir}' TERM=xterm-256color COLORTERM=truecolor '${root}/bin/gi' -tui -db '${db}' -workspace '${dir}' -model test-model 2>'${dir}/runtime.log'`);
  tmux('set-option','-t',session,'status','off');await wait(()=>capture().includes('%/'),'startup');
  const idleBars=bars(capture());
  type('UX queue gate:outcomes');keys('Enter');await wait(()=>!idle()&&color('52;56;58'),'pending tool band');
  assert(color('33;59;73'),'user message band absent');shot('pending');
  type('preserved draft');keys('Left','Left','Left');
  writeFileSync(join(dir,'outcomes'),'go');await wait(()=>idle()&&sql("select count(*) from turns where status='completed';")==='1','completion');
  await wait(()=>color('37;65;49')&&capture().includes('Queue gate streaming preview'),'successful tool result');shot('success');
  keys('Home');await wait(()=>capture().includes('UX queue gate:outcomes'),'top for spacing');
  const spaced=shot('message-spacing'),rows=spaced.split('\n').map(line=>line.replace(/[│█]\s*$/,'').trim());
  const user=rows.findIndex(line=>line==='UX queue gate:outcomes');
  const tool=rows.findIndex(line=>/^\$ /.test(line));
  assert(user>=1&&rows[user-1]===''&&rows[user+1]==='','user top/bottom padding missing');
  assert(tool>=user+3&&rows[tool-1]===''&&rows[tool+1]==='','tool separator/top/bottom padding missing');
  assert(JSON.stringify(bars(spaced))===JSON.stringify(idleBars),'message spacing changed editor/footer rows');
  keys('End');await sleep(100);
  keys('C-a','C-k');type('!!printf "failure-output\\n"; exit 7');keys('Enter');await wait(()=>capture().includes('failure-output')&&capture().includes('(exit 7)'),'exit status');shot('error');
  for(let i=0;i<8;i++){type(`!!printf 'block-${i}\\n'`);keys('Enter');await sleep(85);}
  type("!!for i in $(seq 1 60); do printf 'ROW-%02d long output for wrapped scrollback verification abcdefghijklmnopqrstuvwxyz\\n' $i; done");keys('Enter');await wait(()=>capture().includes('ROW-60'),'long collapsed output');
  type('newer unsent draft');keys('Left','Left','Left');await sleep(150);
  const collapsed=shot('collapsed');assert(collapsed.includes('more lines'),'collapsed hint missing');
  keys('C-o');await wait(()=>capture().includes('to collapse)'),'Ctrl-O expands output');shot('expanded');
  keys('Home');await wait(()=>capture().includes('UX queue gate:outcomes'),'Home reaches first message with focused editor');
  const first=shot('top');assert(first.replaceAll('▌','').includes('newer unsent draft'),'Home damaged editor');
  keys('End');await wait(()=>capture().includes('ROW-60'),'End reaches expanded tail');
  keys('PageUp');await sleep(150);const page=shot('page-up');assert(!page.includes('ROW-60'),'PageUp did not leave tail');
  keys('End');await wait(()=>capture().includes('ROW-60'),'End restores following');
  keys('C-Home');type('X');await sleep(120);assert(capture().replaceAll('▌','').includes('Xnewer unsent draft'),'Ctrl-Home no longer edits');keys('BSpace');
  keys('C-End');type('Y');await sleep(120);assert(capture().replaceAll('▌','').includes('newer unsent draftY'),'Ctrl-End no longer edits');keys('BSpace');
  keys('C-o');await wait(()=>capture().includes('more lines'),'Ctrl-O collapses');
  const final=shot('final');assert(JSON.stringify(bars(final))===JSON.stringify(idleBars),'outcomes added idle rows');
  assert(sql("select count(*) from messages where role='user';")==='1','editor navigation submitted text');
  results.push(`${width}x${height}: RGB user band, visible pending/success/error tool states; real shell outcomes; Ctrl-O collapse/expand; rendered/wrapped scrollback Home/End/PageUp, Ctrl-Home/End editor, unchanged idle rows`);
 }catch(error){try{shot('failure')}catch{};try{writeFileSync(join(artifacts,`${width}-runtime.log`),readFileSync(join(dir,'runtime.log')))}catch{};throw error;}
 finally{try{tmux('kill-session','-t',session)}catch{};rmSync(dir,{recursive:true,force:true});}
}
writeFileSync(join(artifacts,'summary.txt'),results.join('\n')+'\n');console.log(results.join('\n'));
