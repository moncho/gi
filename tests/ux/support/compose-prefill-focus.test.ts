import {test,expect} from 'bun:test';
import {readFileSync} from 'node:fs';
import {patchComposePrefillFocus} from '../../../scripts/patch-compose-prefill-focus.mjs';
import {patchUploadCancel} from '../../../scripts/patch-upload-cancel.mjs';
import {patchModelPicker} from '../../../scripts/patch-model-picker.mjs';
import {patchComposePopupKeys} from '../../../scripts/patch-popup-keys.mjs';

test('Quick Action prefill keeps exact command and fences delayed focus',()=>{
 const source=patchUploadCancel(patchModelPicker(patchComposePopupKeys(readFileSync('web/src/components/compose-box.ts','utf8'))));
 const patched=patchComposePrefillFocus(source);
 expect(patched).not.toContain('skillPrefill');
 expect(patched).toContain('if (!mountedRef.current || document.querySelector(\'.settings-dialog[aria-modal="true"]\')) return;');
 expect(patched).toContain('setContent(resolved.text);');
 expect(readFileSync('web/src/components/compose-box.ts','utf8')).not.toContain('skillPrefill');
 expect(()=>patchComposePrefillFocus(source.replace('updateMentionAutocomplete(resolved.text);','changed'))).toThrow('anchor changed');
 expect(()=>patchComposePrefillFocus(patched)).toThrow('anchor changed');
});
