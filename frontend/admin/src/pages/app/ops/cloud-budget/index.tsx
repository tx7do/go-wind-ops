import { useRef, useState } from 'react';
import type { ProColumns, ActionType } from '@ant-design/pro-components';
import ListTable from '@/components/common/ListTable';
import { Button, Popconfirm, Tag, Tooltip, App } from 'antd';
import {
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import { useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_CloudBudget as CloudBudget } from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { TABLE } from '@/config/constants';
import { fetchListCloudBudgets, useCheckCloudBudget, useDeleteCloudBudget } from '@/api/hooks/cloud-budget';
import { useProTableScrollY } from '@/hooks/useProTableScrollY';
import ContentContainer from '@/layouts/components/PageContainer/ContentContainer';
import CloudBudgetDrawer from './components/CloudBudgetDrawer';

/**
 * 云成本预算页面:预算 CRUD + 立即检查
 */
const CloudBudgetManagement = () => {
  const { t } = useTranslation('cloud-budget');
  const actionRef = useRef<ActionType>(null);
  const queryClient = useQueryClient();
  const { message } = App.useApp();
  const containerRef = useRef<HTMLDivElement>(null);
  const tableScrollY = useProTableScrollY(containerRef);

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerMode, setDrawerMode] = useState<'create' | 'edit'>('create');
  const [selected, setSelected] = useState<CloudBudget | undefined>();

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['listCloudBudgets'] });

  const deleteMutation = useDeleteCloudBudget({
    onSuccess: () => {
      message.success(t('deleteSuccess'));
      invalidate();
      actionRef.current?.reload();
    },
    onError: (e: Error) => message.error(e.message || t('deleteFailed')),
  });

  const checkMutation = useCheckCloudBudget({
    onSuccess: () => {
      message.success(t('checkSuccess'));
      invalidate();
      actionRef.current?.reload();
    },
    onError: (e: Error) => message.error(e.message || t('checkFailed')),
  });

  const columns: ProColumns<CloudBudget>[] = [
    {
      title: t('month'),
      dataIndex: 'month',
      width: 110,
      render: (_, r) => <span style={{ fontWeight: 600 }}>{r.month}</span>,
    },
    {
      title: t('account'),
      dataIndex: 'cloudAccountId',
      width: 110,
      hideInSearch: true,
      render: (_, r) => (r.cloudAccountId ? `#${r.cloudAccountId}` : <Tag color="blue">{t('allAccounts')}</Tag>),
    },
    {
      title: t('amount'),
      dataIndex: 'amount',
      width: 120,
      hideInSearch: true,
      render: (_, r) => `${(r.amount ?? 0).toLocaleString()} ${r.currency || ''}`,
    },
    {
      title: t('thresholdPercent'),
      dataIndex: 'thresholdPercent',
      width: 100,
      hideInSearch: true,
      render: (_, r) => `${r.thresholdPercent ?? 100}%`,
    },
    {
      title: t('anomalyRatio'),
      dataIndex: 'anomalyRatio',
      width: 100,
      hideInSearch: true,
      render: (_, r) => (r.anomalyRatio ? `×${r.anomalyRatio}` : <Tag>{t('anomalyOff')}</Tag>),
    },
    {
      title: t('notified'),
      width: 100,
      hideInSearch: true,
      render: (_, r) => (r.notified ? <Tag color="warning">{t('notifiedYes')}</Tag> : <Tag>{t('notifiedNo')}</Tag>),
    },
    {
      title: t('enabled'),
      width: 80,
      hideInSearch: true,
      render: (_, r) => (r.enabled ? <Tag color="success">{t('yes')}</Tag> : <Tag>{t('no')}</Tag>),
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
      width: 130,
      fixed: 'right',
      render: (_, r) => [
        <Tooltip key="check" title={t('checkNow')}>
          <a onClick={() => checkMutation.mutate(undefined as never)}>
            <ThunderboltOutlined />
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
          <ListTable<CloudBudget>
            actionRef={actionRef}
            columns={columns}
            rowKey="id"
            request={async (params, sorter) => {
              const q = new PaginationQuery({
                paging: { page: params.current || 1, pageSize: params.pageSize || 10 },
                formValues: Object.fromEntries(
                  Object.entries(params).filter(([key]) => !['current', 'pageSize'].includes(key)),
                ),
                orderBy:
                  sorter && Object.keys(sorter).length > 0
                    ? Object.entries(sorter).map(([key, value]) => (value === 'ascend' ? key : `-${key}`))
                    : ['-month'],
              });
              const resp = await fetchListCloudBudgets(q);
              return { data: resp.items || [], total: resp.total || 0, success: true };
            }}
            search={{ labelWidth: 'auto' }}
            pagination={{ defaultPageSize: TABLE.DEFAULT_PAGE_SIZE, showSizeChanger: true }}
            toolBarRender={() => [
              <Button
                key="check"
                icon={<ThunderboltOutlined />}
                loading={checkMutation.isPending}
                onClick={() => checkMutation.mutate(undefined as never)}
              >
                {t('checkNow')}
              </Button>,
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
            scroll={{ y: tableScrollY, x: 950 }}
          />
        </div>
      </ContentContainer>

      <CloudBudgetDrawer
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
    </>
  );
};

export default CloudBudgetManagement;
