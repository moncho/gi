// Installed Pi 0.87.1 agent-core abort with steering queued during an in-flight response.
// Disposable stream, no TUI editor restoration, Piclaw web abort, or durable store.
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
const response = (reason, index) => ({role: 'assistant', content: [{type: 'text', text: reason === 'stop' ? `answer ${index}` : ''}], api: 'openai-completions', provider: 'openai', model: 'fixture', usage: {input: 0, output: 0, cacheRead: 0, cacheWrite: 0, totalTokens: 0, cost: {input: 0, output: 0, cacheRead: 0, cacheWrite: 0, total: 0}}, stopReason: reason, timestamp: Date.now()});
let releaseFirst;
const firstStarted = new Promise(resolve => { releaseFirst = resolve; });
const requests = [];
const agent = new Agent({
  initialState: {model: {id: 'fixture', provider: 'openai', api: 'openai-completions'}},
  convertToLlm: async messages => messages,
  streamFn: (_model, context, {signal}) => {
    const index = requests.length + 1;
    requests.push(context.messages.filter(m => m.role === 'user').map(m => m.content?.find(b => b.type === 'text')?.text));
    const stream = createAssistantMessageEventStream();
    if (index === 1) {
      signal.addEventListener('abort', () => stream.push({type: 'done', reason: 'stop', message: response('aborted', index)}), {once: true});
      releaseFirst();
    } else {
      queueMicrotask(() => stream.push({type: 'done', reason: 'stop', message: response('stop', index)}));
    }
    return stream;
  },
});
const run = agent.prompt('first request');
await firstStarted;
agent.steer(user('steer one'));
agent.steer(user('steer two'));
assert.equal(agent.hasQueuedMessages(), true);
agent.abort();
await run;
assert.deepEqual(requests, [['first request']]);
assert.equal(agent.state.messages.at(-1)?.stopReason, 'aborted');
assert.deepEqual(agent.state.messages.filter(m => m.role === 'user').map(m => m.content[0].text), ['first request']);
assert.deepEqual(agent.peekQueuedMessages().map(m => m.content[0].text), ['steer one']);
assert.equal(agent.hasQueuedMessages(), true);
const retainedAfterAbort = agent.peekQueuedMessages().map(m => m.content[0].text);
await agent.continue();
assert.deepEqual(requests, [
  ['first request'],
  ['first request', 'steer one'],
  ['first request', 'steer one', 'steer two'],
]);
assert.equal(agent.hasQueuedMessages(), false);
assert.deepEqual(agent.state.messages.filter(m => m.role === 'user').map(m => m.content[0].text), ['first request', 'steer one', 'steer two']);
console.log(JSON.stringify({scope: 'Installed Pi 0.87.1 agent-core fixture abort; no TUI, live provider, Piclaw HTTP or durable queue', retainedAfterAbort, requests}, null, 2));
