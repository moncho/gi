import {test,expect} from 'bun:test';
import {execFileSync} from 'node:child_process';
import {readFileSync} from 'node:fs';
import {generateMessages} from '@cucumber/gherkin';
import {IdGenerator,SourceMediaType} from '@cucumber/messages';
import {verifySources} from './catalogue.mjs';

test('every tracked Gherkin file is grouped in the single feature tree and parses',()=>{
 const paths=execFileSync('git',['ls-files','-z','--','*.feature','*.gherkin'],{encoding:'utf8'}).split('\0').filter(Boolean);
 expect(paths).toHaveLength(77);
 expect(paths.every(path=>path.startsWith('features/'))).toBe(true);
 const active=paths.filter(path=>path.endsWith('.feature'));
 const snapshots=paths.filter(path=>path.endsWith('.gherkin'));
 expect(active).toHaveLength(52);expect(snapshots).toHaveLength(25);
 expect(snapshots.every(path=>path.startsWith('features/ux/upstream/'))).toBe(true);
 expect(active.filter(path=>path.startsWith('features/ux/classic/'))).toHaveLength(25);
 expect(active.filter(path=>path.startsWith('features/gi/'))).toHaveLength(11);
 const ledger=readFileSync('features/VALIDATION.md','utf8');
 const listed=[...ledger.matchAll(/^\| \[([^\]]+\.(?:feature|gherkin))\]/gm)].map(match=>match[1]);
 expect(listed).toEqual([...paths].sort());
 for(const path of paths){
  const messages=generateMessages(readFileSync(path,'utf8'),path,SourceMediaType.TEXT_X_CUCUMBER_GHERKIN_PLAIN,{newId:IdGenerator.incrementing(),includeGherkinDocument:true,includePickles:true});
  expect(messages.filter(message=>message.parseError)).toEqual([]);
  expect(messages.some(message=>message.gherkinDocument?.feature)).toBe(true);
 }
 verifySources();
});
