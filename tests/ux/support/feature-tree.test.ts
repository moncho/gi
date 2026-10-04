import {test,expect} from 'bun:test';
import {execFileSync} from 'node:child_process';
import {readFileSync} from 'node:fs';
import {generateMessages} from '@cucumber/gherkin';
import {IdGenerator,SourceMediaType} from '@cucumber/messages';

test('every tracked Gherkin file is grouped in the single feature tree and parses',()=>{
 // Shared browser Gherkin lives only in the fixtures-vibes submodule.
 const paths=execFileSync('git',['ls-files','-z','--','*.feature','*.gherkin'],{encoding:'utf8'}).split('\0').filter(path=>path&&!path.startsWith('references/'));
 expect(paths).toHaveLength(21);
 expect(paths.every(path=>path.startsWith('features/')&&path.endsWith('.feature'))).toBe(true);
 expect(paths.some(path=>path.startsWith('features/ux/'))).toBe(false);
 expect(paths.filter(path=>path.startsWith('features/gi/'))).toHaveLength(11);
 const ledger=readFileSync('features/VALIDATION.md','utf8');
 const listed=[...ledger.matchAll(/^\| \[([^\]]+\.(?:feature|gherkin))\]/gm)].map(match=>match[1]);
 expect(listed).toEqual([...paths].sort());
 for(const path of paths){
  const messages=generateMessages(readFileSync(path,'utf8'),path,SourceMediaType.TEXT_X_CUCUMBER_GHERKIN_PLAIN,{newId:IdGenerator.incrementing(),includeGherkinDocument:true,includePickles:true});
  expect(messages.filter(message=>message.parseError)).toEqual([]);
  expect(messages.some(message=>message.gherkinDocument?.feature)).toBe(true);
 }
});
