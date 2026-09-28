import { test, expect } from 'bun:test';
import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { resolve } from 'node:path';
import { loadCorpus, verifySources, mappedIds, sharedMappedIds, uxRoot } from './catalogue.mjs';

test('historical Piclaw and Vibes/Tau snapshots retain their exact hashes', () => {
  expect(() => verifySources()).not.toThrow();
});

test('imported Piclaw component sources retain their upstream hashes', () => {
  const root = uxRoot;
  for (const name of ['piclaw-menu-70d33bc93.json', 'piclaw-session-picker-70d33bc93.json', 'piclaw-quick-actions-70d33bc93.json']) {
    const manifest = JSON.parse(readFileSync(resolve(root, 'web/upstream', name), 'utf8'));
    for (const file of [...manifest.files,...(manifest.css?[manifest.css]:[])]) {
      const hash = createHash('sha256').update(readFileSync(resolve(root, file.destination))).digest('hex');
      expect(hash).toBe(file.sha256);
    }
  }
});

test('all active Piclaw 3.2.4 scenarios and outline examples are inventoried, not just mapped tests', () => {
  const cases = loadCorpus();
  expect(cases).toHaveLength(262);
  expect(cases.filter(row => row.id === '@ux-reconnect-004').map(row => row.steps[1])).toEqual(['the composer is draft-filled', 'the composer is clean']);
  expect(new Set(cases.map(item => item.id)).size).toBe(241);
  expect(mappedIds.has('@ux-original-027')).toBe(false);
  for (const row of cases.filter(row => row.id.startsWith('@ux-chat-lifecycle-'))) expect(mappedIds.has(row.id)).toBe(false);
  for (const id of mappedIds) expect(cases.some(item => item.id === id)).toBe(true);
  expect(cases.find(row => row.id === '@ux-compaction-006')?.name).toBe('Check model context compatibility before switching');
  expect(cases.find(row => row.id === '@ux-compaction-007')?.name).toBe('Refresh model information after an accepted switch');
  expect(cases.find(row => row.id === '@ux-context-001')?.name).toBe('Show supplied usage in the context tooltip');
  expect(cases.find(row => row.id === '@ux-context-005')?.name).toBe('Apply the coded usage warning colours');
  expect(cases.find(row => row.id === '@ux-context-003')?.name).toBe('Offer compaction only when a callback exists');
  expect(cases.find(row => row.id === '@ux-reconnect-002')?.name).toBe('Refresh authoritative chat state after reconnect');
  expect(cases.find(row => row.id === '@ux-reconnect-004')?.name).toBe('Show version drift without automatically reloading');
  expect(cases.find(row => row.id === '@ux-reconnect-003')?.name).toBe('Avoid replacing an active search with main-timeline refresh');
  expect(cases.find(row => row.id === '@ux-reconnect-005')?.name).toBe('Avoid duplicate initial refresh after recent chat activation');
  const shared = loadCorpus('shared');
  expect(shared).toHaveLength(42);
  for (const id of sharedMappedIds) expect(shared.some(item => item.id === id)).toBe(true);
  expect(shared.find(row => row.id === '@shared-28')?.name).toBe('Return a queued item by replacing the Classic editor draft');
  expect(shared.find(row => row.id === '@shared-30')?.name).toBe('Let the backend steer or send a queued item after the stream ends');
});

test('Quick Actions provenance includes unchanged pinned sources and the exact CSS region',()=>{
 const root=uxRoot;const manifest=JSON.parse(readFileSync(resolve(root,'web/upstream/piclaw-quick-actions-70d33bc93.json'),'utf8'));
 expect(manifest.commit).toBe('70d33bc93ab540845bbcf5f80503ca8125c71594');expect(manifest.files).toHaveLength(3);
 expect(manifest.css.start).toBe('.timeline-quick-actions-overlay {');expect(manifest.css.endBefore).toBe('.compose-submit-spinner {');
});
