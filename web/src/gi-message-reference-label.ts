// Presentation only. Never use these labels as storage, API, link or submit IDs.
export function messageReferenceLabel(id: unknown, labels: Record<string,string> = {}) {
    const canonical=String(id);
    if(Object.prototype.hasOwnProperty.call(labels,canonical))return labels[canonical];
    return canonical.length>14 ? `${canonical.slice(0,4)}…${canonical.slice(-6)}` : canonical;
}
export function messageReferenceLabels(posts: any[]) {
    const labels:Record<string,string>=Object.create(null);
    for(const post of posts||[])if(Number.isSafeInteger(post.display_row_id)&&post.display_row_id>0)labels[String(post.id)]=String(post.display_row_id);
    return labels;
}
