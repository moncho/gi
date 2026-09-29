// Installed Pi 0.87.1 agent-core mode changes while two messages are queued.
// Disposable in-memory provider; no TUI settings, durable queue, Piclaw HTTP or live model.
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import path from 'node:path';
import {pathToFileURL} from 'node:url';
const root = process.env.PICLAW_ORACLE_ROOT || '/opt/piclaw/current';
const pi = path.join(root, 'app/node_modules/@earendil-works/pi-coding-agent');
assert.equal(JSON.parse(await fs.readFile(path.join(pi, 'package.json'), 'utf8')).version, '0.87.1');
const {Agent} = await import(pathToFileURL(path.join(root, 'app/node_modules/@earendil-works/pi-agent-core/dist/agent.js')).href);
const {createAssistantMessageEventStream} = await import(pathToFileURL(path.join(root, 'app/node_modules/@earendil-works/pi-ai/dist/utils/event-stream.js')).href);
const user = text => ({role: 'user', content: [{type: 'text', text}], timestamp: Date.now()});
const response = index => ({role: 'assistant', content: [{type: 'text', text: `answer ${index}`}], api: 'openai-completions', provider: 'openai', model: 'fixture', usage: {input: 0, output: 0, cacheRead: 0, cacheWrite: 0, totalTokens: 0, cost: {input: 0, output: 0, cacheRead: 0, cacheWrite: 0, total: 0}}, stopReason: 'stop', timestamp: Date.now()});
const cases = [];
for (const [initialMode, finalMode] of [['one-at-a-time', 'all'], ['all', 'one-at-a-time']]) {
  let firstStarted;
  const started = new Promise(resolve => { firstStarted = resolve; });
  let releaseFirst;
  const released = new Promise(resolve => { releaseFirst = resolve; });
  const requests = [];
  const agent = new Agent({initialState: {model: {id: 'fixture', provider: 'openai', api: 'openai-completions'}}, steeringMode: initialMode,
    convertToLlm: async messages => messages,
    streamFn: (_model, context) => {
      const index = requests.length + 1;
      requests.push(context.messages.filter(m => m.role === 'user').map(m => m.content?.find(b => b.type === 'text')?.text));
      const stream = createAssistantMessageEventStream();
      if (index === 1) { firstStarted(); void released.then(() => stream.push({type: 'done', reason: 'stop', message: response(index)})); }
      else queueMicrotask(() => stream.push({type: 'done', reason: 'stop', message: response(index)}));
      return stream;
    },
  });
  const run = agent.prompt('first request');
  await started;
  agent.steer(user('steer one'));
  agent.steer(user('steer two'));
  agent.steeringMode = finalMode;
  releaseFirst();
  await run;
  assert.deepEqual(requests, finalMode === 'all'
    ? [['first request'], ['first request', 'steer one', 'steer two']]
    : [['first request'], ['first request', 'steer one'], ['first request', 'steer one', 'steer two']]);
  assert.equal(agent.hasQueuedMessages(), false);
  cases.push({initialMode, finalMode, requests});
}
console.log(JSON.stringify({scope: 'Installed Pi 0.87.1 agent-core mutable queue-wide mode; no TUI settings, Piclaw HTTP, durable store or live model', cases}, null, 2));
