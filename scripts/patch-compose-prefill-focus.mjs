// Preserve focus ownership when a delayed Quick Action prefill settles after
// the composer unmounts or Settings opens. Supplied components remain unchanged.
export function patchComposePrefillFocus(source) {
    const anchor = `        updateMentionAutocomplete(resolved.text);
        requestAnimationFrame(() => {
            resizeTextarea();`;
    if (source.split(anchor).length !== 2) throw new Error('Compose prefill focus adapter anchor changed');
    return source.replace(anchor, `        updateMentionAutocomplete(resolved.text);
        requestAnimationFrame(() => {
            if (!mountedRef.current || document.querySelector('.settings-dialog[aria-modal="true"]')) return;
            resizeTextarea();`);
}
