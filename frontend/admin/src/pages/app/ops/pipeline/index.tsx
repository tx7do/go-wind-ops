import { useRef, useState } from 'react';
import type { ProColumns, ActionType } from '@ant-design/pro-components';
import ListTable from '@/components/common/ListTable';
import { Button, Popconfirm, Space, Tag, App } from 'antd';
import {
  DeleteOutlined,
  EditOutlined,
  HistoryOutlined,
  PlusOutlined,
  RocketOutlined,
} from '@ant-design/icons';
import { useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_Pipeline as Pipeline } from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { TABLE } from '@/config/constants';
import { fetchListPipelines, useDeletePipeline, useTriggerPipeline } from '@/api/hooks/pipeline';
import { useProTableScrollY } from '@/hooks/useProTableScrollY';
import ContentContainer from '@/layouts/components/PageContainer/ContentContainer';
import PipelineDrawer from './components/PipelineDrawer';
import PipelineRunsModal from './components/PipelineRunsModal';

const PipelineManagement = () => {
  const { t } = useTranslation('pipeline');
  const actionRef = useRef<ActionType>(null);
  const queryClient = useQueryClient();
  const { message } = App.useApp();
  const containerRef = useRef<HTMLDivElement>(null);
  const tableScrollY = useProTableScrollY(containerRef);

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerMode, setDrawerMode] = useState<'create' | 'edit'>('create');
  const [selected, setSelected] = useState<Pipeline | undefined>();
  const [runsFor, setRunsFor] = useState<Pipeline | undefined>();

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['listPipelines'] });

  const deleteMutation = useDeletePipeline({
    onSuccess: () => {
      message.success(t('deleteSuccess'));
      invalidate();
      actionRef.current?.reload();
    },
    onError: (e: Error) => message.error(e.message || t('deleteFailed')),
  });
  const triggerMutation = useTriggerPipeline({
    onSuccess: (resp) => {
      message.success(t('triggerSent', { runId: resp.runId }));
      setRunsFor((prev) => prev); // 保持弹窗
      setTimeout(() => setRunsFor((p) => (p ? { ...p } : p)), 300);
    },
    onError: (e: Error) => message.error(e.message || t('triggerFailed')),
  });

  const stepTagColor = (type: string | undefined) =>
    type === 'BUILD' ? 'blue' : type === 'SCRIPT' ? 'purple' : type === 'DISTRIBUTION' ? 'cyan' : 'default';

  const columns: ProColumns<Pipeline>[] = [
    {
      title: t('name'),
      dataIndex: 'name',
      width: 180,
      render: (_, r) => <span style={{ fontWeight: 600 }}>{r.name || '-'}</span>,
    },
    {
      title: t('steps'),
      dataIndex: 'steps',
      hideInSearch: true,
      render: (_, r) => (
        <Space size={4} wrap>
          {(r.steps ?? []).map((st, i) => (
            <Tag key={i} color={stepTagColor(st.type)}>
              {i + 1}. {st.type}
              {st.agentId ? `@${st.agentId}` : ''}
            </Tag>
          ))}
        </Space>
      ),
    },
    { title: t('description'), dataIndex: 'description', ellipsis: true, hideInSearch: true },
    {
      title: t('action'),
      valueType: 'option',
      width: 200,
      fixed: 'right',
      render: (_, record) => [
        <a key="trigger" onClick={() => record.id && triggerMutation.mutate({ id: record.id })}>
          <RocketOutlined /> {t('trigger')}
        </a>,
        <a key="runs" onClick={() => setRunsFor(record)}>
          <HistoryOutlined /> {t('runs')}
        </a>,
        <a
          key="edit"
          onClick={() => {
            setDrawerMode('edit');
            setSelected(record);
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

  return (
    <>
      <ContentContainer heightMode="fixed" padding="16px" bottomMargin={0}>
        <div ref={containerRef} className="page-container-content">
          <ListTable<Pipeline>
            actionRef={actionRef}
            columns={columns}
            request={async (params) => {
              const query = new PaginationQuery({
                paging: { page: params.current || 1, pageSize: params.pageSize || 10 },
                formValues: Object.fromEntries(
                  Object.entries(params).filter(([key]) => !['current', 'pageSize'].includes(key)),
                ),
              });
              const resp = await fetchListPipelines(query);
              return { data: resp.items || [], total: resp.total || 0, success: true };
            }}
            rowKey="id"
            search={{ labelWidth: 'auto', defaultCollapsed: false }}
            pagination={{ defaultPageSize: TABLE.DEFAULT_PAGE_SIZE, showSizeChanger: true }}
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
            size="middle"
            bordered
            scroll={{ y: tableScrollY, x: 900 }}
          />
        </div>
      </ContentContainer>

      <PipelineDrawer
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

      <PipelineRunsModal pipeline={runsFor} onClose={() => setRunsFor(undefined)} />
    </>
  );
};

export default PipelineManagement;
