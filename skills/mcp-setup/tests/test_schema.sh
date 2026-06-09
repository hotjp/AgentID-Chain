#!/bin/bash
set -e
DIR="$(cd "$(dirname "$0")/../.." && pwd)"
SCHEMA="$DIR/mcp-setup/schema.json"
python3 -c "
import json
with open('$SCHEMA') as f:
    s = json.load(f)
assert s['name'] == 'mcp_setup'
assert 'client' in s['input_schema']['required']
print('OK: mcp-setup schema valid')
"
