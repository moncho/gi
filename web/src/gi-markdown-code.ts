// Apply HTML compatibility transforms to prose, never authored code.
export function mapMarkdownOutsideCode(text: string, transform: (prose: string) => string): string {
    const source = String(text || '').replace(/\r\n?/g, '\n');
    const ranges: [number, number][] = [];
    const opening = /^ {0,3}(`{3,}|~{3,})[^\n]*\n/gm;
    let match: RegExpExecArray | null;
    while ((match = opening.exec(source))) {
        const fence = match[1];
        const closing = new RegExp(`^ {0,3}${fence[0]}{${fence.length},}[ \\t]*(?:\\n|$)`, 'gm');
        closing.lastIndex = opening.lastIndex;
        const end = closing.exec(source);
        const stop = end ? closing.lastIndex : source.length;
        ranges.push([match.index, stop]);
        opening.lastIndex = stop;
    }
    let result = '', cursor = 0;
    const prose = (value: string) => {
        // Inline code uses matching backtick runs; unmatched runs remain prose.
        const inline = /(`+)(?!`)([\s\S]*?[^`])\1(?!`)/g;
        let out = '', from = 0, span: RegExpExecArray | null;
        while ((span = inline.exec(value))) {
            out += transform(value.slice(from, span.index)) + span[0];
            from = inline.lastIndex;
        }
        return out + transform(value.slice(from));
    };
    for (const [start, end] of ranges) {
        result += prose(source.slice(cursor, start)) + source.slice(start, end);
        cursor = end;
    }
    return result + prose(source.slice(cursor));
}
