// Message references carry the numeric row ID (Piclaw's message IDs are row
// IDs): "msg:42" is easy to read and say, and the messages tool takes row_ids.
// The canonical text ID stays the post's identity everywhere else.
type Post = { id?: any; display_row_id?: any };

export function messageReference(posts: Post[], id: any) {
    const row = Number(posts.find(p => p.id === id)?.display_row_id);
    return Number.isInteger(row) && row > 0 ? row : id;
}

export function scrollToReferencedPost(posts: Post[], ref: any) {
    const post = posts.find(p => String(p.display_row_id) === String(ref) || p.id === ref);
    const el = post && document.getElementById('post-' + post.id);
    if (!el) return;
    el.scrollIntoView({ behavior: 'smooth', block: 'center' });
    el.classList.add('post-highlight');
    setTimeout(() => el.classList.remove('post-highlight'), 2000);
}
