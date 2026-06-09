#!/bin/bash
set -e
DIR="$(cd "$(dirname "$0")/../.." && pwd)"
SCHEMA="$DIR/rollback/schema.json"
python3 -c "
import json
with open('$SCHEMA') as f:
    s = json.load(f)
assert s['name'] == 'rollback'
assert 'scope' in s['input_schema']['required']
print('OK: rollback schema valid')
"
