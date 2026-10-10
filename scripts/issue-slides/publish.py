#!/usr/bin/env python3
"""Publish immutable release assets and preserve complete original issue bodies."""
import argparse, hashlib, json, pathlib, subprocess, time

def run(*args):
    attempts=3 if args[:2]==('gh','api') else 1
    for attempt in range(attempts):
        try:
            return subprocess.check_output(args, text=True).strip()
        except subprocess.CalledProcessError:
            if attempt+1==attempts: raise
            time.sleep(2)

def api(endpoint, payload=None):
    args=['gh','api',endpoint]
    if payload is None:
        return json.loads(run(*args))
    p=subprocess.run(args+['--method','POST' if endpoint.endswith('/releases') else 'PATCH','--input','-'],input=json.dumps(payload),text=True,check=True,capture_output=True)
    return json.loads(p.stdout)

def original(body):
    marker='<!-- issue-slides:original -->'
    if marker in body:
        wrapped=body.split(marker,1)[1].split('<!-- /issue-slides:original -->',1)[0]
        return wrapped[1:-1] if wrapped.startswith('\n') and wrapped.endswith('\n') else wrapped
    return body

def main():
    p=argparse.ArgumentParser()
    p.add_argument('--repo',required=True);p.add_argument('--manifest',required=True)
    p.add_argument('--snapshot',required=True);p.add_argument('--publish',action='store_true')
    args=p.parse_args()
    manifest=json.loads(pathlib.Path(args.manifest).read_text())
    before={x['number']:x['body'] for x in json.loads(pathlib.Path(args.snapshot).read_text())}
    base='repos/'+args.repo
    tag='issue-diagrams'
    # Check every input and current body before any remote mutation.
    for issue in manifest:
        if issue['number'] not in before: raise RuntimeError('Missing snapshot')
        for image in issue['images']:
            if not pathlib.Path(image['file']).is_file(): raise RuntimeError('Missing image')
    if not args.publish:
        print(json.dumps({x['number']:len(x['images']) for x in manifest}));return
    current_bodies={}
    page=1
    while True:
        batch=api(f'{base}/issues?state=all&per_page=100&page={page}')
        current_bodies.update({x['number']:x['body'] for x in batch})
        if len(batch)<100: break
        page+=1
    for issue in manifest:
        n=issue['number']
        if current_bodies.get(n)!=before[n]: raise RuntimeError(f'Concurrent edit on #{n}; refresh snapshot and merge')
    print('Issue snapshots verified',flush=True)
    releases=api(base+'/releases?per_page=100')
    release=next((r for r in releases if r['tag_name']==tag),None)
    if release is None:
        release=api(base+'/releases',{'tag_name':tag,'target_commitish':run('git','rev-parse','HEAD'),'name':'Issue diagrams (documentation assets)','body':'Issue説明用の画像・再生成用データ。アプリケーションのリリースではありません。画像はGit履歴や通常のclone/pullに含まれません。','prerelease':True,'make_latest':'false'})
    assets={a['name']:a['browser_download_url'] for a in api(f"{base}/releases/{release['id']}/assets?per_page=100")}
    # paginate: repositories may contain more than 100 slides
    page=2
    while len(assets)>=(page-1)*100:
        more=api(f"{base}/releases/{release['id']}/assets?per_page=100&page={page}")
        if not more: break
        assets.update({a['name']:a['browser_download_url'] for a in more});page+=1
    paths=[im for x in manifest for im in x['images'] if im['name'] not in assets]
    for offset in range(0,len(paths),15):
        group=paths[offset:offset+15]
        run('gh','release','upload',tag,'--repo',args.repo,*[im['file'] for im in group])
        print(f'Uploaded {min(offset+15,len(paths))}/{len(paths)}',flush=True)
    spec=pathlib.Path(args.manifest).parent/'spec.json'
    if spec.exists():
        name='spec-'+hashlib.sha256(spec.read_bytes()).hexdigest()[:16]+'.json'
        copy=spec.with_name(name);copy.write_bytes(spec.read_bytes())
        if name not in assets: run('gh','release','upload',tag,'--repo',args.repo,str(copy))
    updated=[]
    for issue in manifest:
        n=issue['number']
        current=api(f'{base}/issues/{n}')['body']
        if current!=before[n]: raise RuntimeError(f'Concurrent edit on #{n}; assets uploaded, body unchanged')
        sections=[]
        for j,im in enumerate(issue['images'],1):
            url=f'https://github.com/{args.repo}/releases/download/{tag}/{im["name"]}'
            sections.append(f'## {j}. {im["title"]}\n\n![{im["title"]}]({url})')
        intro=issue.get('intro_md','説明を論点ごとに図解しました。画像はクリックで拡大できます。')
        after=issue.get('after_images_md','')
        body='<!-- issue-slides:v1 -->\n'+intro+'\n\n'+'\n\n'.join(sections)+'\n\n'+after+'\n\n---\n\n<details>\n<summary>元の説明・コピーできるコード・根拠資料</summary>\n\n<!-- issue-slides:original -->\n'+original(before[n])+'\n<!-- /issue-slides:original -->\n\n</details>\n<!-- /issue-slides:v1 -->\n'
        result=api(f'{base}/issues/{n}',{'body':body})
        if result['body']!=body or api(f'{base}/issues/{n}')['body']!=body: raise RuntimeError(f'Verification failed #{n}')
        updated.append(n);print(f'Updated and verified #{n}',flush=True)
    pathlib.Path(args.manifest).with_name('published.json').write_text(json.dumps(updated))

if __name__=='__main__': main()
