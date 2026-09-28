import {test,expect} from 'bun:test';
import {readFileSync} from 'node:fs';
import {patchComposeEscape} from '../../../scripts/patch-compose-escape.mjs';
import {patchComposeCommands} from '../../../scripts/patch-compose-commands.mjs';
import {patchComposePopupKeys} from '../../../scripts/patch-popup-keys.mjs';
const source=readFileSync('web/src/components/compose-box.ts','utf8');

test('Escape adapter preserves upstream bytes and earlier keyboard ownership',()=>{
 const output=patchComposeCommands(patchComposePopupKeys(patchComposeEscape(source)));
 expect(readFileSync('web/src/components/compose-box.ts','utf8')).toBe(source);
 expect(output).toContain("if (showModelPopup || showSessionPopup || showSlash || showMention) return;");
 expect(output).toContain('e.stopPropagation();\n            textareaRef.current?.blur();');
 const handler=output.slice(output.indexOf('const handleKeyDown = (e) => {'),output.indexOf('const addMediaFiles = (files) => {'));
 expect(handler.indexOf('if (declineComposeKey(e)) return;')).toBeLessThan(handler.indexOf('textareaRef.current?.blur();'));
 expect(handler.indexOf('onExitSearch?.();')).toBeLessThan(handler.indexOf('textareaRef.current?.blur();'));
 expect(handler.indexOf('setShowSlash(false);')).toBeLessThan(handler.indexOf('textareaRef.current?.blur();'));
 for(const changed of ['',source+source,patchComposeEscape(source)])expect(()=>patchComposeEscape(changed)).toThrow('anchor changed');
});
