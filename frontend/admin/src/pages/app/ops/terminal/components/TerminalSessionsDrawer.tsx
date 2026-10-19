import { useEffect, useRef } from 'react';
import type { ProColumns, ActionType } from '@ant-design/pro-components';
import { Drawer, Tag, Button } from 'antd';
import { PlayCircleOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_TerminalSession as TerminalSession } from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { TABLE } from '@/config/constants';
import ListTable from '@/components/common/ListTable';
import { useProTableScrollY } from '@/hooks/useProTableScrollY';

interface Props {
  open: boolean;
  onClose: () => void;
  onReplay: (session: TerminalSession) => void;
}

/**
 * 终端会话审计列表:谁在什么时候在哪个节点开过终端、流量与录制状态。
 */
const TerminalSessionsDrawer = ({ open, onClose, onReplay }: Props) => {
  const { t } = useTranslation('terminal-audit');
  const actionRef = useRef<ActionType>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const tableScrollY = useProTableScrollY(containerRef);

  useEffect(() => {
    if (open) actionRef.current?.reload();
  }, [open]);

  const columns: ProColumns<TerminalSession>[] = [
    { title: t('openedAt'), dataIndex: 'createdAt', width: 165, valueType: 'dateTime' },
    { title: t('nodeId'), dataIndex: 'nodeId', width: 80 },
    { title: t('agentId'), dataIndex: 'agentId', width: 150, ellipsis: true },
    { title: t('operator'), dataIndex: 'operatorId', width: 80 },
    {
      title: t('status'),
      dataIndex: 'status',
      width: 100,
      render: (_, r) =>
        r.status === 'OPEN' ? (
          <Tag color="processing">{t('statusOpen')}</Tag>
        ) : (
          <Tag>{t('statusClosed')}</Tag>
        ),
    },
    {
      title: t('duration'),
      width: 110,
      render: (_, r) => {
        const end = r.closedAt ? new Date(r.closedAt).getTime() : Date.now();
        const start = r.createdAt ? new Date(r.createdAt).getTime() : end;
        const sec = Math.max(0, Math.round((end - start) / 1000));
        return sec >= 60 ? `${Math.floor(sec / 60)}m${sec % 60}s` : `${sec}s`;
      },
    },
    {
      title: t('traffic'),
      width: 150,
      render: (_, r) => `${fmtBytes(r.inputBytes)} ↑ / ${fmtBytes(r.outputBytes)} ↓`,
    },
    {
      title: t('chunks'),
      width: 110,
      render: (_, r) => (
        <>
          {r.chunkCount ?? 0}
          {r.truncated ? <Tag color="warning" style={{ marginLeft: 4 }}>{t('truncated')}</Tag> : null}
        </>
      ),
    },
    {
      title: t('action'),
      valueType: 'option',
      width: 90,
      render: (_, r) => [
        (r.chunkCount ?? 0) > 0 ? (
          <Button
            key="replay"
            size="small"
            type="link"
            icon={<PlayCircleOutlined />}
            onClick={() => onReplay(r)}
          >
            {t('replay')}
          </Button>
        ) : (
          <span key="none">-</span>
        ),
      ],
    },
  ];

  return (
    <Drawer title={t('title')} open={open} onClose={onClose} size={960} destroyOnHidden>
      {/* Drawer body 有确定高度(antd 骨架 flex:1),须显式撑满:
          page-container-content 的 flex:1 只在 PageContainer 布局里生效,
          Drawer 里没有固定高度祖先,不撑满会被 hook 量成初始 100px。 */}
      <div ref={containerRef} className="page-container-content" style={{ height: '100%' }}>
        <ListTable<TerminalSession>
          actionRef={actionRef}
          columns={columns}
          rowKey="id"
          request={async (params) => {
            const q = new PaginationQuery({
              paging: { page: params.current || 1, pageSize: params.pageSize || 10 },
              orderBy: ['-createdAt'],
            });
            const resp = await (await import('@/api/hooks/terminal-audit')).fetchListTerminalSessions(q);
            return { data: resp.items || [], total: resp.total || 0, success: true };
          }}
          search={false}
          pagination={{ defaultPageSize: TABLE.DEFAULT_PAGE_SIZE, showSizeChanger: true }}
          options={{ density: true, reload: true }}
          size="middle"
          bordered
          scroll={{ y: tableScrollY, x: 1000 }}
        />
      </div>
    </Drawer>
  );
};

function fmtBytes(n?: number | string): string {
  const v = Number(n) || 0;
  if (v < 1024) return `${v}B`;
  if (v < 1048576) return `${(v / 1024).toFixed(1)}KB`;
  return `${(v / 1048576).toFixed(1)}MB`;
}

export default TerminalSessionsDrawer;
