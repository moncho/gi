import {test,expect} from 'bun:test';
import {readFileSync} from 'node:fs';
import {patchModelThinking} from '../../../scripts/patch-model-thinking.mjs';
import {patchModelPanel} from '../../../scripts/patch-model-panel.mjs';
import {patchModelPicker} from '../../../scripts/patch-model-picker.mjs';
import {patchPickerGeometry} from '../../../scripts/patch-picker-geometry.mjs';
import {patchComposePopupKeys} from '../../../scripts/patch-popup-keys.mjs';
import {modelPickerKey} from '../../../web/src/gi-model-picker';
const source=readFileSync('web/src/components/compose-box.ts','utf8');
const base=patchModelPanel(patchModelPicker(patchPickerGeometry(patchComposePopupKeys(source))));
test('thinking adapter uses validated token/model path and preserves guarded supplied source',()=>{
 const result=patchModelThinking(base);
 expect(result).toContain('selectAgentThinking(currentChatJid, activeModel, requested, thinkingState.thinking_token)');
 expect(result).toContain('thinkingState.current !== activeModel');expect(result).toContain('modelMutationRef.current');
 expect(result).toContain('Provider default');expect(result).not.toContain('Thinking level (read-only)');
 expect(readFileSync('web/src/components/compose-box.ts','utf8')).toBe(source);
 for(const bad of ['',base+base,result])expect(()=>patchModelThinking(bad)).toThrow('anchor changed');
});
test('native thinking select keyboard does not activate or navigate model rows',()=>{
 for(const key of ['ArrowDown','ArrowUp','Enter','Home','End','PageDown','PageUp']){
  const event={key,target:{closest:(s:string)=>s==='select'?{}:null}} as any;
  expect(modelPickerKey(event,[{label:'model'}],0,{value:'',updatedAt:0})).toBeNull();
 }
});
