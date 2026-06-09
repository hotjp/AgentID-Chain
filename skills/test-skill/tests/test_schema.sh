#!/bin/bash
set -e
DIR="$(cd "$(dirname "$0")/../.." && pwd)"
SCHEMA="$DIR/test-skill/schema.json"
python3 -c "
import json
with open('$SCHEMA') as f:
    s = json.load(f)
assert s['name'] == 'test'
print('OK: test schema valid')
"
