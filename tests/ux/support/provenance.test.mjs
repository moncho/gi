import { test, expect } from 'bun:test';
import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { resolve } from 'node:path';
import {verifySources, uxRoot} from './provenance.mjs';

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



test('Quick Actions provenance includes unchanged pinned sources and the exact CSS region',()=>{
 const root=uxRoot;const manifest=JSON.parse(readFileSync(resolve(root,'web/upstream/piclaw-quick-actions-70d33bc93.json'),'utf8'));
 expect(manifest.commit).toBe('70d33bc93ab540845bbcf5f80503ca8125c71594');expect(manifest.files).toHaveLength(3);
 expect(manifest.css.start).toBe('.timeline-quick-actions-overlay {');expect(manifest.css.endBefore).toBe('.compose-submit-spinner {');
});
