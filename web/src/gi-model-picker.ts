import { sessionTypeahead } from './gi-session-typeahead.js';

// Preserve authoritative catalogue order and option identity while filtering.
export function filterModelOptions<T>(options: T[], query: string, label: (option: T) => string): T[] {
    const terms = query.toLowerCase().trim().split(/\s+/).filter(Boolean);
    return options.filter(option => terms.every(term => label(option).toLowerCase().includes(term)));
}

// Piclaw's model picker pages seven selectable entries and clamps at both ends.
// Keep this distinct from the session picker's eight-row, wrapping navigation.
export function moveModelPickerIndex(current: number, length: number, key: string): number {
    if (length <= 0) return -1;
    if (key === 'Home') return 0;
    if (key === 'End') return length - 1;
    if (key === 'ArrowDown') return current < 0 ? 0 : Math.min(length - 1, current + 1);
    if (key === 'ArrowUp') return current < 0 ? length - 1 : Math.max(0, current - 1);
    if (key === 'PageDown') return current < 0 ? 0 : Math.min(length - 1, current + 7);
    if (key === 'PageUp') return current < 0 ? length - 1 : Math.max(0, current - 7);
    return current;
}

// Indices always belong to the visible list, not its enabled-only projection.
export function modelPickerKey(event: KeyboardEvent, entries: {label: string; disabled: boolean}[], current: number, previous: any) {
    if (event.defaultPrevented || event.isComposing || event.altKey || (event.ctrlKey && event.metaKey)) return null;
    const target = event.target as Element;
    const editing = Boolean(target?.closest?.('input, textarea, select, [contenteditable="true"]'));
    const searchJump = Boolean(target?.matches?.('input[type="search"]'))
        && (event.ctrlKey || event.metaKey) && ['Home', 'End'].includes(event.key);
    if ((event.ctrlKey || event.metaKey) && !searchJump) return null;
    const nativeButton = target?.closest?.('button');
    const focused = target?.closest?.('[data-model-index]')?.getAttribute('data-model-index');
    const index = focused == null ? current : Number(focused);
    const enabled = entries.map((entry, index) => ({entry, index})).filter(item => !item.entry.disabled);
    const buffer = {value: '', updatedAt: 0};
    if (['ArrowDown', 'ArrowUp', 'PageDown', 'PageUp'].includes(event.key)
        || ((!editing || searchJump) && ['Home', 'End'].includes(event.key))) {
        const selected = enabled.findIndex(item => item.index === index);
        return {index: enabled[moveModelPickerIndex(selected, enabled.length, event.key)]?.index ?? -1, buffer, activate: false, focus: !editing};
    }
    if (event.key === 'Enter') {
        if (nativeButton && !event.repeat) return null; // The actual focused button owns activation.
        return {index, buffer, activate: !event.repeat && Boolean(entries[index] && !entries[index].disabled), focus: false};
    }
    const typed = sessionTypeahead(event, entries, previous);
    return typed ? {...typed, activate: false, focus: true} : null;
}
