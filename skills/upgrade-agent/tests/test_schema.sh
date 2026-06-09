#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
python3 -m json.tool "$SKILL_DIR/schema.json" > /dev/null
python3 -c "import json; d=json.load(open('$SKILL_DIR/schema.json')); assert d['name']=='upgrade_agent'"
[ -f "$SKILL_DIR/SKILL.md" ]
echo "OK: upgrade-agent"
