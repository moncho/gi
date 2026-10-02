// Guarded build-time correction: keep the supplied Markdown renderer unchanged on disk.
export function patchMarkdownCode(source) {
    const anchor = `    // Decode HTML entities first (in case content has encoded entities)
    const decoded = decodeEntitiesDeep(stripped, 2);
    const normalized = normalizeHtmlCodeTags(decoded);
    const escaped = normalized
        .replace(/</g, '&lt;');
    const safeHtml = restoreAllowedHtmlTags(escaped);`;
    if (source.split(anchor).length !== 2) throw new Error('Gi Markdown code adapter anchor changed');
    return "import { mapMarkdownOutsideCode } from './gi-markdown-code.js';\n" + source.replace(anchor,
`    const safeHtml = mapMarkdownOutsideCode(stripped, prose => {
        const decoded = decodeEntitiesDeep(prose, 2);
        const normalized = normalizeHtmlCodeTags(decoded);
        return restoreAllowedHtmlTags(normalized.replace(/</g, '&lt;'));
    });`);
}
