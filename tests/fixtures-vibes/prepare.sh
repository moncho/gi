#!/bin/sh
# Disposable fixture-only build. The production binary never registers this model.
set -eu
: "${FIXTURES_ROOT:?}"
: "${FIXTURE_MODEL_URL:?}"
: "${FIXTURE_MODEL_ID:?}"
[ "$FIXTURE_MODEL_ID" = 'fixture-1' ] || { echo 'Unexpected fixture model' >&2; exit 1; }
case "$FIXTURES_ROOT" in /tmp/fixtures-gi-*) ;; *) echo 'Refusing non-disposable fixture root' >&2; exit 1;; esac
mkdir -p "$FIXTURES_ROOT/bin" "$FIXTURES_ROOT/workspace/.pi" "$FIXTURES_ROOT/workspace/.piclaw" "$FIXTURES_ROOT/workspace/.gi/skills/proof" "$FIXTURES_ROOT/home/.pi/agent"
cp "$(dirname "$0")/../ux/fixtures/skills/.gi/skills/proof/SKILL.md" "$FIXTURES_ROOT/workspace/.gi/skills/proof/SKILL.md"
mkdir -p "$FIXTURES_ROOT/workspace/.gi/skills/review"
printf '%s\n' '---' 'name: review' 'description: Examine the vermilion verification marker' '---' 'Canonical skill body: VERMILION_NATIVE_SKILL_V1.' > "$FIXTURES_ROOT/workspace/.gi/skills/review/SKILL.md"
# Shared editor specs open a visible workspace README.
printf '%s\n' '# Fixture workspace' '' 'Sample workspace file.' > "$FIXTURES_ROOT/workspace/README.md"
# Shared compose specs reference a visible (non-root) workspace folder.
mkdir -p "$FIXTURES_ROOT/workspace/notes"
printf '%s\n' '# Notes' '' 'Fixture folder.' > "$FIXTURES_ROOT/workspace/notes/index.md"
printf '%s\n' '{"defaultProvider":"fixture-vibes","defaultModel":"fixture-vibes/fixture-1","defaultThinkingLevel":"low","enabledModels":["fixture-vibes/fixture-1","fixture-vibes/fixture-2"],"maxIterations":4,"inboundWork":{"enabled":false}}' > "$FIXTURES_ROOT/workspace/.pi/settings.json"
printf '%s\n' '{"assistant":{"assistantName":"Gi Fixture"},"user":{"userName":"Fixture User"}}' > "$FIXTURES_ROOT/workspace/.piclaw/config.json"
printf '%s\n' '{"fixture-vibes":{"type":"api_key","apiKey":"fixture-only"}}' > "$FIXTURES_ROOT/home/.pi/agent/auth.json"
# The Makefile builds once before Playwright starts. Replacement workers only
# copy that fixture build, so compilation cannot exhaust their setup timeout.
if [ -n "${GI_FIXTURE_BIN:-}" ]; then
  cp "$GI_FIXTURE_BIN" "$FIXTURES_ROOT/bin/gi"
else
  cd "$(dirname "$0")/../.."
  go build -tags fixtures_vibes -o "$FIXTURES_ROOT/bin/gi" ./cmd/gi
fi
