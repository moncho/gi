# Classic /theme and /tint 001–015: native command gap

`tests/ux/features/classic/compose/theme-tint.feature` requires commands sent
through the Classic composer, timeline responses and visual changes. Gi has a
browser-local Appearance Settings pane, but it is a different interaction:
`web/src/gi-settings-appearance.ts` exposes explicit Save/Reset;
`web/src/gi-appearance.ts` persists `gi_browser_appearance_v1`. It does not
write the legacy `piclaw_theme`/`piclaw_tint` keys. The native quick-action
catalogue (`internal/web/quick_actions.go`) advertises `/model`, `/compact`
and loaded skills, not `/theme` or `/tint`. The Go turn/web handlers have no
`/theme` or `/tint` command path. No `@ux-theme-001`–`015` Gi browser test is
tagged.

| Frozen IDs | Missing interaction | Existing evidence that cannot substitute |
|---|---|---|
| `001`, `004`, `010`, `011` | Composer commands list themes/usage or reject invalid theme/tint with timeline feedback and unchanged appearance. | Native Appearance Settings validation does not submit composer commands or yield those responses. |
| `002`, `003`, `005`, `013`–`015` | `/theme` switches to/from ristretto, updates root attributes, CSS variables, legacy storage and timeline; round-trip matches the starting colours. | A saved Settings preset changes browser-local appearance, but uses different storage, explicit Save and no command timeline. |
| `006`–`009`, `012` | `/tint` sets named/hex colours, off, default-theme interaction, visual differences and refresh persistence through Classic command state. | Settings accepts `#RGB`/`#RRGGBB` for the default preset, not the full Classic command semantics or named-colour path. |

This is a **native capability gap for all 15 frozen scenarios**. It does not
negate `@gi-settings-009`/`010` or their own Appearance Settings tests. Piclaw
3.2.4 theme-command UI was not separately probed here; the frozen Classic
Gherkin is retained as historical contract, and implementation decisions must
keep that version boundary explicit. No parity or physical/pixel credit follows
from visual CSS variables or the Settings tests alone.
