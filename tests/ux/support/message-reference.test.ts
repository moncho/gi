import { expect, test } from 'bun:test';
import { messageReference } from '../../../web/src/gi-message-reference';

test('message references use the numeric row ID', () => {
    const posts = [{ id: 'msg_1790942745618391663', display_row_id: 42 }, { id: 'msg_2' }];
    expect(messageReference(posts, 'msg_1790942745618391663')).toBe(42);
    expect(messageReference(posts, 'msg_2')).toBe('msg_2');
    expect(messageReference(posts, 'unknown')).toBe('unknown');
});
