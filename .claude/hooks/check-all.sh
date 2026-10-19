#!/usr/bin/env bash
# Runs post-edit.sh over every hand-written .go file. Exit 1 if any file violates layer rules.
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
status=0
while IFS= read -r f; do
  printf '{"tool_input":{"file_path":"%s"}}' "$f" | "$ROOT/.claude/hooks/post-edit.sh" || status=1
done < <(find "$ROOT" -name '*.go' -not -path '*/repository/sqlcgen/*')
[[ $status -eq 0 ]] && echo "layer check: ok"
exit $status
