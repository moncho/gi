// Installed Pi 0.87.1 agent loop: steering arrives during async next-turn preparation.
// Disposable provider and preparation gate; not a real compaction or TUI fixture.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import {pathToFileURL} from 'node:url';
const root = process.env.PICLAW_ORACLE_ROOT || '/opt/piclaw/current';
const pi = path.join(root, 'app/node_modules/@earendil-works/pi-coding-agent');
assert.equal(JSON.parse(await fs.readFile(path.join(pi, 'package.json'), 'utf8')).version, '0.87.1');
const {runAgentLoop} = await import(pathToFileURL(path.join(root, 'app/node_modules/@earendil-works/pi-agent-core/dist/agent-loop.js')).href);
const {createAssistantMessageEventStream} = await import(pathToFileURL(path.join(root, 'app/node_modules/@earendil-works/pi-ai/dist/utils/event-stream.js')).href);
const user = text => ({role: 'user', content: [{type: 'text', text}], timestamp: Date.now()});
const answer = index => ({role: 'assistant', content: [{type: 'text', text: `answer ${index}`}], api: 'openai-completions', provider: 'openai', model: 'fixture', usage: {input: 0, output: 0, cacheRead: 0, cacheWrite: 0, totalTokens: 0, cost: {input: 0, output: 0, cacheRead: 0, cacheWrite: 0, total: 0}}, stopReason: 'stop', timestamp: Date.now()});
let preparationStarted, releasePreparation;
const started = new Promise(resolve => { preparationStarted = resolve; });
const released = new Promise(resolve => { releasePreparation = resolve; });
const queue = [];
let polls = 0;
const requests = [];
const config = {model: {id: 'fixture', provider: 'openai', api: 'openai-completions'}, convertToLlm: async messages => messages,
  getSteeringMessages: async () => { polls++; return queue.splice(0, 1); },
  prepareNextTurn: async () => { preparationStarted(); await released; },
};
const streamFn = (_model, context) => {
  const index = requests.length + 1;
  requests.push(context.messages.filter(m => m.role === 'user').map(m => m.content?.find(b => b.type === 'text')?.text));
  const stream = createAssistantMessageEventStream();
  if (index === 1) queue.push(user('steer one'));
  queueMicrotask(() => stream.push({type: 'done', reason: 'stop', message: answer(index)}));
  return stream;
};
const run = runAgentLoop([user('first request')], {systemPrompt: 'fixture', messages: [], tools: []}, config, () => {}, undefined, streamFn);
await started;
queue.push(user('steer two'));
releasePreparation();
await run;
assert.deepEqual(requests, [['first request'], ['first request', 'steer one'], ['first request', 'steer one', 'steer two']]);
assert.equal(queue.length, 0);
console.log(JSON.stringify({scope: 'Installed Pi 0.87.1 agent-loop async preparation; not real compaction, TUI, live provider or durable queue', requests, polls}, null, 2));
