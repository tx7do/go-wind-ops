import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { App, Button, Select, Space, Tag, Typography } from 'antd';
import { PoweroffOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_TerminalSession } from '@/api/generated/admin/service/v1';
import { HistoryOutlined } from '@ant-design/icons';
import TerminalSessionsDrawer from './components/TerminalSessionsDrawer';
import TerminalReplayDrawer from './components/TerminalReplayDrawer';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';
import { useQuery } from '@tanstack/react-query';
import { PaginationQuery } from '@/core';
import { fetchListAgentNodes } from '@/api/hooks/agent-node';
import { useAuthStore } from '@/stores/auth';

/** 构造终端 WS 地址(经 dev 代理或网关同源转发)。 */
function buildWsUrl(token: string, agentId: string): string {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  return `${proto}://${location.host}/admin/v1/ws/terminal?token=${encodeURIComponent(token)}&agentId=${encodeURIComponent(agentId)}`;
}

const TerminalPage = () => {
  const { t } = useTranslation('terminal');
  const { message } = App.useApp();

  const [agentId, setAgentId] = useState<string>();
  const [auditOpen, setAuditOpen] = useState(false);
  const [replaySession, setReplaySession] = useState<opsservicev1_TerminalSession | undefined>();
  const [connected, setConnected] = useState(false);
  const [connecting, setConnecting] = useState(false);

  const hostRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<Terminal | null>(null);
  const fitRef = useRef<FitAddon | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  const { data: nodesResp } = useQuery({
    queryKey: ['listAgentNodesForTerminal', 'all'],
    queryFn: () => fetchListAgentNodes(new PaginationQuery({ paging: { page: 1, pageSize: 200 } })),
  });
  const agentOptions = useMemo(
    () =>
      (nodesResp?.items || []).map((n) => ({
        label: n.hostname ? `${n.hostname} (${n.agentId})` : n.agentId,
        value: n.agentId || '',
      })),
    [nodesResp],
  );

  /** 建立 xterm 实例(懒,首次连接时创建)。 */
  const ensureTerm = useCallback(() => {
    if (termRef.current) return termRef.current;
    const term = new Terminal({
      fontFamily: 'Consolas, Menlo, monospace',
      fontSize: 13,
      cursorBlink: true,
      convertEol: false,
      theme: {
        background: '#141414',
      },
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    if (hostRef.current) {
      term.open(hostRef.current);
      fit.fit();
    }
    termRef.current = term;
    fitRef.current = fit;
    term.onData((d) => {
      wsRef.current?.send(new TextEncoder().encode(d));
    });
    // 尺寸变化 → 后端同步调整 PTY(真 PTY 模式下行宽/交互程序依赖它)
    term.onResize(({ cols, rows }) => {
      const ws = wsRef.current;
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ event: 'RESIZE', cols, rows }));
      }
    });
    return term;
  }, []);

  const disconnect = useCallback(() => {
    if (reconnectTimer.current) clearTimeout(reconnectTimer.current);
    const ws = wsRef.current;
    wsRef.current = null;
    if (ws) {
      ws.onclose = null; // 主动断开不提示
      ws.close();
    }
    setConnected(false);
    termRef.current?.writeln('');
    termRef.current?.writeln(`\x1b[33m[${t('disconnected')}]\x1b[0m`);
  }, [t]);

  const connect = useCallback(() => {
    if (!agentId) {
      message.warning(t('pickAgentFirst'));
      return;
    }
    const token = useAuthStore.getState().accessToken;
    if (!token) {
      message.error(t('tokenMissing'));
      return;
    }
    const term = ensureTerm();
    term.reset();
    term.writeln(`\x1b[36m${t('connecting')} ${agentId} ...\x1b[0m`);
    setConnecting(true);

    // 本次连接是否已收到输出字节(OPENED 清"正在连接"行时避免误伤回显)。
    let gotOutput = false;

    /** 把当前 xterm 尺寸推给后端(刚连上与会话就绪时各推一次,其后走 onResize)。 */
    const sendResize = () => {
      const ws = wsRef.current;
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ event: 'RESIZE', cols: term.cols, rows: term.rows }));
      }
    };

    const ws = new WebSocket(buildWsUrl(token, agentId));
    ws.binaryType = 'arraybuffer';
    wsRef.current = ws;

    ws.onopen = () => {
      setConnected(true);
      setConnecting(false);
      sendResize();
    };
    ws.onmessage = (ev) => {
      if (typeof ev.data === 'string') {
        // 控制事件帧
        try {
          const msg = JSON.parse(ev.data) as { event?: string; error?: string };
          if (msg.event === 'OPENED') {
            // 会话已开:把"正在连接"行冲成绿色就绪提示(shell 横幅先到则不动)。
            if (!gotOutput) {
              term.write('\x1b[A\r\x1b[2K');
              term.writeln(`\x1b[32m[${t('connected')}]\x1b[0m`);
            }
            sendResize();
          } else if (msg.event === 'CLOSED') {
            term.writeln('');
            term.writeln(`\x1b[31m[${t('sessionClosed')}${msg.error ? ': ' + msg.error : ''}]\x1b[0m`);
            setConnected(false);
          }
        } catch {
          /* 非 JSON 忽略 */
        }
        return;
      }
      gotOutput = true;
      term.write(new Uint8Array(ev.data as ArrayBuffer));
    };
    ws.onerror = () => {
      term.writeln(`\x1b[31m[${t('wsError')}]\x1b[0m`);
    };
    ws.onclose = () => {
      setConnected(false);
      setConnecting(false);
      if (wsRef.current === ws) {
        wsRef.current = null;
        term.writeln(`\x1b[31m[${t('wsClosed')}]\x1b[0m`);
      }
    };
  }, [agentId, ensureTerm, message, t]);

  // 容器尺寸自适应
  useEffect(() => {
    if (!hostRef.current) return;
    const ro = new ResizeObserver(() => fitRef.current?.fit());
    ro.observe(hostRef.current);
    return () => ro.disconnect();
  }, []);

  // 卸载清理
  useEffect(
    () => () => {
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current);
      wsRef.current?.close();
      termRef.current?.dispose();
      termRef.current = null;
      fitRef.current = null;
    },
    [],
  );

  return (
    <div style={{ padding: 16, height: '100%', display: 'flex', flexDirection: 'column' }}>
      <Space wrap style={{ marginBottom: 12 }}>
        <Select
          showSearch
          optionFilterProp="label"
          placeholder={t('selectAgent')}
          style={{ minWidth: 260 }}
          value={agentId}
          options={agentOptions}
          onChange={(v) => {
            if (connected) disconnect();
            setAgentId(v);
          }}
        />
        {connected ? (
          <Button danger onClick={disconnect} icon={<PoweroffOutlined />}>
            {t('disconnect')}
          </Button>
        ) : (
          <Button type="primary" loading={connecting} onClick={connect} icon={<PoweroffOutlined />}>
            {t('connect')}
          </Button>
        )}
        {connected && <Tag color="success">{t('connected')}</Tag>}
        <Button icon={<HistoryOutlined />} onClick={() => setAuditOpen(true)}>
          {t('auditEntry')}
        </Button>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {t('hint')}
        </Typography.Text>
      </Space>
      <div
        ref={hostRef}
        style={{
          flex: 1,
          minHeight: 420,
          borderRadius: 8,
          padding: 8,
          background: '#141414',
        }}
      />

      <TerminalSessionsDrawer
        open={auditOpen}
        onClose={() => setAuditOpen(false)}
        onReplay={(session) => setReplaySession(session)}
      />
      <TerminalReplayDrawer
        session={replaySession}
        onClose={() => setReplaySession(undefined)}
      />
    </div>
  );
};

export default TerminalPage;
