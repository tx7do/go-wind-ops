import { useCallback, useEffect, useRef, useState } from 'react';
import { Button, Drawer, Popconfirm, Spin, Switch, Tag, App } from 'antd';
import { ClearOutlined, ReloadOutlined } from '@ant-design/icons';
import { useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_Project as Project } from '@/api/generated/admin/service/v1';
import { useClearProjectLog, useGetProjectLog } from '@/api/hooks/project';
import { useAuthStore } from '@/stores/auth';

interface ProjectLogDrawerProps {
  data?: Project;
  onClose: () => void;
}

const LOG_VIEW_LIMIT = 64 * 1024;

/** proto bytes(TS 端为 base64 字符串)→ 原始字节。 */
function b64ToBytes(v: unknown): Uint8Array {
  if (v instanceof Uint8Array) return v;
  if (typeof v === 'string' && v.length > 0) {
    const bin = atob(v);
    const bytes = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
    return bytes;
  }
  return new Uint8Array();
}

/** 字节 → 展示文本:UTF-8 严格解码失败(Windows GBK 输出)回落 GBK。 */
function bytesToText(bytes: Uint8Array): string {
  try {
    return new TextDecoder('utf-8', { fatal: true }).decode(bytes);
  } catch {
    try {
      return new TextDecoder('gbk').decode(bytes);
    } catch {
      return new TextDecoder().decode(bytes);
    }
  }
}

function buildWsUrl(token: string, projectId: number, intervalMs: number): string {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  return `${proto}://${location.host}/admin/v1/ws/projects/log?token=${encodeURIComponent(token)}&projectId=${projectId}&interval=${intervalMs}`;
}

/**
 * 项目运行日志抽屉:手动刷新(5s 轮询)或 实时 tail(WS 增量推送)。
 * 内容以原始字节消费,UTF-8 失败回落 GBK 解码(Windows 控制台输出不乱码)。
 */
const ProjectLogDrawer: React.FC<ProjectLogDrawerProps> = ({ data, onClose }) => {
  const { t } = useTranslation('project');
  const queryClient = useQueryClient();
  const { message } = App.useApp();
  const preRef = useRef<HTMLPreElement>(null);

  const [autoScroll, setAutoScroll] = useState(true);
  const [live, setLive] = useState(false);
  const [liveText, setLiveText] = useState('');
  const [liveStatus, setLiveStatus] = useState<'idle' | 'connecting' | 'open'>('idle');
  const wsRef = useRef<WebSocket | null>(null);
  const bufRef = useRef<Uint8Array>(new Uint8Array());
  const resetRef = useRef(false);

  const projectId = data?.id;
  const appendLive = useCallback((chunk: Uint8Array) => {
    const merged = new Uint8Array(bufRef.current.length + chunk.length);
    merged.set(bufRef.current, 0);
    merged.set(chunk, bufRef.current.length);
    // 上限保护:超过 512KB 丢头部
    bufRef.current = merged.length > 512 * 1024 ? merged.slice(merged.length - 512 * 1024) : merged;
    setLiveText(bytesToText(bufRef.current));
  }, []);

  // ── 实时 WS 生命周期 ──
  useEffect(() => {
    if (!live || !projectId) return;
    const token = useAuthStore.getState().accessToken;
    if (!token) {
      message.error(t('tokenMissing'));
      setLive(false);
      return;
    }
    bufRef.current = new Uint8Array();
    setLiveText('');
    setLiveStatus('connecting');
    const ws = new WebSocket(buildWsUrl(token, projectId, 2000));
    ws.binaryType = 'arraybuffer';
    wsRef.current = ws;
    ws.onopen = () => setLiveStatus('open');
    ws.onmessage = (ev) => {
      if (typeof ev.data === 'string') {
        try {
          const msg = JSON.parse(ev.data) as { event?: string };
          if (msg.event === 'RESET') {
            resetRef.current = true;
            bufRef.current = new Uint8Array();
            setLiveText('');
          }
        } catch { /* 忽略非 JSON 帧 */ }
        return;
      }
      if (ev.data instanceof ArrayBuffer) {
        if (resetRef.current) {
          resetRef.current = false;
          bufRef.current = new Uint8Array();
        }
        appendLive(new Uint8Array(ev.data));
      }
    };
    ws.onclose = () => setLiveStatus('idle');
    return () => {
      ws.close();
      wsRef.current = null;
      setLiveStatus('idle');
    };
  }, [live, projectId, appendLive, message, t]);

  // 关抽屉时退出实时
  useEffect(() => {
    if (!data) setLive(false);
  }, [data]);

  const query = useGetProjectLog(
    { id: data?.id, maxBytes: LOG_VIEW_LIMIT },
    {
      enabled: !!data?.id && !live, // 实时模式下停轮询
      refetchInterval: 5000,
    },
  );

  const clearMutation = useClearProjectLog({
    onSuccess: () => {
      message.success(t('logClearSuccess'));
      bufRef.current = new Uint8Array();
      setLiveText('');
      queryClient.invalidateQueries({ queryKey: ['getProjectLog'] });
    },
    onError: (error: Error) => message.error(error.message || t('logClearFailed')),
  });

  useEffect(() => {
    if (autoScroll && preRef.current) {
      preRef.current.scrollTop = preRef.current.scrollHeight;
    }
  }, [live ? liveText : query.data?.content, autoScroll]);

  const content = live ? liveText : bytesToText(b64ToBytes(query.data?.contentB64)) || query.data?.content || '';
  const size = live ? bufRef.current.length : query.data?.size ?? 0;

  const statusTag =
    liveStatus === 'open' ? (
      <Tag color="success">{t('logLiveOn')}</Tag>
    ) : liveStatus === 'connecting' ? (
      <Tag color="processing">{t('logLiveConnecting')}</Tag>
    ) : null;

  return (
    <Drawer
      title={
        <span>
          {t('logTitle')}
          {data?.name && <Tag style={{ marginLeft: 8 }}>{data.name}</Tag>}
          <Tag>{t('logSize', { size })}</Tag>
          {statusTag}
        </span>
      }
      open={!!data}
      onClose={onClose}
      size={820}
      destroyOnHidden
      extra={
        <span>
          <Switch
            checkedChildren={t('logLive')}
            unCheckedChildren={t('logLive')}
            checked={live}
            onChange={setLive}
            style={{ marginRight: 12 }}
          />
          <Button
            size="small"
            icon={<ReloadOutlined />}
            style={{ marginRight: 8 }}
            onClick={() => query.refetch()}
            loading={query.isFetching}
            disabled={live}
          >
            {t('logRefresh')}
          </Button>
          <Popconfirm
            title={t('logClearConfirmTitle')}
            onConfirm={() => data?.id && clearMutation.mutate({ id: data.id })}
            okText={t('common:button.ok')}
            cancelText={t('common:button.cancel')}
          >
            <Button size="small" danger icon={<ClearOutlined />} loading={clearMutation.isPending}>
              {t('logClear')}
            </Button>
          </Popconfirm>
        </span>
      }
    >
      <div style={{ marginBottom: 8 }}>
        <label style={{ fontSize: 12, color: 'var(--ant-color-text-secondary)' }}>
          <input
            type="checkbox"
            checked={autoScroll}
            onChange={(e) => setAutoScroll(e.target.checked)}
            style={{ marginRight: 4 }}
          />
          {t('logAutoScroll')}
        </label>
      </div>
      {!live && query.isFetching && !query.data ? (
        <Spin style={{ display: 'block', margin: '40px auto' }} />
      ) : (
        <pre
          ref={preRef}
          style={{
            background: 'var(--ant-color-bg-layout, #f6f6f6)',
            border: '1px solid var(--ant-color-border)',
            borderRadius: 8,
            padding: 12,
            maxHeight: 'calc(100vh - 220px)',
            overflow: 'auto',
            fontSize: 12,
            lineHeight: 1.6,
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-all',
            margin: 0,
          }}
        >
          {content || t('logEmpty')}
        </pre>
      )}
    </Drawer>
  );
};

export default ProjectLogDrawer;
