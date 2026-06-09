#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
SCHEMA="$SKILL_DIR/schema.json"

python3 -m json.tool "$SCHEMA" > /dev/null
python3 -c "import json; d=json.load(open('$SCHEMA')); assert d['name']=='get_agent'"
[ -f "$SKILL_DIR/SKILL.md" ]
[ -d "$SKILL_DIR/examples" ]
echo "OK: query-agent"
