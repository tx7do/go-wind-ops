import { useRef, useState } from 'react';
import type { ProColumns, ActionType } from '@ant-design/pro-components';
import ListTable from '@/components/common/ListTable';
import { Button, Popconfirm, Switch, Tag, App } from 'antd';
import { DeleteOutlined, EditOutlined, PlusOutlined } from '@ant-design/icons';
import { useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_Monitor as Monitor } from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { TABLE } from '@/config/constants';
import { fetchListMonitors, useDeleteMonitor } from '@/api/hooks/monitor';
import { useProTableScrollY } from '@/hooks/useProTableScrollY';
import ContentContainer from '@/layouts/components/PageContainer/ContentContainer';
import MonitorDrawer from './components/MonitorDrawer';

const MonitorManagement = () => {
  const { t } = useTranslation('monitor');
  const actionRef = useRef<ActionType>(null);
  const queryClient = useQueryClient();
  const { message } = App.useApp();
  const containerRef = useRef<HTMLDivElement>(null);
  const tableScrollY = useProTableScrollY(containerRef);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerMode, setDrawerMode] = useState<'create' | 'edit'>('create');
  const [selected, setSelected] = useState<Monitor | undefined>();

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['listMonitors'] });

  const deleteMutation = useDeleteMonitor({
    onSuccess: () => { message.success(t('deleteSuccess')); invalidate(); actionRef.current?.reload(); },
    onError: (e: Error) => message.error(e.message || t('deleteFailed')),
  });

  const columns: ProColumns<Monitor>[] = [
    { title: t('name'), dataIndex: 'name', width: 150, render: (_, r) => <span style={{ fontWeight: 600 }}>{r.name}</span> },
    {
      title: t('targetType'), width: 110, hideInSearch: true,
      render: (_, r) => r.targetType === 'NODE_METRIC'
        ? <Tag color="purple">{t('targetNodeMetric')}</Tag>
        : r.targetType === 'PROCESS'
          ? <Tag color="red">{t('targetProcess')}</Tag>
          : <Tag color="blue">{t('targetProject')}</Tag>,
    },
    {
      title: t('target'), width: 180, hideInSearch: true, ellipsis: true,
      render: (_, r) => {
        if (r.targetType === 'NODE_METRIC') {
          const metric = r.metricType === 'MEMORY' ? t('metricMemory') : r.metricType === 'DISK' ? t('metricDisk') : t('metricCpu');
          return `#${r.nodeId} · ${metric} ≥ ${r.thresholdPercent ?? 0}%`;
        }
        if (r.targetType === 'PROCESS') {
          return r.processName ? `#${r.nodeId} · ${r.processName}` : `#${r.nodeId}`;
        }
        return r.projectId ? `#${r.projectId}` : '-';
      },
    },
    { title: t('cron'), dataIndex: 'cron', width: 130, hideInSearch: true },
    { title: t('autoRestart'), width: 90, hideInSearch: true, render: (_, r) => r.autoRestart ? <Tag color="processing">{t('yes')}</Tag> : <Tag>{t('no')}</Tag> },
    { title: t('enabled'), width: 80, hideInSearch: true, render: (_, r) => <Switch checked={r.enabled} disabled size="small" /> },
    { title: t('description'), dataIndex: 'description', hideInSearch: true, ellipsis: true },
    {
      title: t('action'), valueType: 'option', width: 100,
      render: (_, r) => [
        <a key="edit" onClick={() => { setDrawerMode('edit'); setSelected(r); setDrawerOpen(true); }}><EditOutlined /></a>,
        <Popconfirm key="del" title={t('deleteConfirmTitle')} onConfirm={() => r.id && deleteMutation.mutate({ id: r.id })} okText="OK" cancelText="Cancel">
          <a style={{ color: 'var(--ant-color-error)' }}><DeleteOutlined /></a>
        </Popconfirm>,
      ],
    },
  ];

  return (
    <>
      <ContentContainer heightMode="fixed" padding="16px" bottomMargin={0}>
        <div ref={containerRef} className="page-container-content">
          <ListTable<Monitor>
            actionRef={actionRef} columns={columns} rowKey="id"
            request={async (params, sorter) => {
              const q = new PaginationQuery({
                paging: { page: params.current || 1, pageSize: params.pageSize || 10 },
                formValues: Object.fromEntries(Object.entries(params).filter(([k]) => !['current', 'pageSize'].includes(k))),
                orderBy: sorter && Object.keys(sorter).length > 0 ? Object.entries(sorter).map(([k, v]) => v === 'ascend' ? k : `-${k}`) : undefined,
              });
              const resp = await fetchListMonitors(q);
              return { data: resp.items || [], total: resp.total || 0, success: true };
            }}
            search={{ labelWidth: 'auto' }}
            pagination={{ defaultPageSize: TABLE.DEFAULT_PAGE_SIZE, showSizeChanger: true }}
            toolBarRender={() => [
              <Button key="create" type="primary" icon={<PlusOutlined />} onClick={() => { setDrawerMode('create'); setSelected(undefined); setDrawerOpen(true); }}>{t('create')}</Button>,
            ]}
            options={{ density: true, fullScreen: true, setting: true, reload: true }}
            size="middle" bordered scroll={{ y: tableScrollY, x: 900 }}
          />
        </div>
      </ContentContainer>
      <MonitorDrawer open={drawerOpen} mode={drawerMode} data={selected} onClose={() => { setDrawerOpen(false); setSelected(undefined); }} onSuccess={() => { invalidate(); actionRef.current?.reload(); }} />
    </>
  );
};
export default MonitorManagement;
