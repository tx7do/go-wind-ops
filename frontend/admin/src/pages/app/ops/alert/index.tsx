import { useRef, useState } from 'react';
import type { ProColumns, ActionType } from '@ant-design/pro-components';
import ListTable from '@/components/common/ListTable';
import { Popconfirm, Tag, Tooltip, App } from 'antd';
import { SendOutlined } from '@ant-design/icons';
import AlertDeliveriesDrawer from './components/AlertDeliveriesDrawer';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_Alert as Alert } from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { TABLE } from '@/config/constants';
import { fetchListAlerts, useResolveAlert } from '@/api/hooks/alert';
import { useProTableScrollY } from '@/hooks/useProTableScrollY';
import ContentContainer from '@/layouts/components/PageContainer/ContentContainer';

const AlertManagement = () => {
  const { t } = useTranslation('alert');
  const [deliveriesAlert, setDeliveriesAlert] = useState<Alert | undefined>();
  const actionRef = useRef<ActionType>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const tableScrollY = useProTableScrollY(containerRef);

  const resolveMutation = useResolveAlert({
    onSuccess: () => { message.success(t('resolveSuccess')); actionRef.current?.reload(); },
    onError: (e: Error) => message.error(e.message || t('resolveFailed')),
  });

  const { message } = App.useApp();

  const columns: ProColumns<Alert>[] = [
    { title: t('source'), dataIndex: 'source', width: 95, hideInSearch: true, render: (_, r) => {
        const map: Record<string, { color: string; label: string }> = {
          MONITOR: { color: 'blue', label: t('sourceMonitor') },
          BUDGET: { color: 'orange', label: t('sourceBudget') },
          CERT: { color: 'purple', label: t('sourceCert') },
          NODE_OFFLINE: { color: 'red', label: t('sourceNodeOffline') },
          AUDIT_CHAIN: { color: 'volcano', label: t('sourceAuditChain') },
        };
        const it = map[r.source || 'MONITOR'] || map.MONITOR;
        return <Tag color={it.color}>{it.label}</Tag>;
      } },
    { title: t('monitorId'), dataIndex: 'monitorId', width: 90, hideInSearch: true },
    { title: t('projectId'), dataIndex: 'projectId', width: 90, hideInSearch: true, render: (_, r) => (r.projectId ? r.projectId : '-') },
    { title: t('nodeId'), dataIndex: 'nodeId', width: 90, hideInSearch: true, render: (_, r) => (r.nodeId ? r.nodeId : '-') },
    {
      title: t('level'), dataIndex: 'level', width: 100,
      render: (_, r) => r.level === 'ERROR' ? <Tag color="error">{r.level}</Tag> : r.level === 'RECOVERY' ? <Tag color="success">{r.level}</Tag> : <Tag color="warning">{r.level}</Tag>,
    },
    { title: t('message'), dataIndex: 'message', ellipsis: true },
    {
      title: t('resolved'), width: 90, hideInSearch: true,
      render: (_, r) => r.resolved ? <Tag color="success">{t('yes')}</Tag> : <Tag color="error">{t('no')}</Tag>,
    },
    { title: t('createdAt'), dataIndex: 'createdAt', width: 165, valueType: 'dateTime' },
    {
      title: t('action'), valueType: 'option', width: 130,
      render: (_, r) => [
        <Tooltip key="deliveries" title={t('deliveries')}>
          <a onClick={() => setDeliveriesAlert(r)}><SendOutlined /></a>
        </Tooltip>,
        ...(!r.resolved ? [
          <Popconfirm key="resolve" title={t('resolveConfirm')} onConfirm={() => r.id && resolveMutation.mutate({ id: r.id })} okText="OK" cancelText="Cancel">
            <a>{t('resolve')}</a>
          </Popconfirm>,
        ] : []),
      ],
    },
  ];

  return (
    <ContentContainer heightMode="fixed" padding="16px" bottomMargin={0}>
      <div ref={containerRef} className="page-container-content">
        <ListTable<Alert>
          actionRef={actionRef} columns={columns} rowKey="id"
          headerTitle={t('moduleName')}
          request={async (params, sorter) => {
            const q = new PaginationQuery({
              paging: { page: params.current || 1, pageSize: params.pageSize || 10 },
              orderBy: sorter && Object.keys(sorter).length > 0 ? Object.entries(sorter).map(([k, v]) => v === 'ascend' ? k : `-${k}`) : ['-createdAt'],
            });
            const resp = await fetchListAlerts(q);
            return { data: resp.items || [], total: resp.total || 0, success: true };
          }}
          pagination={{ defaultPageSize: TABLE.DEFAULT_PAGE_SIZE, showSizeChanger: true }}
          toolBarRender={() => []}
          options={{ density: true, fullScreen: true, setting: true, reload: true }}
          size="middle" bordered scroll={{ y: tableScrollY, x: 1000 }}
        />
      </div>
      <AlertDeliveriesDrawer
        alertId={deliveriesAlert?.id}
        onClose={() => setDeliveriesAlert(undefined)}
      />
    </ContentContainer>
  );
};
export default AlertManagement;
