import {test,expect} from 'bun:test';
import {readFileSync} from 'node:fs';
import {loadGiDeviations,giDeviationFile,attachGiDeviation} from './gi-deviations.mjs';
import {loadCorpus,mappedIds,sharedMappedIds} from './catalogue.mjs';

test('Gi-specific original behaviours parse and cannot count as Piclaw parity', async () => {
  const rows = loadGiDeviations();
  expect(rows.map(row => row.id)).toEqual(['@gi-ux-001','@gi-ux-002','@gi-ux-003','@gi-ux-004','@gi-ux-005']);
  for (const row of rows) {
    expect(row.steps.length).toBeGreaterThan(2);
    expect(row.tags).toContain('@gi-specific');
    expect(mappedIds.has(row.id)).toBe(false);
    expect(sharedMappedIds.has(row.id)).toBe(false);
  }
  const specFiles = ['quick-actions','skills','rendering','queue-return','queue-steer'];
  for (const [index, file] of specFiles.entries()) {
    const spec = readFileSync(new URL(`../${file}.spec.mjs`, import.meta.url), 'utf8');
    expect(spec).toContain(`attachGiDeviation(info,'@gi-ux-00${index + 1}')`);
  }
  for (const id of ['@ux-original-007','@ux-original-008','@ux-original-029']) expect(mappedIds.has(id)).toBe(false);
  for (const id of ['@shared-17','@shared-28','@shared-30']) expect(sharedMappedIds.has(id)).toBe(false);
  expect(loadCorpus()).toHaveLength(257);
  expect(loadCorpus('shared')).toHaveLength(42);
  const attachments = [];
  const info = {attach: async (name, attachment) => attachments.push({name, ...attachment})};
  await attachGiDeviation(info, '@gi-ux-004');
  expect(attachments[0].name).toBe('gi-specific-gherkin');
  expect(attachments[0].body).toContain(`${giDeviationFile} @gi-ux-004`);
  expect(attachGiDeviation(info, '@gi-ux-999')).rejects.toThrow('Missing Gi-specific UX scenario');
});
