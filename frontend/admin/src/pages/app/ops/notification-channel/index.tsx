import { useRef, useState } from 'react';
import type { ProColumns, ActionType } from '@ant-design/pro-components';
import ListTable from '@/components/common/ListTable';
import { Button, Popconfirm, Tag, App } from 'antd';
import { DeleteOutlined, EditOutlined, PlusOutlined } from '@ant-design/icons';
import { useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_NotificationChannel as Channel } from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { TABLE } from '@/config/constants';
import { fetchListNotificationChannels, useDeleteNotificationChannel } from '@/api/hooks/notification-channel';
import { useProTableScrollY } from '@/hooks/useProTableScrollY';
import ContentContainer from '@/layouts/components/PageContainer/ContentContainer';
import NotificationChannelDrawer from './components/NotificationChannelDrawer';

const NotificationChannelManagement = () => {
  const { t } = useTranslation('notification-channel');
  const actionRef = useRef<ActionType>(null);
  const queryClient = useQueryClient();
  const { message } = App.useApp();
  const containerRef = useRef<HTMLDivElement>(null);
  const tableScrollY = useProTableScrollY(containerRef);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerMode, setDrawerMode] = useState<'create' | 'edit'>('create');
  const [selected, setSelected] = useState<Channel | undefined>();

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['listNotificationChannels'] });

  const deleteMutation = useDeleteNotificationChannel({
    onSuccess: () => { message.success(t('deleteSuccess')); invalidate(); actionRef.current?.reload(); },
    onError: (e: Error) => message.error(e.message || t('deleteFailed')),
  });

  const typeTag = (v: string | undefined) => {
    switch (v) {
      case 'WEBHOOK': return <Tag color="blue">{t('typeWebhook')}</Tag>;
      case 'EMAIL': return <Tag color="green">{t('typeEmail')}</Tag>;
      default: return <Tag>{v}</Tag>;
    }
  };

  const columns: ProColumns<Channel>[] = [
    { title: t('name'), dataIndex: 'name', width: 160, render: (_, r) => <span style={{ fontWeight: 600 }}>{r.name}</span> },
    { title: t('type'), dataIndex: 'type', width: 110, render: (_, r) => typeTag(r.type) },
    { title: t('webhookUrl'), dataIndex: 'webhookUrl', width: 250, hideInSearch: true, ellipsis: true },
    { title: t('smtpHost'), dataIndex: 'smtpHost', width: 160, hideInSearch: true, ellipsis: true },
    {
      title: t('enabled'), width: 80, hideInSearch: true,
      render: (_, r) => r.enabled ? <Tag color="success">{t('on')}</Tag> : <Tag>{t('off')}</Tag>,
    },
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
          <ListTable<Channel>
            actionRef={actionRef} columns={columns} rowKey="id"
            request={async (params, sorter) => {
              const q = new PaginationQuery({
                paging: { page: params.current || 1, pageSize: params.pageSize || 10 },
                formValues: Object.fromEntries(Object.entries(params).filter(([k]) => !['current', 'pageSize'].includes(k))),
                orderBy: sorter && Object.keys(sorter).length > 0 ? Object.entries(sorter).map(([k, v]) => v === 'ascend' ? k : `-${k}`) : undefined,
              });
              const resp = await fetchListNotificationChannels(q);
              return { data: resp.items || [], total: resp.total || 0, success: true };
            }}
            search={{ labelWidth: 'auto' }}
            pagination={{ defaultPageSize: TABLE.DEFAULT_PAGE_SIZE, showSizeChanger: true }}
            toolBarRender={() => [
              <Button key="create" type="primary" icon={<PlusOutlined />} onClick={() => { setDrawerMode('create'); setSelected(undefined); setDrawerOpen(true); }}>{t('create')}</Button>,
            ]}
            options={{ density: true, fullScreen: true, setting: true, reload: true }}
            size="middle" bordered scroll={{ y: tableScrollY, x: 1000 }}
          />
        </div>
      </ContentContainer>
      <NotificationChannelDrawer open={drawerOpen} mode={drawerMode} data={selected} onClose={() => { setDrawerOpen(false); setSelected(undefined); }} onSuccess={() => { invalidate(); actionRef.current?.reload(); }} />
    </>
  );
};
export default NotificationChannelManagement;
