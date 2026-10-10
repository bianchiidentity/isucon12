import fs from 'node:fs/promises';
import path from 'node:path';
import {spawn} from 'node:child_process';
import {createHash} from 'node:crypto';

// Dependency-free Chrome DevTools renderer. Generated files never enter git.
const specPath = process.argv[2];
if (!specPath) throw Error('Usage: node scripts/issue-slides/render.mjs SPEC.json');
const spec = JSON.parse(await fs.readFile(specPath, 'utf8'));
const out = path.resolve('.issue-slides/rendered');
await fs.mkdir(out, {recursive:true});
const chrome = process.env.CHROME_PATH || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const child = spawn(chrome, ['--headless', '--disable-gpu', '--no-first-run', '--remote-debugging-port=0', `--user-data-dir=${path.resolve('.issue-slides/chrome')}`, 'about:blank'], {stdio:['ignore','ignore','pipe']});
let ws;
try {
  const endpoint = await new Promise((resolve,reject) => {
    let buf=''; const timer=setTimeout(()=>reject(Error('Chrome startup timeout')),30000);
    child.on('error',reject); child.stderr.on('data', d=>{buf+=d;const m=buf.match(/DevTools listening on (ws:\/\/[^\s]+)/);if(m){clearTimeout(timer);resolve(m[1]);}});
    child.on('exit',code=>reject(Error(`Chrome exited ${code}`)));
  });
  ws = new WebSocket(endpoint); await new Promise((r,j)=>{ws.onopen=r;ws.onerror=j;});
  let seq=0;const pending=new Map();
  ws.onmessage=e=>{const m=JSON.parse(e.data);if(pending.has(m.id)){const [r,j,t]=pending.get(m.id);clearTimeout(t);pending.delete(m.id);m.error?j(Error(JSON.stringify(m.error))):r(m.result);}};
  const send=(method,params={},sessionId)=>new Promise((r,j)=>{const id=++seq;const t=setTimeout(()=>{pending.delete(id);j(Error(`CDP timeout: ${method}`));},30000);pending.set(id,[r,j,t]);ws.send(JSON.stringify({id,method,params,sessionId}));});
  const {targetId}=await send('Target.createTarget',{url:'about:blank'});
  const {sessionId}=await send('Target.attachToTarget',{targetId,flatten:true});
  const call=(m,p)=>send(m,p,sessionId);
  await call('Emulation.setDeviceMetricsOverride',{width:1672,height:941,deviceScaleFactor:1,mobile:false});
  const manifest=[];
  for (const issue of spec) {
    const images=[];
    for (const [index,slide] of issue.slides.entries()) {
      const html=render(issue,slide,index);
      const hash=createHash('sha256').update(html).digest('hex').slice(0,16);
      const name=`issue-${issue.number}-${String(index+1).padStart(2,'0')}-${hash}.png`;
      const filename=path.join(out,name);
      await call('Runtime.evaluate',{expression:`document.open();document.write(${JSON.stringify(html)});document.close();`});
      await call('Runtime.evaluate',{expression:'document.fonts.ready',awaitPromise:true});
      const check=await call('Runtime.evaluate',{expression:`JSON.stringify([...document.querySelectorAll('.card,.content,.lead,h1,.note')].filter(e=>e.scrollHeight>e.clientHeight+2||e.scrollWidth>e.clientWidth+2).map(e=>({class:e.className,text:e.textContent.slice(0,80)})))`,returnByValue:true});
      const overflow=JSON.parse(check.result.value);
      if(overflow.length) throw Error(`#${issue.number} slide ${index+1} overflows: ${JSON.stringify(overflow)}`);
      const shot=await call('Page.captureScreenshot',{format:'png',captureBeyondViewport:false});
      await fs.writeFile(filename,Buffer.from(shot.data,'base64'));
      await fs.writeFile(filename.replace('.png','.html'),html);
      images.push({title:slide.title,file:filename,name});
    }
    manifest.push({number:issue.number,title:issue.title,images});
    console.log(`#${issue.number}: ${images.length} slides`);
  }
  await fs.copyFile(specPath,path.join(out,'spec.json'));
  await fs.writeFile(path.join(out,'manifest.json'),JSON.stringify(manifest,null,2));
} finally {ws?.close();child.kill();}

function esc(s){return String(s??'').replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;').replaceAll('"','&quot;');}
function render(issue,s,index){
 const cards=s.cards.map(c=>`<section class="card"><h2>${esc(c.label)}</h2>${c.code?`<pre>${esc(c.code)}</pre>`:c.flow?`<div class="flow">${c.flow.map((x,i)=>`${i?'<div class="arrow">↓</div>':''}<div class="step">${esc(x)}</div>`).join('')}</div>`:c.diagram?`<pre class="diagram">${esc(c.diagram)}</pre>`:`<div class="prose">${(c.items||[c.text]).filter(Boolean).map(x=>`<p>${esc(x)}</p>`).join('')}</div>`}</section>`).join('');
 return `<!doctype html><html lang="ja"><meta charset="utf-8"><style>
 *{box-sizing:border-box}body{margin:0;background:#f5f8fc;color:#172b48;font-family:"Hiragino Sans","Noto Sans CJK JP",sans-serif;width:1672px;height:941px;padding:48px 64px;overflow:hidden}
 .eyebrow{color:#426c91;font-size:20px;letter-spacing:1px;margin-bottom:16px}h1{font-size:42px;line-height:1.4;margin:0;height:65px;overflow:hidden} .lead{font-size:25px;line-height:1.6;height:84px;margin:14px 0 22px;overflow:hidden}
 .content{display:grid;grid-template-columns:repeat(${s.cards.length},1fr);gap:24px;height:530px;overflow:hidden}.card{background:white;border:1px solid #dce6ef;border-top:6px solid #147b90;border-radius:16px;padding:26px;overflow:hidden}h2{font-size:24px;margin:0 0 22px;color:#146a83}.prose{font-size:25px;line-height:1.7;overflow-wrap:anywhere}.prose p{margin:0 0 20px}pre{font-family:"SFMono-Regular",Menlo,"Hiragino Sans",monospace;font-size:20px;line-height:1.65;white-space:pre-wrap;overflow-wrap:anywhere;margin:0}.diagram{font-size:24px;line-height:1.8}.step{font-size:25px;line-height:1.5;padding:16px;background:#edf5fa;border-radius:10px;white-space:pre-wrap}.arrow{text-align:center;font-size:27px;color:#168095;height:40px}.note{margin-top:22px;font-size:21px;line-height:1.6;height:68px;overflow:hidden}footer{position:absolute;bottom:22px;left:64px;right:64px;color:#587085;font-size:16px;display:flex;justify-content:space-between}
 </style><div class="eyebrow">ISUCON12 / ISSUE #${issue.number} / TECHNICAL NOTES</div><h1>${esc(s.title)}</h1><div class="lead">${esc(s.lead)}</div><main class="content">${cards}</main><div class="note">${esc(s.note||'')}</div><footer><span>${esc(issue.title)}</span><span>${index+1} / ${issue.slides.length}</span></footer></html>`;
}
