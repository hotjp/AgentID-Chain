#!/bin/bash
set -e
DIR="$(cd "$(dirname "$0")/../.." && pwd)"
SCHEMA="$DIR/a2a-setup/schema.json"
python3 -c "
import json
with open('$SCHEMA') as f:
    s = json.load(f)
assert s['name'] == 'a2a_setup'
assert 'agent_id' in s['input_schema']['required']
print('OK: a2a-setup schema valid')
"
