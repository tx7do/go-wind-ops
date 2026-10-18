import { useRef, useState } from 'react';
import type { ProColumns, ActionType } from '@ant-design/pro-components';
import ListTable from '@/components/common/ListTable';
import { Button, Popconfirm, Tag, App } from 'antd';
import {
  DeleteOutlined,
  EditOutlined,
  HistoryOutlined,
  PlusOutlined,
  RocketOutlined,
} from '@ant-design/icons';
import { useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_Build as Build } from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { TABLE } from '@/config/constants';
import {
  fetchListBuilds,
  useDeleteBuild,
  useTriggerBuild,
} from '@/api/hooks/build';
import { useProTableScrollY } from '@/hooks/useProTableScrollY';
import ContentContainer from '@/layouts/components/PageContainer/ContentContainer';
import BuildDrawer from './components/BuildDrawer';
import BuildHistoryDrawer from './components/BuildHistoryDrawer';

/**
 * 构建配置管理页面
 */
const BuildManagement = () => {
  const { t } = useTranslation('build');
  const actionRef = useRef<ActionType>(null);
  const queryClient = useQueryClient();
  const { message } = App.useApp();

  const containerRef = useRef<HTMLDivElement>(null);
  const tableScrollY = useProTableScrollY(containerRef);

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerMode, setDrawerMode] = useState<'create' | 'edit'>('create');
  const [selectedBuild, setSelectedBuild] = useState<Build | undefined>();
  const [historyBuild, setHistoryBuild] = useState<Build | undefined>();

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['listBuilds'] });

  const deleteMutation = useDeleteBuild({
    onSuccess: () => {
      message.success(t('deleteSuccess'));
      invalidate();
      actionRef.current?.reload();
    },
    onError: (error: Error) => message.error(error.message || t('deleteFailed')),
  });

  const triggerMutation = useTriggerBuild({
    onSuccess: (resp) => {
      message.success(t('triggerSent', { no: resp.buildNo }));
      setTimeout(() => setHistoryBuild((prev) => prev), 100);
      // 若历史抽屉开着,延时刷新由抽屉内轮询完成
    },
    onError: (error: Error) => message.error(error.message || t('triggerFailed')),
  });

  const columns: ProColumns<Build>[] = [
    {
      title: t('name'),
      dataIndex: 'name',
      width: 160,
      render: (_, record) => (
        <span style={{ fontWeight: 600 }}>{record.name || '-'}</span>
      ),
    },
    {
      title: t('repositoryId'),
      dataIndex: 'repositoryId',
      width: 90,
      hideInSearch: true,
    },
    {
      title: t('branch'),
      dataIndex: 'branch',
      width: 110,
      hideInSearch: true,
      render: (_, record) => record.branch || 'main',
    },
    {
      title: t('buildCommand'),
      dataIndex: 'buildCommand',
      width: 240,
      hideInSearch: true,
      ellipsis: true,
    },
    {
      title: t('artifactPath'),
      dataIndex: 'artifactPath',
      width: 160,
      hideInSearch: true,
      ellipsis: true,
    },
    {
      title: t('buildNo'),
      dataIndex: 'buildNo',
      width: 90,
      hideInSearch: true,
      render: (_, record) => <Tag color="blue">#{record.buildNo ?? 0}</Tag>,
    },
    {
      title: t('action'),
      valueType: 'option',
      width: 170,
      fixed: 'right',
      render: (_, record) => [
        <a
          key="trigger"
          style={{ color: 'var(--ant-color-primary)' }}
          onClick={() => record.id && triggerMutation.mutate({ id: record.id })}
        >
          <RocketOutlined /> {t('trigger')}
        </a>,
        <a
          key="history"
          onClick={() => setHistoryBuild(record)}
        >
          <HistoryOutlined /> {t('history')}
        </a>,
        <a
          key="edit"
          onClick={() => {
            setDrawerMode('edit');
            setSelectedBuild(record);
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
          <ListTable<Build>
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

              const response = await fetchListBuilds(query);

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
                  setSelectedBuild(undefined);
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
              x: 1100,
            }}
          />
        </div>
      </ContentContainer>

      <BuildDrawer
        open={drawerOpen}
        mode={drawerMode}
        data={selectedBuild}
        onClose={() => {
          setDrawerOpen(false);
          setSelectedBuild(undefined);
        }}
        onSuccess={() => {
          invalidate();
          actionRef.current?.reload();
        }}
      />

      <BuildHistoryDrawer
        build={historyBuild}
        onClose={() => setHistoryBuild(undefined)}
      />
    </>
  );
};

export default BuildManagement;
