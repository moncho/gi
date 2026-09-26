import {test, expect} from 'bun:test';
import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {generateMessages} from '@cucumber/gherkin';
import {IdGenerator, SourceMediaType} from '@cucumber/messages';

const root=resolve(import.meta.dir,'../../..');
const file=resolve(root,'tests/ux/features/oracle/piclaw-3.2.4-basic-interactions.feature');
const text=readFileSync(file,'utf8');
const messages=generateMessages(text,file,SourceMediaType.TEXT_X_CUCUMBER_GHERKIN_PLAIN,{newId:IdGenerator.incrementing(),includeGherkinDocument:true,includePickles:true});

test('versioned oracle scenarios are parseable, isolated and intentionally outside Gi parity',()=>{
 expect(messages.filter(m=>m.parseError)).toEqual([]);
 const feature=messages.find(m=>m.gherkinDocument)?.gherkinDocument?.feature;
 expect(feature?.tags?.map(t=>t.name)).toEqual(['@oracle-only','@piclaw-3.2.4','@not-gi-parity']);
 const pickles=messages.filter(m=>m.pickle).map(m=>m.pickle);
 expect(pickles.map(p=>p.name)).toEqual([
  "A first Return sends the selected chat's captured text once",
  'A slash Quick Action replaces an existing unsent composer draft',
  'A listed skill command uses the same draft-replacing prefill path',
  'Return to editor replaces the existing draft before removing the queued row',
  'A safe fenced SVG is rendered as an isolated image by default',
 ]);
 expect(pickles.every(p=>p.tags.some(t=>t.name==='@oracle-only')&&p.tags.some(t=>t.name==='@not-gi-parity'))).toBe(true);
 expect(pickles.every(p=>p.steps.length>=5)).toBe(true);
 const normal=readFileSync(resolve(root,'tests/ux/support/catalogue.mjs'),'utf8');
 expect(normal).not.toContain('features/oracle');
});

test('Gi first-send scenarios remain separately tagged and parseable',()=>{
 const path=resolve(root,'tests/features/sessions/gi-basic-send.feature');
 const text=readFileSync(path,'utf8');
 const envelopes=generateMessages(text,path,SourceMediaType.TEXT_X_CUCUMBER_GHERKIN_PLAIN,{newId:IdGenerator.incrementing(),includeGherkinDocument:true,includePickles:true});
 expect(envelopes.filter(e=>e.parseError)).toEqual([]);
 const pickles=envelopes.filter(e=>e.pickle).map(e=>e.pickle);
 expect(pickles.map(p=>p.tags.find(t=>t.name.startsWith('@gi-basic-'))?.name)).toEqual(['@gi-basic-001','@gi-basic-002']);
 expect(pickles.every(p=>p.tags.some(t=>t.name==='@not-frozen-parity'))).toBe(true);
});
