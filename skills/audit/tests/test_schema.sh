#!/bin/bash
set -e
DIR="$(cd "$(dirname "$0")/../.." && pwd)"
SCHEMA="$DIR/audit/schema.json"
python3 -c "
import json, sys
with open('$SCHEMA') as f:
    s = json.load(f)
assert s['name'] == 'audit_agent'
assert 'agent_id' in s['input_schema']['required']
print('OK: audit schema valid')
"
