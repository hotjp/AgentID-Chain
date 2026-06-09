#!/bin/bash
set -e
DIR="$(cd "$(dirname "$0")/../.." && pwd)"
SCHEMA="$DIR/docker-deploy/schema.json"
python3 -c "
import json
with open('$SCHEMA') as f:
    s = json.load(f)
assert s['name'] == 'docker_deploy'
print('OK: docker-deploy schema valid')
"
