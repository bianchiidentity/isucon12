import { test, expect } from '@playwright/test';
import fs from 'node:fs/promises';
import path from 'node:path';

const domain=process.env.E2E_DOMAIN || 't.isucon.dev';
const port=process.env.E2E_HTTPS_PORT ? `:${process.env.E2E_HTTPS_PORT}` : '';
const origin=name=>`https://${name}.${domain}${port}`;
const uuid=/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
const uuidRequired=process.env.E2E_EXPECT_UUID !== '0';
const responseFor=(page,url,method='GET')=>page.waitForResponse(r=>new URL(r.url()).pathname===url && r.request().method()===method);
async function success(response) {
  expect(response.status(),await response.text()).toBe(200);
  const body=await response.json(); expect(body.status).toBe(true); return body.data;
}

test('画面でたどる大会運営：テナント作成から請求・失格まで',async ({browser},info)=>{
  const tenant=`e2e-${Date.now().toString(36)}`;
  const title='E2E サンプル大会';
  const contexts=[];
  const shots=[];
  const runDir=path.resolve('../.e2e-artifacts/runs',tenant);
  await fs.mkdir(runDir,{recursive:true});
  const manifest={tenant,startedAt:new Date().toISOString(),target:origin(tenant),status:'running',screens:shots};
  const saveManifest=()=>fs.writeFile(path.join(runDir,'manifest.json'),JSON.stringify(manifest,null,2));
  async function newPage(){
    const context=await browser.newContext({ignoreHTTPSErrors:true,viewport:{width:1440,height:900},locale:'ja-JP'});
    contexts.push(context); const page=await context.newPage();
    page.on('dialog',async dialog=>{ await info.attach('ブラウザの通知',{body:dialog.message(),contentType:'text/plain'}); await dialog.accept(); });
    return page;
  }
  async function capture(page,id,title,role,description){
    const file=path.join(runDir,`${id}.png`);
    await page.evaluate(()=>document.fonts.ready);
    await page.screenshot({path:file,animations:'disabled',clip:{x:220,y:0,width:1000,height:700}});
    shots.push({id,title,role,description,file:path.basename(file),url:page.url()});
    await info.attach(title,{path:file,contentType:'image/png'}); await saveManifest();
  }
  async function open(page,url,api){
    const response=responseFor(page,api); await page.goto(url); return success(await response);
  }
  async function playerLogin(page,id){
    await page.goto(origin(tenant));
    await page.getByLabel('アカウントID').fill(id);
    const response=responseFor(page,'/api/player/competitions');
    await page.getByRole('button',{name:'認証する'}).click();
    await success(await response); await expect(page).toHaveURL(/\/mypage$/);
  }
  let admin,organizer,viewer,scorer,players,competition;
  try {
    await test.step('01 SaaS管理者が新しい団体を作成する',async()=>{
      admin=await newPage(); await admin.goto(origin('admin'));
      await admin.getByRole('button',{name:'認証する'}).click();
      await expect(admin.getByRole('heading',{name:'テナント一覧'})).toBeVisible();
      await admin.getByRole('button',{name:'テナント追加'}).click();
      await admin.locator('[name=tenant_name]').fill(tenant);
      await admin.locator('[name=tenant_display_name]').fill('E2E 画面案内クラブ');
      const response=responseFor(admin,'/api/admin/tenants/add','POST');
      await admin.getByRole('button',{name:'作成',exact:true}).click();
      const data=await success(await response); expect(data.tenant.name).toBe(tenant);
      await expect(admin.getByRole('link',{name:tenant,exact:true})).toBeVisible();
      await capture(admin,'01-admin','団体を作る・団体ごとの請求を見る','SaaS管理者','新規テナントが一覧に現れる。各テナントは専用のホスト名を持つ。');
    });
    await test.step('02 新テナントで主催者モードに切り替える',async()=>{
      organizer=await newPage(); await organizer.goto(origin(tenant));
      await expect(organizer.getByRole('heading',{name:'プレイヤーサイトへログイン'})).toBeVisible();
      await capture(organizer,'02-login','同じ団体に、参加者と主催者の入口がある','共通','参加者はアカウントIDでログイン。主催者は右上の管理者モードへ切り替える。');
      await organizer.getByRole('button',{name:'テナント管理者モードに切り替える'}).click();
      await expect(organizer.getByRole('link',{name:'プレイヤー一覧'})).toBeVisible();
    });
    await test.step('03 選手を追加し、UUIDのIDが一覧に表示される',async()=>{
      const loaded=responseFor(organizer,'/api/organizer/players');
      await organizer.getByRole('link',{name:'プレイヤー一覧'}).click();
      expect((await success(await loaded)).players, '空の選手一覧はnullではなく[]を返す').toEqual([]);
      await organizer.getByRole('button',{name:'プレイヤー追加',exact:true}).click();
      for(const name of ['葵（選手A）','凛（選手B）','空（閲覧のみ）']){
        await organizer.locator('.modal-outer:visible input[type=text]').fill(name);
        await organizer.getByRole('button',{name:'リストに追加'}).click();
      }
      const response=responseFor(organizer,'/api/organizer/players/add','POST');
      await organizer.getByRole('button',{name:'作成',exact:true}).click();
      players=(await success(await response)).players;
      expect(players).toHaveLength(3);
      for(const player of players){ if(uuidRequired) expect(player.id).toMatch(uuid); await expect(organizer.getByRole('cell',{name:player.display_name,exact:true})).toBeVisible(); }
      await capture(organizer,'03-players','選手を登録し、参加資格を管理する','主催者','3人を作成。2人はスコア登録、1人は閲覧だけ。失格操作もこの一覧から行う。');
    });
    await test.step('04 大会を作り、CSV入稿の入口を確認する',async()=>{
      await open(organizer,`${origin(tenant)}/organizer/competitions`,'/api/organizer/competitions');
      await organizer.getByRole('button',{name:'大会作成'}).click();
      await organizer.locator('.modal-outer:visible [name=competition_name]').fill(title);
      const response=responseFor(organizer,'/api/organizer/competitions/add','POST');
      await organizer.getByRole('button',{name:'作成',exact:true}).click();
      competition=(await success(await response)).competition;
      if(uuidRequired) expect(competition.id).toMatch(uuid);
      await expect(organizer.getByRole('row').filter({hasText:title})).toContainText('開催中');
      await capture(organizer,'04-competition','大会を作る・CSVを入れる・終了する','主催者','大会一覧にCSV入稿と終了操作が並ぶ。開催中だけスコアを更新できる。');
    });
    const [a,b,v]=players;
    const scoreAPI=`/api/organizer/competition/${competition.id}/score`;
    const rankingAPI=`/api/player/competition/${competition.id}/ranking`;
    const rankingURL=`${origin(tenant)}/competition/${competition.id}`;
    const originalCSV=`player_id,score\n${a.id},100\n${b.id},80\n${a.id},80\n`;
    async function upload(csv,status=200,shot=false){
      await open(organizer,`${origin(tenant)}/organizer/competitions`,'/api/organizer/competitions');
      await organizer.getByRole('row').filter({hasText:title}).getByRole('button',{name:'CSV入稿'}).click();
      await organizer.locator('input[type=file]').setInputFiles({name:'scores.csv',mimeType:'text/csv',buffer:Buffer.from(csv)});
      if(shot) await capture(organizer,'05-upload','CSVの最後の行が、その選手の得点になる','主催者','この例はA=100、B=80、A=80の3行。採用結果はB=80、A=80。応答の入力行数は3。');
      const response=responseFor(organizer,scoreAPI,'POST');
      await organizer.getByRole('button',{name:'アップロード',exact:true}).click();
      const res=await response; expect(res.status(),await res.text()).toBe(status);
      if(status===200){ const data=await success(res); expect(data.rows).toBe(csv.trim().split('\n').length-1); }
    }
    async function ranks(expected){
      const data=await open(viewer,rankingURL,rankingAPI);
      expect(data.ranks.map(r=>[r.player_id,r.score])).toEqual(expected);
      expect(data.ranks.map(r=>r.rank)).toEqual(expected.map((_,i)=>i+1));
      await expect(viewer.getByRole('heading',{name:'スコアランキング'})).toBeVisible();
      const bodyRows=viewer.locator('table tbody tr').filter({has:viewer.locator('td a')});
      await expect(bodyRows).toHaveCount(expected.length);
      for(let i=0;i<expected.length;i++){
        const p=players.find(p=>p.id===expected[i][0]);
        await expect(bodyRows.nth(i)).toContainText(p.display_name);
        await expect(bodyRows.nth(i)).toContainText(String(expected[i][1]));
      }
      return data;
    }
    await test.step('05 重複選手・同点のCSVを入稿し、ランキングと個人戦績を見る',async()=>{
      await upload(originalCSV,200,true);
      viewer=await newPage(); await playerLogin(viewer,v.id);
      await expect(viewer.getByRole('link',{name:title,exact:true})).toBeVisible();
      await capture(viewer,'06-player-home','参加者は大会一覧から結果を見に行く','参加者','作成した大会が一覧に表示される。UUIDの大会IDを含むURLへ遷移する。');
      const response=responseFor(viewer,rankingAPI);
      await viewer.getByRole('link',{name:title,exact:true}).click(); await success(await response);
      await ranks([[b.id,80],[a.id,80]]);
      await capture(viewer,'07-ranking','同点なら、採用されたCSV行が早い選手が上位','参加者','凛は2行目、葵は3行目が採用されるため、同じ80点でも凛が1位。選手名から個人戦績へ進める。');
      const personal=responseFor(viewer,`/api/player/player/${a.id}`);
      await viewer.getByRole('link',{name:a.display_name,exact:true}).click();
      const data=await success(await personal); expect(data.scores).toEqual([{competition_title:title,score:80}]);
      await expect(viewer.getByRole('cell',{name:title,exact:true})).toBeVisible();
      await capture(viewer,'08-personal','選手ごとに、大会と得点を一覧で見る','参加者','個人戦績には大会名と採用されたスコアが表示される。高い方の100点は残らない。');
      scorer=await newPage(); await playerLogin(scorer,a.id);
      await open(scorer,rankingURL,rankingAPI); // A is both a scored player and a viewer.
    });
    await test.step('06 CSV差し替え・不正入力・空CSVでも結果の整合性を保つ',async()=>{
      await upload(`player_id,score\n${a.id},42\n`); await ranks([[a.id,42]]);
      await upload(`player_id,score\n${a.id},bad\n${a.id},999\n`,400); await ranks([[a.id,42]]);
      await upload('player_id,score\n'); await ranks([]);
      const empty=await open(viewer,`${origin(tenant)}/player/${a.id}`,`/api/player/player/${a.id}`); expect(empty.scores).toEqual([]);
      await upload(originalCSV); await ranks([[b.id,80],[a.id,80]]);
    });
    async function billing(){
      const data=await open(organizer,`${origin(tenant)}/organizer/billing`,'/api/organizer/billing');
      return data.reports.find(r=>r.competition_id===competition.id);
    }
    await test.step('07 繰り返し閲覧しても重複課金せず、終了後に210円になる',async()=>{
      for(let i=0;i<3;i++) await ranks([[b.id,80],[a.id,80]]);
      expect(await billing()).toMatchObject({player_count:0,visitor_count:0,billing_yen:0});
      await capture(organizer,'09-billing-open','開催中の大会は、請求人数も金額もゼロ','主催者','スコアや閲覧があっても、開催中の請求は0。終了操作の後に請求を確認する。');
      await open(organizer,`${origin(tenant)}/organizer/competitions`,'/api/organizer/competitions');
      const finished=responseFor(organizer,`/api/organizer/competition/${competition.id}/finish`,'POST');
      await organizer.getByRole('row').filter({hasText:title}).getByRole('button',{name:'完了にする'}).click();
      await success(await finished);
      await expect(organizer.getByRole('row').filter({hasText:title})).toContainText('大会完了');
      await expect(organizer.getByRole('button',{name:'CSV入稿'})).toHaveCount(0);
      const expected={player_count:2,visitor_count:1,billing_player_yen:200,billing_visitor_yen:10,billing_yen:210};
      expect(await billing()).toMatchObject(expected);
      await ranks([[b.id,80],[a.id,80]]); expect(await billing()).toMatchObject(expected);
      await capture(organizer,'10-billing-finished','スコア登録2人＋閲覧のみ1人で、210円','主催者','選手は1人100円、閲覧だけの参加者は1人10円。選手自身の閲覧と繰り返し閲覧は二重に数えない。');
      const tenants=await open(admin,`${origin('admin')}/admin/`,'/api/admin/tenants/billing');
      expect(tenants.tenants.find(t=>t.name===tenant)?.billing).toBe(210);
      await expect(admin.getByRole('row').filter({hasText:tenant})).toContainText('210円');
    });
    await test.step('08 失格にした選手は参加者向けAPIを利用できない',async()=>{
      await open(organizer,`${origin(tenant)}/organizer/players`,'/api/organizer/players');
      const row=organizer.getByRole('row').filter({hasText:a.display_name});
      const response=responseFor(organizer,`/api/organizer/player/${a.id}/disqualified`,'POST');
      await row.getByRole('button',{name:'失格にする'}).click(); await success(await response);
      await expect(row.getByRole('button',{name:'失格にする'})).toHaveCount(0);
      await expect(row).toContainText('失格');
      await capture(organizer,'11-disqualified','失格は選手のアクセス権を止める操作','主催者','葵を失格にする。既存の順位・請求とは別に、本人からの参加者向けアクセスが403になる。');
      for(const [url,api] of [
        [rankingURL,rankingAPI],
        [`${origin(tenant)}/player/${a.id}`,`/api/player/player/${a.id}`],
        [`${origin(tenant)}/mypage`,'/api/player/competitions'],
      ]){
        const denied=responseFor(scorer,api); await scorer.goto(url); expect((await denied).status()).toBe(403);
      }
    });
    manifest.status='passed';
  } catch(error){ manifest.status='failed'; manifest.error=error.message; throw error; }
  finally {
    manifest.finishedAt=new Date().toISOString(); await saveManifest();
    await info.attach('実行結果と画面一覧',{path:path.join(runDir,'manifest.json'),contentType:'application/json'});
    for(const context of contexts) await context.close();
    console.log(`画面・実行結果: ${runDir}`);
  }
});
