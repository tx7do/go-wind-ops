import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { App, Button, Input, Modal, Select, Space, Tag, Tooltip, Typography } from 'antd';
import { ClearOutlined, HistoryOutlined, PoweroffOutlined, UploadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_TerminalSession } from '@/api/generated/admin/service/v1';
import TerminalSessionsDrawer from './components/TerminalSessionsDrawer';
import TerminalReplayDrawer from './components/TerminalReplayDrawer';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { WebLinksAddon } from '@xterm/addon-web-links';
import '@xterm/xterm/css/xterm.css';
import { useQuery } from '@tanstack/react-query';
import { PaginationQuery } from '@/core';
import { fetchListAgentNodes } from '@/api/hooks/agent-node';
import { uploadAgentFile, formatBytes, joinRemotePath } from '@/api/hooks/file';
import { useAuthStore } from '@/stores/auth';

/** 构造终端 WS 地址(经 dev 代理或网关同源转发)。 */
function buildWsUrl(token: string, agentId: string): string {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  return `${proto}://${location.host}/admin/v1/ws/terminal?token=${encodeURIComponent(token)}&agentId=${encodeURIComponent(agentId)}`;
}

// 记忆选择与字号(localStorage 持久化)
const LAST_AGENT_KEY = 'terminal:lastAgentId';
const FONT_SIZE_KEY = 'terminal:fontSize';
const LAST_UPLOAD_DIR_KEY = 'terminal:lastUploadDir';
// 网络断开自动重连次数上限(每次重连会在 agent 上开新会话,旧会话闲置 15 分钟自动回收)
const RECONNECT_MAX = 3;

const readFontSize = (): number => {
  const v = Number(localStorage.getItem(FONT_SIZE_KEY));
  return v >= 10 && v <= 24 ? v : 13;
};

const TerminalPage = () => {
  const { t } = useTranslation('terminal');
  const { message } = App.useApp();

  const [agentId, setAgentId] = useState<string>();
  const [auditOpen, setAuditOpen] = useState(false);
  const [replaySession, setReplaySession] = useState<opsservicev1_TerminalSession | undefined>();
  const [connected, setConnected] = useState(false);
  const [connecting, setConnecting] = useState(false);
  const [fontSize, setFontSize] = useState<number>(readFontSize);
  const [title, setTitle] = useState('');
  // 拖拽/按钮上传:待确认文件列表 + 目标目录
  const [dropActive, setDropActive] = useState(false);
  const [uploadOpen, setUploadOpen] = useState(false);
  const [pendingFiles, setPendingFiles] = useState<File[]>([]);
  const [uploadDir, setUploadDir] = useState('');
  const [uploading, setUploading] = useState(false);

  const hostRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<Terminal | null>(null);
  const fitRef = useRef<FitAddon | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const uploadInputRef = useRef<HTMLInputElement>(null);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const openSessionRef = useRef<(agentId: string) => void>(() => {});
  const agentIdRef = useRef<string | undefined>(undefined);
  const attemptRef = useRef(0);
  const userAbortRef = useRef(false);
  const remoteClosedRef = useRef(false);

  const { data: nodesResp } = useQuery({
    queryKey: ['listAgentNodesForTerminal', 'all'],
    queryFn: () => fetchListAgentNodes(new PaginationQuery({ paging: { page: 1, pageSize: 200 } })),
  });
  const agentOptions = useMemo(
    () =>
      (nodesResp?.items || []).map((n) => {
        const offline = n.status === 'OFFLINE';
        const label = n.hostname ? `${n.hostname} (${n.agentId})` : (n.agentId || '');
        return {
          label: offline ? `${label} · ${t('offline')}` : label,
          value: n.agentId || '',
          disabled: offline,
        };
      }),
    [nodesResp, t],
  );

  // 首次加载自动选中:优先记忆的上次机器(在线),仅一台且在线时直接选它
  useEffect(() => {
    if (agentId || !nodesResp?.items?.length) return;
    const items = nodesResp.items;
    const last = localStorage.getItem(LAST_AGENT_KEY);
    const lastNode = last ? items.find((n) => n.agentId === last && n.status !== 'OFFLINE') : undefined;
    const single = items.length === 1 ? items[0] : undefined;
    const pick = lastNode || (single && single.status !== 'OFFLINE' ? single : undefined);
    if (pick?.agentId) setAgentId(pick.agentId);
  }, [agentId, nodesResp]);

  useEffect(() => {
    agentIdRef.current = agentId;
  }, [agentId]);

  /** 建立 xterm 实例(懒,首次连接时创建)。 */
  const ensureTerm = useCallback(() => {
    if (termRef.current) return termRef.current;
    const term = new Terminal({
      fontFamily: 'Consolas, Menlo, monospace',
      fontSize: readFontSize(),
      cursorBlink: true,
      convertEol: false,
      scrollback: 5000,
      theme: {
        background: '#141414',
      },
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.loadAddon(new WebLinksAddon());
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
    // PTY 窗口标题同步:显示当前前台命令(如 "ping -n 30 127.0.0.1");
    // 空闲时 ConPTY 上报 cmd.exe 全路径,视为无前台命令不展示。
    term.onTitleChange((rawTitle) => {
      let clean = rawTitle.replace(/^cmd\.exe\s*-\s*/i, '');
      if (/cmd\.exe$/i.test(clean)) clean = '';
      setTitle(clean.slice(0, 120));
    });
    // 剪贴板快捷键:Ctrl+Shift+C / Ctrl+Insert 复制选中,Ctrl+Shift+V / Shift+Insert 粘贴
    term.attachCustomKeyEventHandler((ev) => {
      if (ev.type !== 'keydown') return true;
      const copyHit = (ev.ctrlKey && ev.shiftKey && ev.key === 'C') || (ev.ctrlKey && ev.key === 'Insert');
      const pasteHit = (ev.ctrlKey && ev.shiftKey && ev.key === 'V') || (ev.shiftKey && ev.key === 'Insert');
      if (copyHit && term.hasSelection()) {
        navigator.clipboard?.writeText(term.getSelection()).catch(() => {});
        return false;
      }
      if (pasteHit) {
        navigator.clipboard
          ?.readText()
          .then((txt) => {
            if (txt) term.paste(txt);
          })
          .catch(() => {});
        return false;
      }
      return true;
    });
    return term;
  }, []);

  // 右键快捷(PuTTY/Windows Terminal 习惯):有选中=复制,否则=粘贴
  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;
    const onContextMenu = (e: MouseEvent) => {
      e.preventDefault();
      const term = termRef.current;
      if (!term) return;
      if (term.hasSelection()) {
        navigator.clipboard?.writeText(term.getSelection()).catch(() => {});
        term.clearSelection();
        return;
      }
      navigator.clipboard
        ?.readText()
        .then((txt) => {
          if (txt) term.paste(txt);
        })
        .catch(() => {});
    };
    host.addEventListener('contextmenu', onContextMenu);
    return () => host.removeEventListener('contextmenu', onContextMenu);
  }, []);

  // Ctrl+滚轮调字号(10-24),持久化;fit 触发 onResize → 自动同步 PTY
  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;
    const onWheel = (e: WheelEvent) => {
      if (!e.ctrlKey) return;
      e.preventDefault();
      setFontSize((v) => Math.min(24, Math.max(10, v + (e.deltaY < 0 ? 1 : -1))));
    };
    host.addEventListener('wheel', onWheel, { passive: false });
    return () => host.removeEventListener('wheel', onWheel);
  }, []);

  useEffect(() => {
    localStorage.setItem(FONT_SIZE_KEY, String(fontSize));
    const term = termRef.current;
    if (term) {
      term.options.fontSize = fontSize;
      fitRef.current?.fit();
    }
  }, [fontSize]);

  // 连接期间离开页面给一次挽留提示(终端会话将断开)
  useEffect(() => {
    if (!connected) return;
    const onBeforeUnload = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = '';
    };
    window.addEventListener('beforeunload', onBeforeUnload);
    return () => window.removeEventListener('beforeunload', onBeforeUnload);
  }, [connected]);

  // 拖文件进终端=上传到远端:dragover 放行 → drop 收集文件(跳过文件夹)→ 弹目录确认
  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;
    const hasFiles = (e: DragEvent) => Array.from(e.dataTransfer?.types || []).includes('Files');
    const onDragOver = (e: DragEvent) => {
      if (!hasFiles(e)) return;
      e.preventDefault();
      setDropActive(true);
    };
    const onDragLeave = (e: DragEvent) => {
      if (e.relatedTarget && host.contains(e.relatedTarget as Node)) return;
      setDropActive(false);
    };
    const onDrop = (e: DragEvent) => {
      if (!hasFiles(e)) return;
      e.preventDefault();
      setDropActive(false);
      const files: File[] = [];
      let dirs = 0;
      for (const item of Array.from(e.dataTransfer?.items || [])) {
        // 目录项第一版不支持(递归遍历留二期),提示打包
        const entry = (
          item as DataTransferItem & { webkitGetAsEntry?: () => { isDirectory: boolean } | null }
        ).webkitGetAsEntry?.();
        if (entry?.isDirectory) {
          dirs += 1;
          continue;
        }
        const f = item.getAsFile();
        if (f) files.push(f);
      }
      if (dirs > 0) message.warning(t('folderSkip'));
      if (!files.length) return;
      if (!connected) {
        message.warning(t('connectFirst'));
        return;
      }
      setPendingFiles(files);
      setUploadDir(localStorage.getItem(LAST_UPLOAD_DIR_KEY) || '');
      setUploadOpen(true);
    };
    host.addEventListener('dragover', onDragOver);
    host.addEventListener('dragleave', onDragLeave);
    host.addEventListener('drop', onDrop);
    return () => {
      host.removeEventListener('dragover', onDragOver);
      host.removeEventListener('dragleave', onDragLeave);
      host.removeEventListener('drop', onDrop);
    };
  }, [connected, message, t]);

  /** 执行上传并往终端写进度/结果行(进度覆写当前行,结果各自成行)。 */
  const runUploads = useCallback(
    async (files: File[], dir: string) => {
      const term = termRef.current;
      const agent = agentIdRef.current;
      if (!term || !agent) return;
      setUploading(true);
      let ok = 0;
      for (const f of files) {
        const target = joinRemotePath(dir, f.name);
        term.write(`\x1b[36m${t('uploadTermStart', { name: f.name, path: target })}\x1b[0m`);
        try {
          const done = await uploadAgentFile({
            agentId: agent,
            path: target,
            file: f,
            onProgress: (p) => {
              term.write(`\r\x1b[2K\x1b[36m${t('uploadTermProgress', { name: f.name, percent: p })}\x1b[0m`);
            },
          });
          term.write(
            `\r\x1b[2K\x1b[32m${t('uploadTermDone', { name: f.name, size: formatBytes(done.size ?? f.size) })}\x1b[0m\r\n`,
          );
          ok += 1;
        } catch (err) {
          term.write(
            `\r\x1b[2K\x1b[31m${t('uploadTermFail', { name: f.name, reason: (err as Error).message })}\x1b[0m\r\n`,
          );
        }
      }
      setUploading(false);
      if (ok === files.length) message.success(t('uploadAllDone', { n: ok }));
      else message.warning(t('uploadPartialDone', { ok, total: files.length }));
    },
    [message, t],
  );

  const confirmUpload = useCallback(() => {
    const dir = uploadDir.trim();
    if (!dir) {
      message.warning(t('uploadDirRequired'));
      return;
    }
    localStorage.setItem(LAST_UPLOAD_DIR_KEY, dir);
    const files = pendingFiles;
    setUploadOpen(false);
    setPendingFiles([]);
    void runUploads(files, dir);
  }, [uploadDir, pendingFiles, message, t, runUploads]);

  /** 打开上传确认弹层(按钮入口);拖拽入口在 drop 事件里。 */
  const openUploadDialog = useCallback(
    (files: File[]) => {
      if (!files.length) return;
      if (!connected) {
        message.warning(t('connectFirst'));
        return;
      }
      setPendingFiles(files);
      setUploadDir(localStorage.getItem(LAST_UPLOAD_DIR_KEY) || '');
      setUploadOpen(true);
    },
    [connected, message, t],
  );

  const disconnect = useCallback(() => {
    if (reconnectTimer.current) clearTimeout(reconnectTimer.current);
    userAbortRef.current = true;
    const ws = wsRef.current;
    wsRef.current = null;
    if (ws) {
      ws.onclose = null; // 主动断开不提示
      ws.close();
    }
    setConnected(false);
    setTitle('');
    termRef.current?.writeln('');
    termRef.current?.writeln(`\x1b[33m[${t('disconnected')}]\x1b[0m`);
  }, [t]);

  /** 建立 WS 会话(连接与自动重连共用;targetId 显式传入防切换竞态)。 */
  const openSession = useCallback(
    (targetId: string) => {
      const token = useAuthStore.getState().accessToken;
      if (!token) {
        message.error(t('tokenMissing'));
        return;
      }
      const term = ensureTerm();
      term.reset();
      term.writeln(`\x1b[36m${t('connecting')} ${targetId} ...\x1b[0m`);
      setConnecting(true);
      setTitle('');

      // 本次连接是否已收到输出字节(OPENED 清"正在连接"行时避免误伤回显)。
      let gotOutput = false;

      /** 把当前 xterm 尺寸推给后端(刚连上与会话就绪时各推一次,其后走 onResize)。 */
      const sendResize = () => {
        const ws = wsRef.current;
        if (ws && ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ event: 'RESIZE', cols: term.cols, rows: term.rows }));
        }
      };

      const ws = new WebSocket(buildWsUrl(token, targetId));
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
              attemptRef.current = 0; // 会话就绪,重连计数归零
              // 会话已开:把"正在连接"行冲成绿色就绪提示(shell 横幅先到则不动)。
              if (!gotOutput) {
                term.write('\x1b[A\r\x1b[2K');
                term.writeln(`\x1b[32m[${t('connected')}]\x1b[0m`);
              }
              sendResize();
            } else if (msg.event === 'CLOSED') {
              remoteClosedRef.current = true;
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
        if (wsRef.current !== ws) return;
        wsRef.current = null;
        // 主动断开/远端正常关闭/已切换机器:不自动重连
        if (userAbortRef.current || remoteClosedRef.current || agentIdRef.current !== targetId) return;
        if (attemptRef.current >= RECONNECT_MAX) {
          term.writeln(`\x1b[31m[${t('wsClosed')}]\x1b[0m`);
          return;
        }
        attemptRef.current += 1;
        const n = attemptRef.current;
        term.writeln(`\x1b[33m[${t('reconnecting', { n, max: RECONNECT_MAX })}]\x1b[0m`);
        reconnectTimer.current = setTimeout(() => openSessionRef.current(targetId), 1200 * n);
      };
    },
    [ensureTerm, message, t],
  );
  openSessionRef.current = openSession;

  const connect = useCallback(() => {
    if (!agentId) {
      message.warning(t('pickAgentFirst'));
      return;
    }
    userAbortRef.current = false;
    remoteClosedRef.current = false;
    attemptRef.current = 0;
    localStorage.setItem(LAST_AGENT_KEY, agentId);
    openSessionRef.current(agentId);
  }, [agentId, message, t]);

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
            localStorage.setItem(LAST_AGENT_KEY, v);
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
        <Button
          icon={<ClearOutlined />}
          disabled={!connected}
          onClick={() => {
            termRef.current?.clear();
          }}
        >
          {t('clearScreen')}
        </Button>
        <Button
          icon={<UploadOutlined />}
          disabled={!connected}
          onClick={() => uploadInputRef.current?.click()}
        >
          {t('uploadBtn')}
        </Button>
        <input
          ref={uploadInputRef}
          type="file"
          multiple
          hidden
          onChange={(e) => {
            const fs = Array.from(e.target.files || []);
            e.target.value = '';
            openUploadDialog(fs);
          }}
        />
        {connected && <Tag color="success">{t('connected')}</Tag>}
        {connected && title && (
          <Tooltip title={title}>
            <Typography.Text type="secondary" style={{ fontSize: 12, maxWidth: 320 }} ellipsis>
              {title}
            </Typography.Text>
          </Tooltip>
        )}
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
          position: 'relative',
          flex: 1,
          minHeight: 420,
          borderRadius: 8,
          padding: 8,
          background: '#141414',
        }}
      >
        {dropActive && (
          <div
            style={{
              position: 'absolute',
              inset: 8,
              borderRadius: 8,
              border: '2px dashed #1668dc',
              background: 'rgba(22, 104, 220, 0.15)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              zIndex: 10,
              pointerEvents: 'none',
              color: '#4096ff',
              fontSize: 16,
            }}
          >
            {t('dropHere')}
          </div>
        )}
      </div>

      <Modal
        title={t('uploadTitle')}
        open={uploadOpen}
        onCancel={() => {
          if (uploading) return;
          setUploadOpen(false);
          setPendingFiles([]);
        }}
        onOk={confirmUpload}
        okText={t('uploadStart')}
        cancelText={t('uploadCancel')}
        confirmLoading={uploading}
        okButtonProps={{ disabled: uploading }}
        cancelButtonProps={{ disabled: uploading }}
        maskClosable={!uploading}
        width={480}
      >
        <Typography.Paragraph type="secondary" style={{ marginBottom: 6, fontSize: 12 }}>
          {t('uploadTargetDir')}
        </Typography.Paragraph>
        <Input
          value={uploadDir}
          placeholder={t('uploadDirPlaceholder')}
          onChange={(e) => setUploadDir(e.target.value)}
          disabled={uploading}
        />
        <div style={{ marginTop: 8, maxHeight: 180, overflow: 'auto' }}>
          {pendingFiles.map((f, i) => (
            <div key={`${f.name}-${i}`} style={{ display: 'flex', justifyContent: 'space-between', gap: 12, padding: '2px 0' }}>
              <Typography.Text ellipsis style={{ fontSize: 12, maxWidth: 320 }}>
                {f.name}
              </Typography.Text>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                {formatBytes(f.size)}
              </Typography.Text>
            </div>
          ))}
        </div>
      </Modal>

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
