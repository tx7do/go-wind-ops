import { useEffect, useRef, useState } from 'react';
import type { ProColumns, ActionType } from '@ant-design/pro-components';
import { App, Drawer, Tag } from 'antd';
import { DownloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_BackupRecord as BackupRecord } from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { TABLE } from '@/config/constants';
import ListTable from '@/components/common/ListTable';
import { fetchListBackupRecords, useGetBackupRecordDownloadUrl } from '@/api/hooks/backup';
import { useProTableScrollY } from '@/hooks/useProTableScrollY';

interface Props {
  taskId?: number;
  taskName?: string;
  onClose: () => void;
}

const PENDING_POLL_MS = 3000;

/**
 * 备份记录抽屉:按任务查看执行记录;存在 PENDING 记录时延时自动刷新。
 */
const BackupRecordsDrawer = ({ taskId, taskName, onClose }: Props) => {
  const { t } = useTranslation('backup');
  const { message } = App.useApp();
  const actionRef = useRef<ActionType>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const tableScrollY = useProTableScrollY(containerRef);
  const pollTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const [open, setOpen] = useState(false);

  useEffect(() => {
    setOpen(!!taskId);
    return () => {
      if (pollTimer.current) clearTimeout(pollTimer.current);
    };
  }, [taskId]);

  const downloadMutation = useGetBackupRecordDownloadUrl({
    onSuccess: (resp) => {
      if (resp.url) window.open(resp.url, '_blank');
    },
    onError: (e: Error) => message.error(e.message || t('downloadFailed')),
  });

  const schedulePoll = (items: BackupRecord[]) => {
    if (pollTimer.current) clearTimeout(pollTimer.current);
    if (items.some((r) => r.status === 'PENDING')) {
      pollTimer.current = setTimeout(() => actionRef.current?.reload(), PENDING_POLL_MS);
    }
  };

  const columns: ProColumns<BackupRecord>[] = [
    {
      title: t('recordTime'),
      dataIndex: 'createdAt',
      width: 165,
      valueType: 'dateTime',
    },
    {
      title: t('recordSize'),
      dataIndex: 'sizeBytes',
      width: 100,
      render: (_, r) => formatBytes(r.sizeBytes ?? 0),
    },
    {
      title: t('recordStatus'),
      dataIndex: 'status',
      width: 100,
      render: (_, r) =>
        r.status === 'SUCCESS' ? (
          <Tag color="success">{t('recordSuccess')}</Tag>
        ) : r.status === 'FAILED' ? (
          <Tag color="error" title={r.error || undefined}>
            {t('recordFailed')}
          </Tag>
        ) : (
          <Tag color="processing">{t('recordPending')}</Tag>
        ),
    },
    {
      title: t('recordError'),
      dataIndex: 'error',
      ellipsis: true,
      render: (_, r) => r.error || '-',
    },
    {
      title: t('action'),
      valueType: 'option',
      width: 90,
      render: (_, r) =>
        r.status === 'SUCCESS' ? (
          <a onClick={() => r.id && downloadMutation.mutate({ id: r.id })}>
            <DownloadOutlined /> {t('download')}
          </a>
        ) : (
          <span>-</span>
        ),
    },
  ];

  return (
    <Drawer
      title={t('recordsTitle', { name: taskName })}
      open={open}
      onClose={onClose}
      size={820}
      destroyOnHidden
    >
      <div ref={containerRef} className="page-container-content">
        <ListTable<BackupRecord>
          actionRef={actionRef}
          columns={columns}
          rowKey="id"
          request={async (params) => {
            if (!taskId) return { data: [], total: 0, success: true };
            const q = new PaginationQuery({
              paging: { page: params.current || 1, pageSize: params.pageSize || 10 },
              formValues: { taskId },
              orderBy: ['-createdAt'],
            });
            const resp = await fetchListBackupRecords(q);
            const items = resp.items || [];
            schedulePoll(items);
            return { data: items, total: resp.total || 0, success: true };
          }}
          search={false}
          pagination={{ defaultPageSize: TABLE.DEFAULT_PAGE_SIZE, showSizeChanger: true }}
          options={{ density: true, reload: true }}
          size="middle"
          bordered
          scroll={{ y: tableScrollY }}
        />
      </div>
    </Drawer>
  );
};

function formatBytes(n: number | string): string {
  // proto int64 经 JSON 序列化是字符串,先归一为数字
  const v0 = Number(n);
  if (!v0 || Number.isNaN(v0)) return '-';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let v = v0;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(v >= 100 || i === 0 ? 0 : 1)} ${units[i]}`;
}

export default BackupRecordsDrawer;
