#!/usr/bin/env python3
"""Start pprotein collection when the benchmark announces its load phase."""
import datetime
import json
import pathlib
import re
import subprocess
import time
import urllib.request

api = 'http://127.0.0.1:9000/api/'
def get(path):
    with urllib.request.urlopen(api + path, timeout=15) as r:
        return json.load(r)

targets = get('group/targets')
expected = {t: {x['Label'] for x in targets if x['Type'] == t} for t in ('pprof', 'httplog', 'slowlog')}
def ready(entries):
    flat = [e for es in entries.values() for e in es]
    return (bool(flat) and len({e['Snapshot']['GroupId'] for e in flat}) == 1
            and all(len(es) == len(expected[t]) and {e['Snapshot']['Label'] for e in es} == expected[t]
                    and all(e['Status'] == 'ok' for e in es) for t, es in entries.items()))

started = datetime.datetime.now(datetime.timezone.utc)
prefix = pathlib.Path('/home/isucon/bench-results') / started.strftime('%Y%m%d-%H%M%S-pprotein')
prefix.parent.mkdir(exist_ok=True)
before = {t: {e['Snapshot']['ID'] for e in get(t)} for t in ('pprof', 'httplog', 'slowlog')}
command = ['./bench', '-target-url', 'https://t.isucon.dev', '-target-addr', '127.0.0.1:443', '-duration', '1m', '-exit-error-on-fail']
proc = subprocess.Popen(command, cwd='/home/isucon/bench', stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, bufsize=1)
collected = False
collection_error = None
with prefix.with_suffix('.log').open('w') as log:
    for line in proc.stdout:
        log.write(line)
        log.flush()
        print(line, end='', flush=True)
        if not collected and '負荷走行を開始します' in line:
            collected = True
            try:
                with urllib.request.urlopen(api + 'group/collect', timeout=15) as r:
                    print('PPROTEIN_COLLECTION_STARTED', r.status, flush=True)
            except Exception as e:
                collection_error = str(e)
                print('PPROTEIN_COLLECTION_ERROR', e, flush=True)
exit_code = proc.wait()
entries = {}
for _ in range(30):
    entries = {t: [e for e in get(t) if e['Snapshot']['ID'] not in before[t]] for t in before}
    if ready(entries):
        break
    time.sleep(1)
text = prefix.with_suffix('.log').read_text()
scores = re.findall(r'SCORE:\s*(\d+)', text)
errors = re.findall(r'Error (\d+) \(Critical:(\d+)\)', text)
passed = re.findall(r'PASSED:\s*(true|false)', text)
result = {
    'started_at_utc': started.isoformat(),
    'finished_at_utc': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'command': command, 'exit_code': exit_code,
    'score': int(scores[-1]) if scores else None,
    'passed': passed[-1] == 'true' if passed else None,
    'errors': int(errors[-1][0]) if errors else None,
    'critical_errors': int(errors[-1][1]) if errors else None,
    'log': str(prefix.with_suffix('.log')),
    'collection_error': collection_error,
    'pprotein': entries,
    'conditions': 'same server, 60s, reproduce=false, nginx LTSV + MySQL all queries + CPU profiling + SQLite native PROFILE',
}
prefix.with_suffix('.json').write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n')
for typ, es in entries.items():
    for e in es:
        print('PPROTEIN_RESULT', typ, e['Status'], e['Message'], 'group=' + e['Snapshot']['GroupId'], flush=True)
print('RESULT_FILE', prefix.with_suffix('.json'), flush=True)
if exit_code or collection_error or not collected or not ready(entries):
    raise SystemExit(1)
