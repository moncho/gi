// Golden values for internal/codemode tests, produced by Pi's own codemode
// extension: full and budgeted codemode descriptions, and the mode "on"
// descriptions of declared tools. Run with bun after updating Pi; writes
// internal/codemode/testdata/pi-description.json.
import { writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { existsSync as piModulesExist } from "node:fs";
// Pi's packages: PI_NODE_MODULES, the global bun install, or the reference install.
const PI_MODULES = [process.env.PI_NODE_MODULES, `${homedir()}/.bun/install/global/node_modules`, "/workspace/.cache/pi-ref/node_modules"].find((dir) => dir && piModulesExist(`${dir}/@earendil-works/pi-coding-agent`));
const P = `${PI_MODULES}/@earendil-works`;
const tool = await import(`${P}/pi-coding-agent/dist/extensions/codemode/tool.js`);
const mcp = await import(`${P}/pi-coding-agent/dist/extensions/mcp/tools.js`);
const tools = [
  { name: 'read', description: 'Read a file.', parameters: { type: 'object', properties: { path: { type: 'string', description: 'File path' }, offset: { type: 'number' } }, required: ['path'] } },
  { name: 'my-tool', description: 'Does things\nsecond line', parameters: { type: 'object', properties: { items: { type: 'array', items: { type: 'string' } }, mode: { enum: ['a', 'b'] } } } },
  { name: 'bash', description: 'Run a command.', parameters: { type: 'object', properties: { command: { type: 'string' } }, required: ['command'] }, outputSchema: { type: 'object', properties: { output: { type: 'string' }, exit_code: { type: 'number' }, full_output_path: { type: 'string' } }, required: ['output', 'exit_code'] } },
  { name: 'mcp__hub__search_code', description: 'Search code', parameters: { type: 'object', properties: { text: { type: 'string' } } }, outputSchema: mcp.createMcpResultSchema() },
  { name: 'mcp__hub__upper', description: 'Uppercase text', parameters: { type: 'object', properties: { text: { type: 'string' } } }, outputSchema: mcp.createMcpResultSchema({ type: 'object', properties: { value: { type: 'string' } } }) },
  { name: 'mcp__docs__lookup', description: 'Look up docs', parameters: { type: 'object', properties: {} }, outputSchema: mcp.createMcpResultSchema() },
];
const namespaces = new Map([
  ['mcp__hub__search_code', { name: 'mcp__hub', description: 'Code hosting' }],
  ['mcp__hub__upper', { name: 'mcp__hub', description: 'Code hosting' }],
  ['mcp__docs__lookup', { name: 'mcp__docs' }],
]);
const full = tool.createCodemodeDescription(tools, { models: false, namespaces });
const budget = tool.createCodemodeDescription(tools, { models: false, namespaces, inlineBudget: 120, deferred: new Set(['mcp__docs__lookup']) });
// Mode "on": declared callable tools say how scripts call them.
const definition = tool.createCodemodeToolDefinition({ models: false });
const loadout = definition.prepareLoadout({
  declared: tools, callable: tools,
  getExposure: (name) => (name.startsWith('mcp__') ? 'codemode' : 'direct'),
  getNamespace: (name) => namespaces.get(name),
});
const scriptCalls = Object.fromEntries(tools.map((t) => [t.name, loadout.descriptions[t.name]]));
writeFileSync('internal/codemode/testdata/pi-description.json', JSON.stringify({
  tools: tools.map((t) => ({ name: t.name, description: t.description, inputSchema: t.parameters, outputSchema: t.outputSchema ?? { type: 'string' } })),
  full, budget, scriptCalls,
}) + '\n');
console.log('wrote internal/codemode/testdata/pi-description.json');
