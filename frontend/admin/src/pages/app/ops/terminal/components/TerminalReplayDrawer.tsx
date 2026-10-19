import { useEffect, useRef } from 'react';
import { Drawer, Tag, Spin } from 'antd';
import { useTranslation } from 'react-i18next';
import { Terminal } from '@xterm/xterm';
import type { opsservicev1_TerminalSession as TerminalSession } from '@/api/generated/admin/service/v1';
import { useListTerminalChunks } from '@/api/hooks/terminal-audit';

interface Props {
  session?: TerminalSession;
  onClose: () => void;
}

/**
 * 终端会话回放抽屉:xterm 只读实例按 seq 顺序回放 OUTPUT 分块
 * (原始字节回灌,GBK 回显保持原样)。
 */
const TerminalReplayDrawer = ({ session, onClose }: Props) => {
  const { t } = useTranslation('terminal-audit');
  const hostRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<Terminal | null>(null);

  const { data: chunks, isFetching } = useListTerminalChunks(session?.id);

  // 会话切换时建实例并回灌
  useEffect(() => {
    if (!session || !hostRef.current) return;
    const term = new Terminal({
      disableStdin: true,
      convertEol: false,
      fontSize: 12,
      theme: { background: '#111827' },
    });
    term.open(hostRef.current);
    termRef.current = term;
    term.writeln(`\x1b[33m${t('replayBanner', {
      node: session.agentId,
      time: (session.createdAt || '').replace('T', ' ').slice(0, 19),
    })}\x1b[0m`);

    // 输入块以暗色标注(与回显区分),输出块按原始字节回灌。
    // 生成客户端把 proto bytes 产出为 base64 字符串,先解码回字节
    // (Windows GBK 回显以原始字节保真)。
    const toBytes = (v: unknown): Uint8Array => {
      if (v instanceof Uint8Array) return v;
      if (typeof v === 'string' && v.length > 0) {
        const bin = atob(v);
        const bytes = new Uint8Array(bin.length);
        for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
        return bytes;
      }
      return new Uint8Array();
    };
    let lastSeq = -1;
    for (const chunk of chunks?.items || []) {
      if ((chunk.seq ?? 0) <= lastSeq) continue;
      lastSeq = chunk.seq ?? 0;
      const raw = toBytes(chunk.payload);
      if (chunk.direction === 'INPUT') {
        const text = new TextDecoder().decode(raw);
        if (text.trim()) term.writeln(`\x1b[90m[cmd] ${text.replace(/\r?\n/g, ' ').trim()}\x1b[0m`);
        continue;
      }
      term.write(raw);
    }

    return () => {
      term.dispose();
      termRef.current = null;
    };
  }, [session, chunks]);

  return (
    <Drawer
      title={t('replayTitle', { id: session?.id })}
      open={!!session}
      onClose={onClose}
      size={860}
      destroyOnHidden
    >
      {isFetching ? (
        <div style={{ textAlign: 'center', padding: 48 }}><Spin /></div>
      ) : (
        <>
          <div style={{ marginBottom: 8, fontSize: 12, color: 'var(--ant-color-text-secondary)' }}>
            {t('replayHint')}
            {session?.truncated ? <Tag color="warning" style={{ marginLeft: 8 }}>{t('truncated')}</Tag> : null}
          </div>
          <div
            ref={hostRef}
            style={{ background: '#111827', padding: 8, borderRadius: 8, minHeight: 320 }}
          />
        </>
      )}
    </Drawer>
  );
};

export default TerminalReplayDrawer;
