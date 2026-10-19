import { useRef, useState } from 'react';
import type { ProColumns, ActionType } from '@ant-design/pro-components';
import ListTable from '@/components/common/ListTable';
import { Button, Popconfirm, Switch, Tag, Tooltip, App } from 'antd';
import {
  DeleteOutlined,
  EditOutlined,
  HistoryOutlined,
  PlayCircleOutlined,
  PlusOutlined,
} from '@ant-design/icons';
import { useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_BackupTask as BackupTask } from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { TABLE } from '@/config/constants';
import { fetchListBackupTasks, useDeleteBackupTask, useTriggerBackup } from '@/api/hooks/backup';
import { useProTableScrollY } from '@/hooks/useProTableScrollY';
import ContentContainer from '@/layouts/components/PageContainer/ContentContainer';
import BackupTaskDrawer from './components/BackupTaskDrawer';
import BackupRecordsDrawer from './components/BackupRecordsDrawer';

/**
 * 备份任务页面:任务 CRUD + 立即备份 + 备份记录
 */
const BackupTaskManagement = () => {
  const { t } = useTranslation('backup');
  const actionRef = useRef<ActionType>(null);
  const queryClient = useQueryClient();
  const { message } = App.useApp();
  const containerRef = useRef<HTMLDivElement>(null);
  const tableScrollY = useProTableScrollY(containerRef);

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerMode, setDrawerMode] = useState<'create' | 'edit'>('create');
  const [selected, setSelected] = useState<BackupTask | undefined>();
  const [recordsTask, setRecordsTask] = useState<BackupTask | undefined>();

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['listBackupTasks'] });

  const deleteMutation = useDeleteBackupTask({
    onSuccess: () => {
      message.success(t('deleteSuccess'));
      invalidate();
      actionRef.current?.reload();
    },
    onError: (e: Error) => message.error(e.message || t('deleteFailed')),
  });

  const triggerMutation = useTriggerBackup({
    onSuccess: (resp) => {
      message.success(t('triggerSuccess', { recordId: resp.recordId }));
      actionRef.current?.reload();
    },
    onError: (e: Error) => message.error(e.message || t('triggerFailed')),
  });

  const columns: ProColumns<BackupTask>[] = [
    {
      title: t('name'),
      dataIndex: 'name',
      width: 160,
      render: (_, r) => <span style={{ fontWeight: 600 }}>{r.name}</span>,
    },
    {
      title: t('nodeId'),
      dataIndex: 'nodeId',
      width: 90,
      hideInSearch: true,
    },
    {
      title: t('sourceDir'),
      dataIndex: 'sourceDir',
      width: 240,
      hideInSearch: true,
      ellipsis: true,
    },
    {
      title: t('cron'),
      dataIndex: 'cron',
      width: 130,
      hideInSearch: true,
      render: (_, r) => r.cron || <Tag>{t('cronManual')}</Tag>,
    },
    {
      title: t('retention'),
      dataIndex: 'retention',
      width: 100,
      hideInSearch: true,
      render: (_, r) => (r.retention ? `${r.retention}` : t('retentionKeepAll')),
    },
    {
      title: t('enabled'),
      width: 90,
      hideInSearch: true,
      render: (_, r) => <Switch checked={r.enabled} disabled size="small" />,
    },
    {
      title: t('description'),
      dataIndex: 'description',
      hideInSearch: true,
      ellipsis: true,
    },
    {
      title: t('action'),
      valueType: 'option',
      width: 150,
      fixed: 'right',
      render: (_, r) => [
        <Tooltip key="trigger" title={t('triggerNow')}>
          <a onClick={() => r.id && triggerMutation.mutate({ id: r.id })}>
            <PlayCircleOutlined />
          </a>
        </Tooltip>,
        <Tooltip key="records" title={t('records')}>
          <a onClick={() => setRecordsTask(r)}>
            <HistoryOutlined />
          </a>
        </Tooltip>,
        <Tooltip key="edit" title={t('edit')}>
          <a
            onClick={() => {
              setDrawerMode('edit');
              setSelected(r);
              setDrawerOpen(true);
            }}
          >
            <EditOutlined />
          </a>
        </Tooltip>,
        <Popconfirm
          key="delete"
          title={t('deleteConfirmTitle')}
          onConfirm={() => r.id && deleteMutation.mutate({ id: r.id })}
          okText={t('common:button.ok')}
          cancelText={t('common:button.cancel')}
        >
          <a style={{ color: 'var(--ant-color-error)' }}>
            <DeleteOutlined />
          </a>
        </Popconfirm>,
      ],
    },
  ];

  return (
    <>
      <ContentContainer heightMode="fixed" padding="16px" bottomMargin={0}>
        <div ref={containerRef} className="page-container-content">
          <ListTable<BackupTask>
            actionRef={actionRef}
            columns={columns}
            rowKey="id"
            request={async (params, sorter) => {
              const q = new PaginationQuery({
                paging: { page: params.current || 1, pageSize: params.pageSize || 10 },
                formValues: Object.fromEntries(
                  Object.entries(params).filter(
                    ([key]) => !['current', 'pageSize'].includes(key),
                  ),
                ),
                orderBy:
                  sorter && Object.keys(sorter).length > 0
                    ? Object.entries(sorter).map(([key, value]) =>
                        value === 'ascend' ? key : `-${key}`,
                      )
                    : undefined,
              });
              const resp = await fetchListBackupTasks(q);
              return { data: resp.items || [], total: resp.total || 0, success: true };
            }}
            search={{ labelWidth: 'auto' }}
            pagination={{
              defaultPageSize: TABLE.DEFAULT_PAGE_SIZE,
              showSizeChanger: true,
            }}
            toolBarRender={() => [
              <Button
                key="create"
                type="primary"
                icon={<PlusOutlined />}
                onClick={() => {
                  setDrawerMode('create');
                  setSelected(undefined);
                  setDrawerOpen(true);
                }}
              >
                {t('create')}
              </Button>,
            ]}
            options={{ density: true, fullScreen: true, setting: true, reload: true }}
            size="middle"
            bordered
            scroll={{ y: tableScrollY, x: 1000 }}
          />
        </div>
      </ContentContainer>

      <BackupTaskDrawer
        open={drawerOpen}
        mode={drawerMode}
        data={selected}
        onClose={() => {
          setDrawerOpen(false);
          setSelected(undefined);
        }}
        onSuccess={() => {
          invalidate();
          actionRef.current?.reload();
        }}
      />

      <BackupRecordsDrawer
        taskId={recordsTask?.id}
        taskName={recordsTask?.name}
        onClose={() => setRecordsTask(undefined)}
      />
    </>
  );
};

export default BackupTaskManagement;
