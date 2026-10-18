import { useRef, useState } from 'react';
import type { ProColumns, ActionType } from '@ant-design/pro-components';
import ListTable from '@/components/common/ListTable';
import { Button, Popconfirm, App } from 'antd';
import {
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
} from '@ant-design/icons';
import { useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_Repository as Repository } from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { TABLE } from '@/config/constants';
import {
  fetchListRepositories,
  useDeleteRepository,
} from '@/api/hooks/repository';
import { useProTableScrollY } from '@/hooks/useProTableScrollY';
import ContentContainer from '@/layouts/components/PageContainer/ContentContainer';
import RepositoryDrawer from './components/RepositoryDrawer';

/**
 * 构建源仓库管理页面
 */
const RepositoryManagement = () => {
  const { t } = useTranslation('repository');
  const actionRef = useRef<ActionType>(null);
  const queryClient = useQueryClient();
  const { message } = App.useApp();

  const containerRef = useRef<HTMLDivElement>(null);
  const tableScrollY = useProTableScrollY(containerRef);

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerMode, setDrawerMode] = useState<'create' | 'edit'>('create');
  const [selectedRepo, setSelectedRepo] = useState<Repository | undefined>();

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['listRepositories'] });

  const deleteMutation = useDeleteRepository({
    onSuccess: () => {
      message.success(t('deleteSuccess'));
      invalidate();
      actionRef.current?.reload();
    },
    onError: (error: Error) => message.error(error.message || t('deleteFailed')),
  });

  const columns: ProColumns<Repository>[] = [
    {
      title: t('name'),
      dataIndex: 'name',
      width: 160,
      render: (_, record) => (
        <span style={{ fontWeight: 600 }}>{record.name || '-'}</span>
      ),
    },
    {
      title: t('gitUrl'),
      dataIndex: 'gitUrl',
      width: 320,
      ellipsis: true,
    },
    {
      title: t('branch'),
      dataIndex: 'branch',
      width: 110,
      hideInSearch: true,
    },
    {
      title: t('username'),
      dataIndex: 'username',
      width: 120,
      hideInSearch: true,
      render: (_, record) => record.username || '-',
    },
    {
      title: t('description'),
      dataIndex: 'description',
      width: 180,
      hideInSearch: true,
      ellipsis: true,
    },
    {
      title: t('action'),
      valueType: 'option',
      width: 100,
      render: (_, record) => [
        <a
          key="edit"
          onClick={() => {
            setDrawerMode('edit');
            setSelectedRepo(record);
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
          <ListTable<Repository>
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

              const response = await fetchListRepositories(query);

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
                  setSelectedRepo(undefined);
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

      <RepositoryDrawer
        open={drawerOpen}
        mode={drawerMode}
        data={selectedRepo}
        onClose={() => {
          setDrawerOpen(false);
          setSelectedRepo(undefined);
        }}
        onSuccess={() => {
          invalidate();
          actionRef.current?.reload();
        }}
      />
    </>
  );
};

export default RepositoryManagement;
