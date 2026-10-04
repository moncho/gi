import {test,expect} from 'bun:test';
import {readFileSync} from 'node:fs';
import {patchPinnedStatusResize} from '../../../scripts/patch-pinned-status-resize.mjs';
const path='web/piclaw-status-3.2.5/components/status.ts';
const source=readFileSync(path,'utf8');
test('pinned renderer keeps resize safety via guarded adapter, with no source edits',()=>{
 const patched=patchPinnedStatusResize(source);
 expect(patched).toContain('previewOverflow.refs[panelKey]?.(node)');
 expect(patched).toContain('previewOverflow.overflow[panelKey]');
 expect(patched).toContain("panelKey === 'tool-output'");
 expect(readFileSync(path,'utf8')).toBe(source);
 for(const bad of ['',source+source,patched,source.replace('const measuredOverflow','const renamedOverflow')])expect(()=>patchPinnedStatusResize(bad)).toThrow('anchor changed');
});
