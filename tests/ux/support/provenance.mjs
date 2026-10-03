// Frozen fixture provenance only. Shared scenario accounting lives in fixtures-vibes.
import {readFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {resolve, join} from 'node:path';
export const uxRoot = resolve(import.meta.dir, '../../..');
const sha256 = bytes => createHash('sha256').update(bytes).digest('hex');
export function verifySources() {
  const lines = readFileSync(join(uxRoot, 'features/ux/upstream/SHA256SUMS'), 'utf8').trim().split('\n');
  for (const line of lines) {
    const [, hash, source] = line.match(/^([a-f0-9]{64})\s+(.+)$/) || [];
    if (!hash || !source.startsWith('tests/e2e/features/classic/')) throw new Error(`Invalid source manifest line: ${line}`);
    const snapshot = join(uxRoot, 'features/ux/upstream/classic-snapshot', source.slice('tests/e2e/features/classic/'.length).replace(/\.feature$/, '.gherkin'));
    if (sha256(readFileSync(snapshot)) !== hash) throw new Error(`Frozen Piclaw 70d33bc snapshot changed: ${snapshot}`);
  }
  const sharedHash = sha256(readFileSync(join(uxRoot, 'features/ux/upstream/shared-canonical-ux.gherkin')));
  if (sharedHash !== 'a08a623880c6f327bc051edc51bb2bbff2959aed86421b5227e61d5a92fc2441') {
    throw new Error('Shared Vibes/Tau snapshot changed');
  }
}
