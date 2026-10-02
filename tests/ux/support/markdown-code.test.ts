import { test, expect } from 'bun:test';
import { readFileSync } from 'node:fs';
import { mapMarkdownOutsideCode } from '../../../web/src/gi-markdown-code';
import { patchMarkdownCode } from '../../../scripts/gi-markdown-code-adapter.mjs';

const code = 'if (a < b && c > d) return x <i> y;';
const change = (prose: string) => prose.replaceAll('<', '!');

test('HTML compatibility transforms preserve closed and unfinished fenced code verbatim', () => {
    for (const fence of ['```', '~~~~']) {
        for (const end of [`\n${fence}`, '']) {
            const block = `${fence}js\n${code}${end}`;
            expect(mapMarkdownOutsideCode(`<b>before</b>\n\n${block}`, change)).toBe(`!b>before!/b>\n\n${block}`);
        }
    }
});

test('inline code keeps angle brackets and entity spellings while prose is transformed', () => {
    const inline = '`a < b && &lt;i&gt;`';
    expect(mapMarkdownOutsideCode(`<b>prose</b> ${inline}`, change)).toBe(`!b>prose!/b> ${inline}`);
    expect(mapMarkdownOutsideCode('``a ` < b`` <i>after</i>', change)).toBe('``a ` < b`` !i>after!/i>');
});

test('build adapter preserves the supplied file and fails closed on source drift or double application', () => {
    const source = readFileSync('web/src/markdown.ts', 'utf8');
    const patched = patchMarkdownCode(source);
    expect(patched).toContain('mapMarkdownOutsideCode(stripped, prose =>');
    expect(source).not.toContain('mapMarkdownOutsideCode');
    expect(() => patchMarkdownCode(patched)).toThrow('anchor changed');
    expect(() => patchMarkdownCode(source.replace('const decoded = decodeEntitiesDeep(stripped, 2);', 'different'))).toThrow('anchor changed');
});
