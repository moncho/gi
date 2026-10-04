import {test,expect} from 'bun:test';
import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {piclawStatusAdapter,verifyPiclawStatus} from '../../../scripts/piclaw-status-adapter.mjs';
test('pinned installed status sources have provenance and replace only status imports',()=>{
 expect(verifyPiclawStatus()).toBe(resolve('web/piclaw-status-3.2.5'));
 const handlers:any[]=[];piclawStatusAdapter().setup({onLoad:()=>{},onResolve:(filter:any,handler:any)=>handlers.push(handler)});
 expect(handlers[0]({importer:resolve('web/src/app.ts'),path:'./components/status.js'}).path).toBe(resolve('web/piclaw-status-3.2.5/components/status.ts'));
 expect(handlers[0]({importer:resolve('web/src/app.ts'),path:'./gi-status.js'})).toBeUndefined();
 expect(handlers[1]({importer:resolve('web/piclaw-status-3.2.5/components/status.ts'),path:'../api.js'}).path).toBe(resolve('web/src/api.ts'));
 expect(handlers[1]({importer:resolve('web/piclaw-status-3.2.5/components/status.ts'),path:'../vendor/preact-htm.js'}).path).toBe(resolve('web/src/vendor/preact-htm.js'));
 // The old supplied component is retained unchanged, not rewritten in place.
 expect(readFileSync('web/src/components/status.ts','utf8')).not.toContain('TOOL_OUTPUT_MAX_LINES');
});
