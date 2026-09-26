# Model picker keyboard gap (current oracle, frozen Classic021)

Piclaw 3.2.4 shipped source `components/model-picker.ts:75-103` handles ArrowUp,
ArrowDown, PageUp, PageDown, and Control/Meta+Home/End while the model search
field is focused. Plain Home/End stays with text editing; Enter activates the
highlighted entry and Escape closes the picker. The file is byte-identical to
the frozen `70d33bc` version (`frozen-to-oracle-sources.json`). The frozen
`@ux-original-021` clause at `tests/ux/features/classic/canonical/canonical-ux.feature:205-218`
correctly describes that keyboard contract.

Gi's `tests/ux/models.spec.mjs:46-72` is tagged `@ux-original-021`, but tests a
held model response after switching sessions. It does **not** test those keys.
Gi's supplied `web/src/components/compose-box.ts:1491-1535` handles only
ArrowUp/ArrowDown and Enter for the model popup; the unused helper in
`web/src/gi-model-picker.ts:11-33` rejects Control/Meta and cannot provide this
behaviour as-is. Session picker has separate Page/Home/End evidence at
`tests/ux/session.spec.mjs:1031-1063`, which cannot be borrowed for the model
picker clause.

**Finding:** the Classic021 mapping is unsupported by its tagged test and the
current Gi model-popup path. This is a code-and-test gap, not a reason to weaken
Gherkin. Implement a Gi-owned adapter (not a supplied component edit) for the
model picker and add six-project native-keyboard tests: search-field plain
Home/End caret, Ctrl/Meta+Home/End highlight, PageUp/Down, Arrow, Enter exact
selection, Escape focus, disabled entries, and no draft submission. Until then
keep the historical mapping explicitly disputed; no new parity credit.
