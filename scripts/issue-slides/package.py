#!/usr/bin/env python3
"""Package reviewed imagegen outputs without putting binary assets into git."""
import argparse, hashlib, json, pathlib, shutil

p=argparse.ArgumentParser()
p.add_argument('prompts',help='JSON array of number, page, title, prompt')
p.add_argument('directory',help='Reviewed issue-N-PP.png files; must be gitignored')
p.add_argument('--issues',nargs='*',type=int)
args=p.parse_args()
tasks=json.loads(pathlib.Path(args.prompts).read_text())
root=pathlib.Path(args.directory).resolve()
groups={}
for task in tasks:
    n=task['number']; page=task['page']
    if args.issues and n not in args.issues: continue
    source=root/f'issue-{n}-{page:02}.png'
    if not source.is_file(): raise SystemExit(f'Missing reviewed image: {source}')
    digest=hashlib.sha256(source.read_bytes()).hexdigest()[:16]
    name=f'issue-{n}-{page:02}-{digest}.png'
    shutil.copyfile(source,root/name)
    group=groups.setdefault(n,{'number':n,'title':f'Issue #{n}','images':[]})
    group['images'].append({'title':task['title'],'file':str(root/name),'name':name})
(root/'manifest.json').write_text(json.dumps(list(groups.values()),ensure_ascii=False,indent=2))
(root/'spec.json').write_text(json.dumps([t for t in tasks if t['number'] in groups],ensure_ascii=False,indent=2))
print(f'{len(groups)} issues, {sum(len(g["images"]) for g in groups.values())} reviewed images packaged')
