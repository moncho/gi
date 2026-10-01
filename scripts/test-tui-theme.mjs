/** Real controlling-PTY colour negotiation, using Bun and util-linux script. */
import {mkdirSync, mkdtempSync, writeFileSync, rmSync} from 'node:fs';
import {resolve, join} from 'node:path';
const bin = process.env.GI_THEME_TEST_BIN || resolve('bin/gi-theme-test');
const out = resolve('test-results/tui-theme');
mkdirSync(out, {recursive:true});
const plain = s => s.replace(/\x1b\[[0-?]*[ -/]*[@-~]/g, '').replace(/\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)/g, '');
const quote = s => `'${s.replaceAll("'", "'\\''")}'`;
const sleep = ms => Bun.sleep(ms);
const light = '\x1b]10;rgb:0000/0000/0000\x07\x1b]11;rgb:ffff/ffff/ffff\x1b\\';
const dark = '\x1b]10;#ffffff\x07\x1b]11;#101010\x07';
const da = '\x1b[?62;22c';
const cases = [
 {name:'light-osc', reply:light+da, want:'light', ui:true},
 {name:'dark-osc', reply:dark+da, want:'dark', ui:true},
 {name:'fragmented-light', reply:light+da, fragmented:true, want:'light'},
 {name:'scheme-light', reply:'\x1b[?997;2n'+da, want:'light'},
 {name:'bg-overrides-scheme', reply:dark+'\x1b[?997;2n'+da, fgbg:'0;15', want:'dark'},
 {name:'colorfgbg-light', reply:da, fgbg:'0;15', want:'light'},
 {name:'silent-fallback', reply:'', fgbg:'15;0', want:'dark'},
 {name:'malformed-fallback', reply:'\x1b]11;rgb:gg/ff/ff\x07'+da, fgbg:'0;15', want:'light'},
 {name:'explicit-dark', reply:light+da, setting:'dark', want:'dark'},
 {name:'auto-pair-light', reply:light+da, setting:'light/dark', want:'light'},
 {name:'auto-pair-dark', reply:dark+da, setting:'light/dark', want:'dark'},
 {name:'query-disabled', reply:'', disabled:true, fgbg:'0;15', want:'light'},
];
const results=[];
for (const tc of cases) {
 const dir=mkdtempSync(resolve('.gi-theme-pty-'));
 const env={...process.env, HOME:dir, TERM:'xterm-256color', COLORTERM:'truecolor', PI_TRUE_COLOR:'1', COLORFGBG:tc.fgbg||'', GI_THEME_PTY_DIR:dir, GI_THEME_SETTING:tc.setting||'', GI_THEME_PROBE_ONLY:tc.ui?'':'1', GI_TUI_NO_THEME_QUERY:tc.disabled?'1':''};
 const child=Bun.spawn(['script','-q','-e','-f','-c', `stty rows 24 cols 100; ${quote(bin)} -test.run '^TestTerminalThemePTYFixture$' -test.timeout 20s`, '/dev/null'], {env, stdin:'pipe', stdout:'pipe', stderr:'pipe'});
 let raw='', answered=false, readError=null;
 const reading=(async()=>{for await(const bytes of child.stdout){ raw+=new TextDecoder().decode(bytes); if(!answered && raw.includes('\x1b[c')){answered=true; if(tc.reply){if(tc.fragmented){for(const c of tc.reply){child.stdin.write(c); child.stdin.flush(); await sleep(1);}}else{child.stdin.write(tc.reply);child.stdin.flush();}}}}})().catch(e=>{readError=e;});
 const wait=async(fn,label)=>{const end=Date.now()+12000;while(Date.now()<end){if(fn())return;await sleep(20);}throw Error(`${tc.name}: timeout ${label}`);};
 try {
  await wait(()=>raw.includes('THEME-RESULT:'),'startup');
  const m=raw.match(/THEME-RESULT:(\w+) elapsed=(\d+)/);
  if(!m || m[1]!==tc.want)throw Error(`${tc.name}: wrong theme ${m?.[1]}`);
  if(Number(m[2])>1000)throw Error(`${tc.name}: unbounded startup ${m[2]}ms`);
  if(tc.disabled && answered)throw Error('query-disable ignored');
  if(!tc.disabled && !answered)throw Error('query was not sent');
  if(tc.ui){
   await wait(()=>raw.includes('%/'),'editor ready');
   child.stdin.write('/settings\r');child.stdin.flush();
   await wait(()=>plain(raw).includes('settings: peering'),'settings output');
   child.stdin.write('\x1b[H');child.stdin.flush();
   await wait(()=>plain(raw).includes(`theme: ${tc.want}`),'settings theme');
   const rgb=tc.want==='light'?'59;63;65':'222;224;225';
   if(!raw.includes('38;2;'+rgb))throw Error(`${tc.name}: text RGB not emitted`);
   child.stdin.write('\x03');child.stdin.flush();await sleep(50);child.stdin.write('\x03');child.stdin.flush();
  }
  const code=await Promise.race([child.exited,sleep(3000).then(()=>{throw Error(`${tc.name}: exit timeout`);})]);
  await reading;
  if(code!==0||readError)throw Error(`${tc.name}: exit ${code}, ${readError}`);
  writeFileSync(join(out,tc.name+'.ansi'),raw);
  results.push({name:tc.name,theme:tc.want,elapsedMs:Number(m[2]),ui:!!tc.ui});
 } catch(e) {writeFileSync(join(out,tc.name+'-failed.ansi'),raw);throw e;}
 finally {child.kill();child.stdin.end();await child.exited;rmSync(dir,{recursive:true,force:true});}
}
writeFileSync(join(out,'results.json'),JSON.stringify(results,null,2));
console.log(`PASS: ${results.length} real PTY theme scenarios (including light/dark native UI)`);
