# Classic image and text annotations 001–012: native capability gap

`tests/ux/features/classic/timeline/annotation-highlights.feature` combines
seven iPad image-annotator clauses with five persistent text-highlight clauses.
The pinned Piclaw 3.2.4 shipped source map contains
`src/components/image-annotator.ts` and `src/components/post-highlights.ts`;
the latter documents the `PATCH /post/:id/annotations` path. Gi's native
`web/src/components/post.ts` shows audience/priority/updated metadata badges,
search-term highlights, and a stored-image lightbox, **not** an editable
annotation toolbar, saved selection offsets, image crop/export workflow or
persistent highlight API. Source searches found no native
`image-annotator`/`post-highlights` module or matching post-annotations endpoint.
These are capability gaps, not a licence to mark adjacent badges or lightbox
controls as annotation acceptance.

| Frozen IDs | Required interaction | Gi status |
|---|---|---|
| `001`–`005`, `007` | iPad-only inline image annotator with tools, two-finger pinch exclusion, crop/reset, flattened PNG upload/preview/cancel, SVG rasterization. | No native annotator. No tagged positive journey or physical iPad probe. |
| `006` | Non-iPad activation opens lightbox rather than annotator. | A **partial, untagged** Gi touch test in `lightbox.spec.mjs` checks real non-iPad identity, click/tap and lightbox with no annotator UI or writes. It does not verify that annotation actions were enabled or an actual iPad-vs-non-iPad gate. The nearby `013`–`016` lightbox tags passed 24/24 but do not fill this clause. |
| `008`–`012` | Text selection toolbar/colors; saved selection snapshot and `textOffset`; PATCH persistence; fine-pointer near-selection and coarse-pointer docked placement. | No native persistent text-highlight interaction/API or tagged journey. Search-term highlighting and metadata badges are unrelated. |

The current Piclaw 3.2.4 annotation interactions and physical gestures were
not exercised. A source-map module proves availability in the shipped bundle,
not completion of its backend/UI contract. No frozen feature or Gi production
code changed in this review.
