import {readFileSync} from 'node:fs';
import {generateMessages} from '@cucumber/gherkin';
import {IdGenerator,SourceMediaType} from '@cucumber/messages';

export const giDeviationFile = 'tests/features/ux/gi-deviations.feature';
export function loadGiDeviations() {
  const source = readFileSync(new URL('../../features/ux/gi-deviations.feature', import.meta.url), 'utf8');
  const messages = generateMessages(source, giDeviationFile, SourceMediaType.TEXT_X_CUCUMBER_GHERKIN_PLAIN, {
    newId: IdGenerator.incrementing(), includeSource: false, includeGherkinDocument: false, includePickles: true,
  });
  const errors = messages.filter(message => message.parseError);
  if (errors.length) throw new Error(JSON.stringify(errors));
  return messages.flatMap(({pickle}) => pickle ? [{
    id: pickle.tags.map(tag => tag.name).find(tag => /^@gi-ux-\d+$/.test(tag)),
    name: pickle.name,
    steps: pickle.steps.map(step => step.text),
    tags: pickle.tags.map(tag => tag.name),
  }] : []);
}

export async function attachGiDeviation(info, id) {
  const row = loadGiDeviations().find(scenario => scenario.id === id);
  if (!row) throw new Error(`Missing Gi-specific UX scenario: ${id}`);
  await info.attach('gi-specific-gherkin', {body: `${giDeviationFile} ${id}\n${row.steps.join('\n')}`, contentType: 'text/plain'});
}
