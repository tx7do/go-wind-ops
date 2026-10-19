// 冒烟脚本:在页面上下文执行。开终端会话→敲命令→关闭→验证录制,然后巡检/日志快照/配置中心。
const TOKEN_LOGIN = JSON.stringify({ username: 'admin', password: 'h1EAUNtwVm48wiz7iW1sjw==' });

async function smoke() {
  const login = await fetch('/admin/v1/login', {
    method: 'POST', headers: { 'Content-Type': 'application/json' }, body: TOKEN_LOGIN,
  });
  const token = (await login.json()).access_token;
  const H = { 'Content-Type': 'application/json', Authorization: 'Bearer ' + token };
  const out = { tokenLen: token.length };

  // 找 MSI 节点的 agentId
  const nodes = await (await fetch('/admin/v1/agent-nodes?page=1&pageSize=10', { headers: H })).json();
  const node = (nodes.items || [])[0];
  if (!node) return { err: 'no node' };
  out.agentId = node.agentId;

  // ── 1. 终端录制:开 WS 会话、敲两条命令、关闭 ──
  const wsResult = await new Promise((resolve) => {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    const ws = new WebSocket(`${proto}://${location.host}/admin/v1/ws/terminal?token=${encodeURIComponent(token)}&agentId=${encodeURIComponent(node.agentId)}`);
    ws.binaryType = 'arraybuffer';
    let gotOpened = false;
    let outputBytes = 0;
    const timer = setTimeout(() => resolve({ timeout: true, gotOpened, outputBytes }), 12000);
    ws.onmessage = (ev) => {
      if (typeof ev.data === 'string') {
        const msg = JSON.parse(ev.data);
        if (msg.event === 'OPENED' && !gotOpened) {
          gotOpened = true;
          // 敲两条命令(cmd 行尾 \r,agent 侧会压成 \n)
          const enc = new TextEncoder();
          ws.send(enc.encode('echo terminal-audit-smoke\r'));
          setTimeout(() => ws.send(enc.encode('dir\r')), 800);
          setTimeout(() => ws.close(), 2600);
        }
      } else {
        outputBytes += (ev.data || {}).byteLength || 0;
      }
    };
    ws.onclose = () => {
      clearTimeout(timer);
      resolve({ gotOpened, outputBytes });
    };
    ws.onerror = () => resolve({ wsError: true, gotOpened, outputBytes });
  });
  out.terminal = wsResult;
  await new Promise((r) => setTimeout(r, 2500)); // 等录制器 Close 落库

  // 查会话审计列表
  const sessions = await (await fetch('/admin/v1/terminal-sessions?page=1&pageSize=5', { headers: H })).json();
  out.sessions = (sessions.items || []).map((s) => ({
    id: s.id, agent: (s.agentId || '').slice(0, 8), status: s.status,
    in: s.inputBytes, outB: s.outputBytes, chunks: s.chunkCount,
  }));

  // 取最新会话的分块(验证回放数据)
  const latest = (sessions.items || [])[0];
  if (latest && latest.chunkCount > 0) {
    const chunks = await (await fetch('/admin/v1/terminal-sessions/chunks:search', {
      method: 'POST', headers: H, body: JSON.stringify({ sessionId: latest.id, limit: 500 }),
    })).json();
    const items = chunks.items || [];
    let replayText = '';
    for (const c of items) {
      if (c.direction !== 'OUTPUT') continue;
      const bytes = c.payload instanceof Uint8Array ? c.payload : new TextEncoder().encode(String(c.payload || ''));
      replayText += new TextDecoder().decode(bytes);
    }
    out.replayHasEcho = replayText.includes('terminal-audit-smoke');
    out.replayLen = replayText.length;
    out.chunkTotal = items.length;
  }

  // ── 2. 巡检 ──
  let r = await fetch('/admin/v1/inspections/run:send', { method: 'POST', headers: H, body: '{}' });
  const insp = await r.json();
  out.inspectionId = insp.id;
  await new Promise((res) => setTimeout(res, 2500));
  r = await fetch(`/admin/v1/inspections/${insp.id}`, { headers: H });
  const inspDetail = await r.json();
  out.inspection = { status: inspDetail.status, nodes: `${inspDetail.nodeOnline}/${inspDetail.nodeTotal}`, projects: `${inspDetail.projectRunning}/${inspDetail.projectTotal}`, alerts: inspDetail.alertOpen, certs: inspDetail.certExpiring, hasSummary: !!inspDetail.summary };

  // ── 3. 日志快照:先启动 smoke-demo 项目产生日志,再采集+检索 ──
  r = await fetch('/admin/v1/projects/1/operations:send', { method: 'POST', headers: H, body: JSON.stringify({ id: 1, action: 'START' }) });
  out.projectStart = r.status;
  await new Promise((res) => setTimeout(res, 3000));
  r = await fetch('/admin/v1/log-snapshots/capture:send', { method: 'POST', headers: H, body: JSON.stringify({ projectId: 1 }) });
  const snap = await r.json();
  out.snapshot = { id: snap.id, size: snap.sizeBytes, hasContent: !!snap.content };
  r = await fetch('/admin/v1/log-snapshots/search:send', { method: 'POST', headers: H, body: JSON.stringify({ keyword: (snap.content || '').split('\n')[0].slice(0, 12) || 'ops' }) });
  const search = await r.json();
  out.searchHits = (search.hits || []).length;

  // ── 4. 配置中心:建模板 → 下发 → 验证记录 SUCCESS ──
  r = await fetch('/admin/v1/config-templates', {
    method: 'POST', headers: H,
    body: JSON.stringify({ data: { name: 'smoke-app-conf', env: 'dev', targetPath: 'D:\\GoProject\\.go-wind-ops-trim-backup\\smoke\\repo-demo\\app.conf', content: '# smoke config\nmode=audit\n', description: 'tier3 smoke' } }),
  });
  const tpl = await r.json();
  out.createTpl = r.status;
  r = await fetch('/admin/v1/config-templates/distribute:send', { method: 'POST', headers: H, body: JSON.stringify({ templateId: tpl.id, nodeId: node.id }) });
  const dist = await r.json();
  out.distribute = { status: r.status, distStatus: dist.status, err: dist.error };
  await new Promise((res) => setTimeout(res, 2500));
  r = await fetch('/admin/v1/config-templates/distributions:search', { method: 'POST', headers: H, body: JSON.stringify({ templateId: tpl.id, page: 1, pageSize: 5 }) });
  const dists = await r.json();
  out.distRecords = (dists.items || []).map((d) => ({ status: d.status, err: (d.error || '').slice(0, 50) }));

  return out;
}

return smoke();