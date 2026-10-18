import { useRef, useState } from 'react';
import type { ProColumns, ActionType } from '@ant-design/pro-components';
import ListTable from '@/components/common/ListTable';
import { Button, Popconfirm, Tag, App } from 'antd';
import {
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
  RocketOutlined,
  HistoryOutlined,
} from '@ant-design/icons';
import { useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_Distribution as Distribution } from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { TABLE } from '@/config/constants';
import {
  fetchListDistributions,
  useDeleteDistribution,
  useTriggerDistribution,
} from '@/api/hooks/distribution';
import { useProTableScrollY } from '@/hooks/useProTableScrollY';
import ContentContainer from '@/layouts/components/PageContainer/ContentContainer';
import DistributionDrawer from './components/DistributionDrawer';
import DistributionRecordDrawer from './components/DistributionRecordDrawer';

/**
 * 构建产物分发管理页面
 */
const DistributionManagement = () => {
  const { t } = useTranslation('distribution');
  const actionRef = useRef<ActionType>(null);
  const queryClient = useQueryClient();
  const { message } = App.useApp();

  const containerRef = useRef<HTMLDivElement>(null);
  const tableScrollY = useProTableScrollY(containerRef);

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerMode, setDrawerMode] = useState<'create' | 'edit'>('create');
  const [selectedDist, setSelectedDist] = useState<Distribution | undefined>();
  const [recordDist, setRecordDist] = useState<Distribution | undefined>();

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['listDistributions'] });

  const deleteMutation = useDeleteDistribution({
    onSuccess: () => {
      message.success(t('deleteSuccess'));
      invalidate();
      actionRef.current?.reload();
    },
    onError: (error: Error) => message.error(error.message || t('deleteFailed')),
  });

  const triggerMutation = useTriggerDistribution({
    onSuccess: (resp) => {
      message.success(t('triggerSent', { recordId: resp.recordId }));
    },
    onError: (error: Error) => message.error(error.message || t('triggerFailed')),
  });

  const afterOptTag = (v: string | undefined) => {
    switch (v) {
      case 'RESTART':
        return <Tag color="processing">{t('afterRestart')}</Tag>;
      case 'START':
        return <Tag color="success">{t('afterStart')}</Tag>;
      default:
        return <Tag>{t('afterNone')}</Tag>;
    }
  };

  const columns: ProColumns<Distribution>[] = [
    {
      title: t('name'),
      dataIndex: 'name',
      width: 170,
      render: (_, record) => (
        <span style={{ fontWeight: 600 }}>{record.name || '-'}</span>
      ),
    },
    {
      title: t('buildId'),
      dataIndex: 'buildId',
      width: 90,
      hideInSearch: true,
    },
    {
      title: t('projectId'),
      dataIndex: 'projectId',
      width: 90,
      hideInSearch: true,
    },
    {
      title: t('clearOld'),
      dataIndex: 'clearOld',
      width: 100,
      hideInSearch: true,
      render: (_, record) =>
        record.clearOld ? <Tag color="error">{t('yes')}</Tag> : <Tag>{t('no')}</Tag>,
    },
    {
      title: t('afterOpt'),
      width: 100,
      hideInSearch: true,
      render: (_, record) => afterOptTag(record.afterOpt),
    },
    {
      title: t('description'),
      dataIndex: 'description',
      width: 160,
      hideInSearch: true,
      ellipsis: true,
    },
    {
      title: t('action'),
      valueType: 'option',
      width: 180,
      fixed: 'right',
      render: (_, record) => [
        <a
          key="trigger"
          style={{ color: 'var(--ant-color-primary)' }}
          onClick={() => record.id && triggerMutation.mutate({ id: record.id })}
        >
          <RocketOutlined /> {t('trigger')}
        </a>,
        <a key="records" onClick={() => setRecordDist(record)}>
          <HistoryOutlined /> {t('records')}
        </a>,
        <a
          key="edit"
          onClick={() => {
            setDrawerMode('edit');
            setSelectedDist(record);
            setDrawerOpen(true);
          }}
        >
          <EditOutlined />
        </a>,
        <Popconfirm
          key="delete"
          title={t('deleteConfirmTitle')}
          description={t('deleteConfirmDesc', { moduleName: t('moduleName') })}
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
          <ListTable<Distribution>
            actionRef={actionRef}
            columns={columns}
            request={async (params, sorter, _filter) => {
              const query = new PaginationQuery({
                paging: {
                  page: params.current || 1,
                  pageSize: params.pageSize || 10,
                },
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

              const response = await fetchListDistributions(query);

              return {
                data: response.items || [],
                total: response.total || 0,
                success: true,
              };
            }}
            rowKey="id"
            search={{
              labelWidth: 'auto',
              defaultCollapsed: false,
            }}
            pagination={{
              defaultPageSize: TABLE.DEFAULT_PAGE_SIZE,
              showSizeChanger: true,
              showQuickJumper: true,
            }}
            toolBarRender={() => [
              <Button
                key="create"
                type="primary"
                icon={<PlusOutlined />}
                onClick={() => {
                  setDrawerMode('create');
                  setSelectedDist(undefined);
                  setDrawerOpen(true);
                }}
              >
                {t('create')}
              </Button>,
            ]}
            options={{
              density: true,
              fullScreen: true,
              setting: true,
              reload: true,
            }}
            size="middle"
            bordered
            cardBordered={false}
            scroll={{
              y: tableScrollY,
              x: 1000,
            }}
          />
        </div>
      </ContentContainer>

      <DistributionDrawer
        open={drawerOpen}
        mode={drawerMode}
        data={selectedDist}
        onClose={() => {
          setDrawerOpen(false);
          setSelectedDist(undefined);
        }}
        onSuccess={() => {
          invalidate();
          actionRef.current?.reload();
        }}
      />

      <DistributionRecordDrawer
        distribution={recordDist}
        onClose={() => setRecordDist(undefined)}
      />
    </>
  );
};

export default DistributionManagement;
