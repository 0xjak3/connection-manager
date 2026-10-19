#!/usr/bin/env bash
# PostToolUse hook: gofmt + per-layer import boundaries for connection_manager.
# Exit 2 sends stderr back to Claude so it fixes the violation.
# The dependency table lives in .claude/rules/architecture.md; keep both in sync.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MODULE="$(awk '/^module /{print $2; exit}' "$ROOT/go.mod")"
input="$(cat)"

if command -v jq >/dev/null 2>&1; then
  file="$(printf '%s' "$input" | jq -r '.tool_input.file_path // empty')"
else
  file="$(printf '%s' "$input" | grep -o '"file_path"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"\([^"]*\)"$/\1/')"
fi
[[ -n "$file" && "$file" == *.go ]] || exit 0
case "$file" in /*) ;; *) file="$PWD/$file" ;; esac
case "$file" in "$ROOT"/*) ;; *) exit 0 ;; esac
rel="${file#"$ROOT"/}"

if [[ "$rel" == repository/sqlcgen/* ]]; then
  echo "repository/sqlcgen/ is generated. Edit db/queries/*.sql or db/migrations/*.sql and run 'sqlc generate' instead." >&2
  exit 2
fi

if [[ "$rel" == dto/*.gen.go ]]; then
  echo "dto/*.gen.go is generated from openapi.yaml. Edit openapi.yaml and run 'make generate' instead." >&2
  exit 2
fi

[[ -f "$file" ]] || exit 0
gofmt -w "$file" 2>/dev/null # syntax errors are left for the compiler to report

m="$MODULE"
case "${rel%%/*}" in
  domain)     forbid="^$m/(handler|dto|router|usecase|service|repository|util|cmd)(/|$)|^github.com/gofiber/|^github.com/jackc/|^database/sql|^net/http$" ;;
  dto)        forbid="^$m/" ;;
  handler)    forbid="^$m/(repository|service|router|util|cmd)(/|$)|^github.com/jackc/|^database/sql" ;;
  router)     forbid="^$m/(repository|service|usecase|util|cmd)(/|$)|^github.com/jackc/|^database/sql" ;;
  usecase)    forbid="^$m/(handler|dto|router|repository|service|util|cmd)(/|$)|^github.com/gofiber/|^github.com/jackc/|^database/sql" ;;
  service)    forbid="^$m/(handler|dto|router|usecase|repository|util|cmd)(/|$)|^github.com/gofiber/" ;;
  repository) forbid="^$m/(handler|dto|router|usecase|service|util|cmd)(/|$)|^github.com/gofiber/" ;;
  util)       forbid="^$m/" ;;
  test)       forbid="^$m/" ;; # e2e is black-box: HTTP only
  *)          exit 0 ;;
esac

imports="$(awk '
  /^import[[:space:]]*\(/ { blk = 1; next }
  blk && /^\)/           { blk = 0; next }
  blk                    { print; next }
  /^import[[:space:]]+/  { print }
' "$file" | grep -o '"[^"]*"' | tr -d '"')"

bad="$(printf '%s\n' "$imports" | grep -E "$forbid" || true)"
if [[ -n "$bad" ]]; then
  {
    echo "Layer boundary violation in $rel (layer: ${rel%%/*}). Forbidden imports:"
    printf '  %s\n' $bad
    echo "See .claude/rules/architecture.md (Dependency rules) and move the code to the right layer."
  } >&2
  exit 2
fi
exit 0
