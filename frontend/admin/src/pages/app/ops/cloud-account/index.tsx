import { useRef, useState } from 'react';
import type { ProColumns, ActionType } from '@ant-design/pro-components';
import ListTable from '@/components/common/ListTable';
import { App, Button, Popconfirm, Tag, Typography } from 'antd';
import { DeleteOutlined, EditOutlined, PlusOutlined, SafetyCertificateOutlined } from '@ant-design/icons';
import { useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_CloudAccount as CloudAccount } from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { TABLE } from '@/config/constants';
import {
  fetchListCloudAccounts,
  useDeleteCloudAccount,
  useVerifyCloudAccount,
} from '@/api/hooks/cloud-account';
import { useProTableScrollY } from '@/hooks/useProTableScrollY';
import ContentContainer from '@/layouts/components/PageContainer/ContentContainer';
import CloudAccountDrawer from './components/CloudAccountDrawer';

const CloudAccountManagement = () => {
  const { t } = useTranslation('cloud-account');
  const actionRef = useRef<ActionType>(null);
  const queryClient = useQueryClient();
  const { message } = App.useApp();
  const containerRef = useRef<HTMLDivElement>(null);
  const tableScrollY = useProTableScrollY(containerRef);

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editing, setEditing] = useState<CloudAccount | undefined>();
  const [verifyingId, setVerifyingId] = useState<number>();

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['listCloudAccounts'] });

  const deleteMutation = useDeleteCloudAccount({
    onSuccess: () => {
      message.success(t('deleteSuccess'));
      invalidate();
      actionRef.current?.reload();
    },
    onError: (e: Error) => message.error(e.message || t('deleteFailed')),
  });
  const verifyMutation = useVerifyCloudAccount({
    onSuccess: (resp) => {
      message.success(`${t('verifySuccess')}${resp.accountId ? ` (${resp.accountId})` : ''}`);
      setVerifyingId(undefined);
      invalidate();
      actionRef.current?.reload();
    },
    onError: (e: Error) => {
      setVerifyingId(undefined);
      message.error(e.message || t('verifyFailed'));
    },
  });

  const providerTag = (provider?: string) => {
    const color: Record<string, string> = {
      ALIYUN: 'orange',
      TENCENT_CLOUD: 'blue',
      HUAWEI_CLOUD: 'red',
      AWS: 'gold',
    };
    return <Tag color={color[provider || ''] || 'default'}>{t(`provider_${provider || 'UNKNOWN'}`)}</Tag>;
  };

  const columns: ProColumns<CloudAccount>[] = [
    {
      title: t('name'),
      dataIndex: 'name',
      width: 160,
      render: (_, r) => <span style={{ fontWeight: 600 }}>{r.name || '-'}</span>,
    },
    { title: t('provider'), dataIndex: 'provider', width: 110, hideInSearch: true, render: (_, r) => providerTag(r.provider) },
    { title: t('accountId'), dataIndex: 'accountId', width: 150, hideInSearch: true, render: (v) => (v ? <Typography.Text copyable={{ text: String(v), tooltips: t('copy') }}>{String(v)}</Typography.Text> : '-') },
    { title: t('accountName'), dataIndex: 'accountName', width: 140, hideInSearch: true, render: (v) => v || '-' },
    { title: t('accessKey'), dataIndex: 'accessKey', width: 170, hideInSearch: true, render: (v) => (v ? <Typography.Text code>{String(v)}</Typography.Text> : '-') },
    { title: t('secretMask'), dataIndex: 'secretMask', width: 100, hideInSearch: true, render: (v) => (v ? <span style={{ letterSpacing: 2 }}>{String(v)}</span> : <Typography.Text type="secondary">-</Typography.Text>) },
    { title: t('region'), dataIndex: 'region', width: 130, hideInSearch: true, render: (v) => v || '-' },
    {
      title: t('status'),
      dataIndex: 'status',
      width: 90,
      hideInSearch: true,
      render: (_, r) =>
        r.status === 'DISABLED' ? <Tag color="error">{t('statusDisabled')}</Tag> : <Tag color="success">{t('statusEnabled')}</Tag>,
    },
    { title: t('lastVerifiedAt'), dataIndex: 'lastVerifiedAt', width: 165, hideInSearch: true, render: fmtTime },
    { title: t('description'), dataIndex: 'description', ellipsis: true, hideInSearch: true },
    { title: t('createdAt'), dataIndex: 'createdAt', width: 165, hideInSearch: true, render: fmtTime },
    {
      title: t('action'),
      valueType: 'option',
      width: 190,
      fixed: 'right',
      render: (_, record) => [
        <a
          key="verify"
          style={verifyingId === record.id ? { opacity: 0.6, pointerEvents: 'none' } : undefined}
          onClick={() => {
            if (!record.id || verifyingId === record.id) return;
            setVerifyingId(record.id);
            verifyMutation.mutate({ id: record.id });
          }}
        >
          <SafetyCertificateOutlined /> {verifyingId === record.id ? t('verifying') : t('verify')}
        </a>,
        <a key="edit" onClick={() => { setEditing(record); setDrawerOpen(true); }}>
          <EditOutlined />
        </a>,
        <Popconfirm
          key="delete"
          title={t('deleteConfirmTitle')}
          onConfirm={() => record.id && deleteMutation.mutate({ id: record.id })}
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
          <ListTable<CloudAccount>
            actionRef={actionRef}
            columns={columns}
            headerTitle={t('moduleName')}
            request={async (params) => {
              const query = new PaginationQuery({
                paging: { page: params.current || 1, pageSize: params.pageSize || 10 },
              });
              const resp = await fetchListCloudAccounts(query);
              return { data: resp.items || [], total: resp.total || 0, success: true };
            }}
            rowKey="id"
            search={false}
            pagination={{ defaultPageSize: TABLE.DEFAULT_PAGE_SIZE, showSizeChanger: true }}
            toolBarRender={() => [
              <Button
                key="create"
                type="primary"
                icon={<PlusOutlined />}
                onClick={() => {
                  setEditing(undefined);
                  setDrawerOpen(true);
                }}
              >
                {t('create')}
              </Button>,
            ]}
            size="middle"
            bordered
            scroll={{ y: tableScrollY, x: 1500 }}
          />
        </div>
      </ContentContainer>

      <CloudAccountDrawer
        open={drawerOpen}
        mode={editing ? 'edit' : 'create'}
        data={editing}
        onClose={() => {
          setDrawerOpen(false);
          setEditing(undefined);
        }}
        onSuccess={() => {
          invalidate();
          actionRef.current?.reload();
        }}
      />
    </>
  );
};

function fmtTime(v: any): string {
  if (!v) return '-';
  return String(v).replace('T', ' ').slice(0, 19);
}

export default CloudAccountManagement;
