#!/usr/bin/env bash
# test_schema.sh — 验证 register-agent schema 是合法 JSON
#
# 用法: ./skills/register-agent/tests/test_schema.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
SCHEMA="$SKILL_DIR/schema.json"

echo "Testing register-agent schema..."

# 1. JSON 合法
if ! python3 -m json.tool "$SCHEMA" > /dev/null 2>&1; then
    echo "  FAIL: schema.json is not valid JSON"
    exit 1
fi
echo "  PASS: schema is valid JSON"

# 2. 必填字段
for field in name description input_schema output_schema; do
    if ! python3 -c "import json; d=json.load(open('$SCHEMA')); assert '$field' in d, 'missing: $field'" 2>/dev/null; then
        echo "  FAIL: missing field: $field"
        exit 1
    fi
done
echo "  PASS: all required fields present"

# 3. input_schema 必填
for field in type properties required; do
    if ! python3 -c "
import json
d = json.load(open('$SCHEMA'))
assert '$field' in d['input_schema'], 'missing: input_schema.$field'
" 2>/dev/null; then
        echo "  FAIL: missing: input_schema.$field"
        exit 1
    fi
done
echo "  PASS: input_schema complete"

# 4. SKILL.md 存在
if [ ! -f "$SKILL_DIR/SKILL.md" ]; then
    echo "  FAIL: SKILL.md not found"
    exit 1
fi
echo "  PASS: SKILL.md present"

# 5. examples 目录
if [ ! -d "$SKILL_DIR/examples" ]; then
    echo "  FAIL: examples/ not found"
    exit 1
fi
example_count=$(ls "$SKILL_DIR/examples" | wc -l | tr -d ' ')
if [ "$example_count" -lt 1 ]; then
    echo "  FAIL: no examples in examples/"
    exit 1
fi
echo "  PASS: $example_count example(s)"

echo "All tests passed for register-agent"
