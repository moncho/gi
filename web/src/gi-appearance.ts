// Catalogue/renderer exports are added by gi-appearance-renderer in build.js.
// The supplied theme module itself is never modified.
import { giThemePresets, giApplyThemeState } from './ui/theme.js';
import { APPEARANCE_KEY, defaultAppearance, normalizeOutputPad, readAppearance, saveAppearance, validateAppearance, type Appearance } from './gi-appearance-state.js';
export const appearancePresets = Object.keys(giThemePresets);
const changeEvent = 'gi:appearance-changed';
// api.ts announces /theme and /tint results without importing the renderer.
export const themeCommandEvent = 'gi:theme-command';

function readStoredAppearance() {
    try { return readAppearance(window.localStorage, appearancePresets); } catch { return null; }
}

export function currentAppearance(): Appearance {
    const stored = readStoredAppearance();
    if (stored) return stored;
    const theme = document.documentElement.dataset.colorTheme || 'default';
    return { version: 1, theme: appearancePresets.includes(theme) ? theme : 'default', tint: document.documentElement.dataset.tint || '', outputPad: normalizeOutputPad(document.documentElement.dataset.outputPad) };
}

export const appearancePresetLabels: Record<string, string> = Object.fromEntries(
    Object.entries(giThemePresets as Record<string, any>).map(([name, preset]) => [name, preset?.label || name]));

export function applyOutputPad(value: any) {
    const pad = normalizeOutputPad(value);
    document.documentElement.style.setProperty('--output-pad', `${pad}px`);
    document.documentElement.dataset.outputPad = String(pad);
}

function render(value: Appearance) {
    giApplyThemeState(value, { persist: false });
    applyOutputPad(value.outputPad);
    window.dispatchEvent(new CustomEvent(changeEvent, { detail: value }));
}

export function persistAppearance(value: Appearance) {
    const saved = saveAppearance(window.localStorage, value, appearancePresets);
    render(saved);
    return saved;
}

/** A CSS colour name as #rrggbb, or '' when the browser does not know it. */
export function tintToHex(value: string): string {
    if (/^#[0-9a-f]{6}$/i.test(value)) return value.toLowerCase();
    if (!globalThis.CSS?.supports?.('color', value)) return '';
    const context = document.createElement('canvas').getContext('2d');
    if (!context) return '';
    context.fillStyle = value;
    const hex = String(context.fillStyle);
    return /^#[0-9a-f]{6}$/i.test(hex) ? hex.toLowerCase() : '';
}

/** Apply a /theme or /tint result. Gi keeps appearance in the browser, so the
 * server only returns the payload; storage failure still changes this page. */
export function applyThemeCommandPayload(payload: any) {
    if (!payload || typeof payload.theme !== 'string' || !appearancePresets.includes(payload.theme)) return;
    const tint = typeof payload.tint === 'string' ? tintToHex(payload.tint) : '';
    const next = validateAppearance({ ...currentAppearance(), theme: payload.theme, tint }, appearancePresets);
    try { persistAppearance(next); } catch { render(next); }
}

export function subscribeAppearance(onChange: (value: Appearance) => void) {
    const listener = (event: CustomEvent) => onChange(event.detail);
    window.addEventListener(changeEvent, listener);
    return () => window.removeEventListener(changeEvent, listener);
}

export function initGiAppearance() {
    // The explicit Gi preference overrides legacy URL/chat theme lookup without
    // changing those old keys. No preference means legacy startup is unchanged.
    const saved = readStoredAppearance();
    if (saved) render(saved);
    const storage = (event: StorageEvent) => {
        if (event.storageArea !== window.localStorage || (event.key !== APPEARANCE_KEY && event.key !== null)) return;
        const next = readStoredAppearance();
        if (next) render(next);
        else if (event.newValue === null) render(defaultAppearance);
    };
    window.addEventListener('storage', storage);
    const command = (event: Event) => applyThemeCommandPayload((event as CustomEvent).detail);
    window.addEventListener(themeCommandEvent, command);
    return () => { window.removeEventListener('storage', storage); window.removeEventListener(themeCommandEvent, command); };
}
