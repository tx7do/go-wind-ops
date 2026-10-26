import { useRef, useState } from 'react';
import type { ProColumns, ActionType } from '@ant-design/pro-components';
import ListTable from '@/components/common/ListTable';
import { App, Button, Descriptions, Input, Modal, Popconfirm, Space, Tag, Tooltip, Typography } from 'antd';
import {
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
  QuestionCircleOutlined,
} from '@ant-design/icons';
import { useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_ApiKey as ApiKey } from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { TABLE } from '@/config/constants';
import { fetchListApiKeys, useDeleteApiKey, useUpdateApiKey } from '@/api/hooks/api-key';
import { useProTableScrollY } from '@/hooks/useProTableScrollY';
import ContentContainer from '@/layouts/components/PageContainer/ContentContainer';
import ApiKeyDrawer from './components/ApiKeyDrawer';

const ApiKeyManagement = () => {
  const { t } = useTranslation('api-key');
  const actionRef = useRef<ActionType>(null);
  const queryClient = useQueryClient();
  const { message } = App.useApp();
  const containerRef = useRef<HTMLDivElement>(null);
  const tableScrollY = useProTableScrollY(containerRef);

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerMode, setDrawerMode] = useState<'create' | 'edit'>('create');
  const [createdPlainKey, setCreatedPlainKey] = useState<string>();
  const [helpOpen, setHelpOpen] = useState(false);
  const [editing, setEditing] = useState<ApiKey | undefined>();

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['listApiKeys'] });

  // 行内启用/禁用
  const updateMutation = useUpdateApiKey({
    onSuccess: () => {
      message.success(t('updateSuccess'));
      invalidate();
      actionRef.current?.reload();
    },
    onError: (e: Error) => message.error(e.message || t('updateFailed')),
  });
  const deleteMutation = useDeleteApiKey({
    onSuccess: () => {
      message.success(t('deleteSuccess'));
      invalidate();
      actionRef.current?.reload();
    },
    onError: (e: Error) => message.error(e.message || t('deleteFailed')),
  });

  const columns: ProColumns<ApiKey>[] = [
    {
      title: t('name'),
      dataIndex: 'name',
      width: 180,
      render: (_, r) => <span style={{ fontWeight: 600 }}>{r.name || '-'}</span>,
    },
    {
      title: t('prefix'),
      dataIndex: 'prefix',
      width: 160,
      hideInSearch: true,
      render: (v) => <Typography.Text code>{String(v)}…</Typography.Text>,
    },
    {
      title: t('status'),
      dataIndex: 'status',
      width: 100,
      hideInSearch: true,
      render: (_, r) =>
        r.status === 'DISABLED' ? <Tag color="error">{t('statusDisabled')}</Tag> : <Tag color="success">{t('statusEnabled')}</Tag>,
    },
    {
      title: t('scopes'),
      dataIndex: 'scopes',
      width: 180,
      hideInSearch: true,
      render: (_, r) => {
        const scopes = r.scopes ?? [];
        if (scopes.length === 0 || scopes.includes('ALL')) {
          return <Tag color="blue">{t('scopeAll')}</Tag>;
        }
        const label = (v: string) =>
          t(`scope${v.charAt(0)}${v.slice(1).toLowerCase()}`);
        return (
          <Tooltip title={scopes.map(label).join('、')}>
            <span>
              {scopes.slice(0, 2).map((v) => (
                <Tag key={v}>{label(v)}</Tag>
              ))}
              {scopes.length > 2 && <Tag>+{scopes.length - 2}</Tag>}
            </span>
          </Tooltip>
        );
      },
    },
    { title: t('description'), dataIndex: 'description', ellipsis: true, hideInSearch: true },
    { title: t('lastUsedAt'), dataIndex: 'lastUsedAt', width: 165, hideInSearch: true, render: fmtTime },
    { title: t('expiresAt'), dataIndex: 'expiresAt', width: 165, hideInSearch: true, render: fmtTime },
    { title: t('createdAt'), dataIndex: 'createdAt', width: 165, hideInSearch: true, render: fmtTime },
    {
      title: t('action'),
      valueType: 'option',
      width: 170,
      fixed: 'right',
      render: (_, record) => [
        <a
          key="toggle"
          onClick={() =>
            record.id &&
            updateMutation.mutate({
              id: record.id,
              data: { status: record.status === 'DISABLED' ? 'ENABLED' : 'DISABLED' },
            })
          }
        >
          {record.status === 'DISABLED' ? t('enable') : t('disable')}
        </a>,
        <a
          key="edit"
          onClick={() => {
            setDrawerMode('edit');
            setEditing(record);
            setDrawerOpen(true);
          }}
        >
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

  const usageBase = `${location.origin}`;

  return (
    <>
      <ContentContainer heightMode="fixed" padding="16px" bottomMargin={0}>
        <div ref={containerRef} className="page-container-content">
          <ListTable<ApiKey>
            actionRef={actionRef}
            columns={columns}
            headerTitle={t('moduleName')}
            request={async (params) => {
              const query = new PaginationQuery({
                paging: { page: params.current || 1, pageSize: params.pageSize || 10 },
              });
              const resp = await fetchListApiKeys(query);
              return { data: resp.items || [], total: resp.total || 0, success: true };
            }}
            rowKey="id"
            search={false}
            pagination={{ defaultPageSize: TABLE.DEFAULT_PAGE_SIZE, showSizeChanger: true }}
            toolBarRender={() => [
              <Button key="help" icon={<QuestionCircleOutlined />} onClick={() => setHelpOpen(true)}>
                {t('usage')}
              </Button>,
              <Button
                key="create"
                type="primary"
                icon={<PlusOutlined />}
                onClick={() => {
                  setCreatedPlainKey(undefined);
                  setDrawerMode('create');
                  setEditing(undefined);
                  setDrawerOpen(true);
                }}
              >
                {t('create')}
              </Button>,
            ]}
            size="middle"
            bordered
            scroll={{ y: tableScrollY, x: 1180 }}
          />
        </div>
      </ContentContainer>

      {/* 创建/编辑抽屉:创建成功后经 onSuccess 展示明文(仅一次) */}
      <ApiKeyDrawer
        open={drawerOpen}
        mode={drawerMode}
        data={editing}
        onClose={() => {
          setDrawerOpen(false);
          setEditing(undefined);
        }}
        onSuccess={(plainKey) => {
          if (plainKey) setCreatedPlainKey(plainKey);
          invalidate();
          actionRef.current?.reload();
        }}
      />

      {/* 明文展示(仅一次) */}
      <Modal
        title={t('plainKeyTitle')}
        open={!!createdPlainKey}
        onCancel={() => setCreatedPlainKey(undefined)}
        footer={<Button type="primary" onClick={() => setCreatedPlainKey(undefined)}>{t('common:button.ok')}</Button>}
        width={560}
      >
        <Descriptions column={1} size="small">
          <Descriptions.Item label={t('plainKeyWarning')}>
            <Typography.Text type="danger">{t('plainKeyWarningText')}</Typography.Text>
          </Descriptions.Item>
          <Descriptions.Item label="API Key">
            <Space.Compact style={{ width: '100%' }}>
              <Input readOnly value={createdPlainKey} style={{ fontFamily: 'monospace' }} />
              <Button
                type="primary"
                onClick={() => {
                  navigator.clipboard.writeText(createdPlainKey || '');
                  message.success(t('copied'));
                }}
              >
                {t('copy')}
              </Button>
            </Space.Compact>
          </Descriptions.Item>
        </Descriptions>
      </Modal>

      {/* 用法示例 */}
      <Modal title={t('usage')} open={helpOpen} onCancel={() => setHelpOpen(false)} footer={null} width={760}>
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Typography.Text type="secondary">{t('usageIntro')}</Typography.Text>
          {[
            { label: t('usageTriggerBuild'), code: `curl -X POST "${usageBase}/openapi/v1/builds/1/trigger" -H "X-Api-Key: opsk_xxx"` },
            { label: t('usageTriggerDist'), code: `curl -X POST "${usageBase}/openapi/v1/distributions/1/trigger" -H "X-Api-Key: opsk_xxx"` },
            { label: t('usageTriggerPipeline'), code: `curl -X POST "${usageBase}/openapi/v1/pipelines/1/trigger" -H "X-Api-Key: opsk_xxx"` },
            { label: t('usageWebhook'), code: `curl -X POST "${usageBase}/openapi/v1/webhooks/git/1?branch=main" -H "X-Api-Key: opsk_xxx" -d '{"ref":"refs/heads/main"}'` },
          ].map((item) => (
            <div key={item.label}>
              <Typography.Text strong>{item.label}</Typography.Text>
              <pre
                style={{
                  background: '#f6f6f6',
                  border: '1px solid #e8e8e8',
                  borderRadius: 8,
                  padding: 10,
                  fontSize: 12,
                  overflow: 'auto',
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-all',
                  marginBottom: 0,
                }}
              >
                {item.code}
              </pre>
            </div>
          ))}
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {t('usageNote')}
          </Typography.Text>
        </Space>
      </Modal>
    </>
  );
};

function fmtTime(v: any): string {
  if (!v) return '-';
  return String(v).replace('T', ' ').slice(0, 19);
}

export default ApiKeyManagement;
