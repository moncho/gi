import {test,expect} from 'bun:test';
import {readFileSync} from 'node:fs';
import {messageReferenceLabel,messageReferenceLabels} from '../../../web/src/gi-message-reference-label';
import {patchMessageReferenceLabels} from '../../../scripts/patch-message-reference-labels.mjs';
test('reference labels prefer safe durable rows without changing canonical refs',()=>{
 const id='msg_1790500000000000000';
 const labels=messageReferenceLabels([{id,display_row_id:42},{id:'bad',display_row_id:9007199254740992}]);
 expect(messageReferenceLabel(id,labels)).toBe('42');expect(messageReferenceLabel(id)).toBe('msg_…000000');
 expect(messageReferenceLabel('legacy')).toBe('legacy');expect(messageReferenceLabel('toString',labels)).toBe('toString');expect(labels.bad).toBeUndefined();
});
test('guarded adapter changes only both pill labels, preserving full tooltip/removal/submission IDs',()=>{
 const path='web/src/components/compose-box.ts',source=readFileSync(path,'utf8'),patched=patchMessageReferenceLabels(source);
 expect(patched.match(/label=\$\{'msg:' \+ messageReferenceLabel/g)).toHaveLength(2);
 expect(patched).toContain("title=${'Message reference: ' + id}");expect(patched).toContain('onRemoveMessageRef?.(id)');
 expect(patched).toContain('capturedMessageRefs.map((id) => `- message:${id}`)');
 expect(readFileSync(path,'utf8')).toBe(source);
 for(const bad of ['',source+source,patched])expect(()=>patchMessageReferenceLabels(bad)).toThrow();
});
