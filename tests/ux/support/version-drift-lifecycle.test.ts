import {afterEach, expect, test} from 'bun:test';
import {handleUiVersionDriftEvent} from '../../../web/src/ui/app-connection-lifecycle';

const oldWindow = globalThis.window;
const oldDocument = globalThis.document;
afterEach(() => {
  if (oldWindow === undefined) delete (globalThis as any).window;
  else (globalThis as any).window = oldWindow;
  if (oldDocument === undefined) delete (globalThis as any).document;
  else (globalThis as any).document = oldDocument;
});

function probe(draft: string, unsaved = false) {
  const notices: string[] = [];
  const timers: Array<() => void> = [];
  let reloads = 0;
  (globalThis as any).document = {querySelector: () => ({value: draft})};
  (globalThis as any).window = {
    setTimeout: (callback: () => void, delay: number) => {
      expect(delay).toBe(350);
      timers.push(callback);
    },
    location: {reload: () => {reloads++;}},
  };
  const staleUiVersionRef = {current: null as string | null};
  const staleUiReloadScheduledRef = {current: false};
  const observe = () => handleUiVersionDriftEvent({
    serverVersion: 'new', currentAppAssetVersion: 'loaded',
    staleUiVersionRef, staleUiReloadScheduledRef,
    tabStoreHasUnsaved: () => unsaved,
    isAgentRunningRef: {current: false}, pendingRequestRef: {current: null},
    showIntentToast: title => notices.push(title),
  });
  return {observe, notices, timers, reloads: () => reloads};
}

test('the copied Piclaw lifecycle helper schedules a clean-state reload and guards repeated version notices', () => {
  const p = probe('');
  expect(p.observe()).toBe(true);
  expect(p.notices).toEqual(['Updating UI…']);
  expect(p.timers).toHaveLength(1);
  expect(p.observe()).toBe(true);
  expect(p.notices).toHaveLength(1);
  expect(p.timers).toHaveLength(1);
  p.timers[0]();
  expect(p.reloads()).toBe(1);
});

test('the copied Piclaw lifecycle helper warns without reload for a composer draft or unsaved editor', () => {
  for (const [draft, unsaved] of [['draft', false], ['', true]] as const) {
    const p = probe(draft, unsaved);
    expect(p.observe()).toBe(true);
    expect(p.notices).toEqual(['New UI available']);
    expect(p.timers).toHaveLength(0);
    expect(p.observe()).toBe(true);
    expect(p.notices).toHaveLength(1);
  }
});
