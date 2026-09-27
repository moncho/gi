import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { loadCorpus, mappedIds, sharedMappedIds } from '../tests/ux/support/catalogue.mjs';

const classic = loadCorpus();
const shared = loadCorpus('shared');
const expectedProjects = ['chromium-phone','chromium-tablet','chromium-desktop','webkit-phone','webkit-tablet','webkit-desktop'];
const rows = new Map();
for (const item of classic) {
  if (!rows.has(item.id)) rows.set(item.id, { id: item.id, name: item.name, source: `${item.uri}:${item.line}`, expandedCases: 0, status: 'unmapped', results: [] });
  rows.get(item.id).expandedCases++;
}
const sharedRows = new Map(shared.map(item => [item.id, { id: item.id, name: item.name, source: `${item.uri}:${item.line}`, expandedCases: 1, status: 'unmapped', results: [] }]));
// Supplemental fixture results may replace only a skipped generic case with
// one matching passed case per project. Explicit paths prevent stale reuse.
const args = process.argv.slice(2);
const separator = args.indexOf('--supplemental');
if (args.filter(arg => arg === '--supplemental').length > 1 || (separator >= 0 && (separator === 0 || separator === args.length - 1))) {
  throw new Error('Usage: ux-parity-report.mjs <primary results...> [--supplemental <fixture results...>]');
}
const primaryPaths = separator < 0 ? args : args.slice(0, separator);
const supplementalPaths = separator < 0 ? [] : args.slice(separator + 1);
if ([...primaryPaths, ...supplementalPaths].some(path => path.startsWith('--'))) {
  throw new Error('Only explicit result paths are accepted');
}
for (const [resultPath, fixture] of [...primaryPaths.map(path => [path, false]), ...supplementalPaths.map(path => [path, true])]) {
  const result = JSON.parse(readFileSync(resultPath, 'utf8'));
  function walk(suites) {
    for (const suite of suites || []) {
      for (const spec of suite.specs || []) {
        const id = spec.title.match(/@(?:ux|shared)-[\w-]+/)?.[0];
        const row = rows.get(id) || sharedRows.get(id);
        if (!row) continue;
        for (const test of spec.tests || []) {
          const last = test.results?.at(-1);
          row.results.push({ project: test.projectName, status: last?.status || 'not-run', errors: (last?.errors || []).map(error => error.message), ...(fixture ? {fixture: true} : {}) });
        }
      }
      walk(suite.suites);
    }
  }
  walk(result.suites);
}
for (const row of [...rows.values(), ...sharedRows.values()]) {
  if (!mappedIds.has(row.id) && !sharedMappedIds.has(row.id)) continue;
  const primary = row.results.filter(test => !test.fixture);
  const supplemental = row.results.filter(test => test.fixture);
  // An absent, failed or incomplete primary project cannot borrow another
  // fixture's success. Multiple runs for a project also cannot erase failures.
  const replaceable = primary.length === expectedProjects.length * row.expandedCases &&
    expectedProjects.every(project => primary.filter(test => test.project === project).length === row.expandedCases);
  const hasSkipped = primary.some(test => test.status === 'skipped');
  const supplementalComplete = replaceable && hasSkipped && expectedProjects.every(project => {
    const skipped = primary.filter(test => test.project === project && test.status === 'skipped');
    const fixtureResults = supplemental.filter(test => test.project === project);
    return fixtureResults.length === skipped.length && fixtureResults.every(test => test.status === 'passed');
  });
  // A clause omitted from the generic fixture can use its own isolated fixture,
  // but only with an exact complete six-project matrix and no competing primary.
  const fixtureOnly = separator >= 0 && !primary.length && supplemental.length === expectedProjects.length * row.expandedCases &&
    expectedProjects.every(project => supplemental.filter(test => test.project === project && test.status === 'passed').length === row.expandedCases);
  const effective = fixtureOnly ? supplemental : supplementalComplete ? primary.map(test => test.status === 'skipped' ? {...test, status: 'passed', supplemented: true} : test) : primary;
  const status = !primary.length && !supplemental.length ? 'not-run' :
    primary.some(test => test.status === 'failed' || test.status === 'interrupted') || supplemental.some(test => test.status === 'failed' || test.status === 'interrupted') || effective.some(test => test.status !== 'passed') ? 'fail' :
    expectedProjects.every(project => effective.filter(test => test.project === project && test.status === 'passed').length === row.expandedCases) ? 'pass' : 'partial-matrix';
  row.status = status;
  if (supplementalComplete || fixtureOnly) row.supplementedProjects = fixtureOnly ? [...expectedProjects] : expectedProjects.filter(project => primary.some(test => test.project === project && test.status === 'skipped'));
}
const counts = {};
for (const row of rows.values()) counts[row.status] = (counts[row.status] || 0) + 1;
const sharedCounts = {};
for (const row of sharedRows.values()) sharedCounts[row.status] = (sharedCounts[row.status] || 0) + 1;
const report = {
  oracleRelease: 'piclaw-3.2.4-linux-x64-baseline',
  historicalSourceCommit: '70d33bc93ab540845bbcf5f80503ca8125c71594',
  scenarios: rows.size, expandedCases: classic.length, sharedContractCases: shared.length,
  expectedProjects, counts, rows: [...rows.values()], sharedCounts, sharedRows: [...sharedRows.values()],
};
mkdirSync('test-results/ux-parity', { recursive: true });
writeFileSync('test-results/ux-parity/matrix.json', JSON.stringify(report, null, 2) + '\n');
writeFileSync('test-results/ux-parity/matrix.md', `# Gi Piclaw Classic parity\n\nOracle: \`${report.oracleRelease}\`. Historical snapshot: \`${report.historicalSourceCommit}\`. Active Classic contracts: ${rows.size} scenarios / ${classic.length} expanded cases. Active shared contracts: ${shared.length} expanded cases, reported separately below. Historical bytes are verified under tests/ux/upstream/.\n\nCounts: ${JSON.stringify(counts)}. A pass requires all six browser/viewport projects; unmapped is not a pass or skip.\n\n| ID | Status | Scenario |\n|---|---|---|\n${[...rows.values()].map(row => `| ${row.id} | ${row.status} | ${row.name.replaceAll('|', '\\|')} |`).join('\n')}\n`);
const sharedMarkdown = `\n## Shared contract (separate evidence)\n\nCounts: ${JSON.stringify(sharedCounts)}.\n\n| ID | Status | Scenario |\n|---|---|---|\n${[...sharedRows.values()].map(row => `| ${row.id} | ${row.status} | ${row.name.replaceAll('|', '\\|')} |`).join('\n')}\n`;
const matrixPath = 'test-results/ux-parity/matrix.md';
writeFileSync(matrixPath, readFileSync(matrixPath,'utf8') + sharedMarkdown);
console.log(JSON.stringify({ scenarios: rows.size, expandedCases: classic.length, sharedContractCases: shared.length, counts, sharedCounts }));
