import { useRef, useState } from 'react';
import type { ProColumns, ActionType, ProFormInstance } from '@ant-design/pro-components';
import {
  DrawerForm,
  ProFormList,
  ProFormDigit,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import ListTable from '@/components/common/ListTable';
import { App, Button, InputNumber, Modal, Popconfirm, Space, Tag } from 'antd';
import { DeleteOutlined, EditOutlined, PlusOutlined, RocketOutlined } from '@ant-design/icons';
import { useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_PromotionPipeline as Pipeline } from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { TABLE } from '@/config/constants';
import {
  fetchListPromotionPipelines,
  useCreatePromotionPipeline,
  useDeletePromotionPipeline,
  usePromotePromotion,
  useUpdatePromotionPipeline,
} from '@/api/hooks/promotion-pipeline';
import { useProTableScrollY } from '@/hooks/useProTableScrollY';
import ContentContainer from '@/layouts/components/PageContainer/ContentContainer';

/**
 * 环境晋升流水线:同一构建产物按有序阶段(每阶段=既有分发配置)逐环境晋升。
 * 晋升=触发该阶段分发的指定构建历史(缺省最新成功),产物可追溯复用分发记录。
 */
const PromotionPipelineManagement = () => {
  const { t } = useTranslation('promotion-pipeline');
  const actionRef = useRef<ActionType>(null);
  const formRef = useRef<ProFormInstance>(null);
  const queryClient = useQueryClient();
  const { message } = App.useApp();
  const containerRef = useRef<HTMLDivElement>(null);
  const tableScrollY = useProTableScrollY(containerRef);

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerMode, setDrawerMode] = useState<'create' | 'edit'>('create');
  const [editing, setEditing] = useState<Pipeline | undefined>();
  const [promoting, setPromoting] = useState<Pipeline | undefined>();

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['listPromotionPipelines'] });

  const saveMutation = useCreatePromotionPipeline({
    onSuccess: () => {
      message.success(t('createSuccess'));
      invalidate();
      actionRef.current?.reload();
    },
    onError: (e: Error) => message.error(e.message || t('createFailed')),
  });
  const updateMutation = useUpdatePromotionPipeline({
    onSuccess: () => {
      message.success(t('updateSuccess'));
      invalidate();
      actionRef.current?.reload();
    },
    onError: (e: Error) => message.error(e.message || t('updateFailed')),
  });
  const deleteMutation = useDeletePromotionPipeline({
    onSuccess: () => {
      message.success(t('deleteSuccess'));
      invalidate();
      actionRef.current?.reload();
    },
    onError: (e: Error) => message.error(e.message || t('deleteFailed')),
  });

  const promoteMutation = usePromotePromotion({
    onSuccess: (resp) => {
      message.success(t('promoteSent', { env: resp.env || '', count: resp.recordIds?.length || 0 }));
      setPromoting(undefined);
    },
    onError: (e: Error) => message.error(e.message || t('promoteFailed')),
  });

  const columns: ProColumns<Pipeline>[] = [
    { title: t('name'), dataIndex: 'name', width: 220, render: (_, r) => <span style={{ fontWeight: 600 }}>{r.name}</span> },
    {
      title: t('stages'),
      dataIndex: 'stages',
      render: (_, r) => (
        <Space wrap size={4}>
          {(r.stages ?? []).map((st, i) => (
            <Tag key={i} color="blue">
              {st.name || st.env} (#{st.distributionId})
            </Tag>
          ))}
        </Space>
      ),
    },
    { title: t('description'), dataIndex: 'description', ellipsis: true },
    {
      title: t('action'),
      valueType: 'option',
      width: 160,
      fixed: 'right',
      render: (_, record) => [
        <a
          key="promote"
          style={{ color: 'var(--ant-color-primary)' }}
          onClick={() => setPromoting(record)}
        >
          <RocketOutlined /> {t('promote')}
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

  return (
    <>
      <ContentContainer heightMode="fixed" padding="16px" bottomMargin={0}>
        <div ref={containerRef} className="page-container-content">
          <ListTable<Pipeline>
            actionRef={actionRef}
            columns={columns}
            headerTitle={t('moduleName')}
            request={async (params) => {
              const query = new PaginationQuery({
                paging: { page: params.current || 1, pageSize: params.pageSize || 10 },
              });
              const resp = await fetchListPromotionPipelines(query);
              return {
                data: resp.items || [],
                total: resp.total || 0,
                success: true,
              };
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
                  setDrawerMode('create');
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

      <DrawerForm
        title={drawerMode === 'create' ? t('create') : `${t('edit')}: ${editing?.name ?? ''}`}
        formRef={formRef}
        open={drawerOpen}
        drawerProps={{ onClose: () => { setDrawerOpen(false); setEditing(undefined); }, destroyOnHidden: true, width: 560 }}
        onFinish={async (values: Record<string, unknown>) => {
          if (drawerMode === 'create') await saveMutation.mutateAsync(values as never);
          else if (editing?.id) await updateMutation.mutateAsync({ id: editing.id, values: values as never });
          setDrawerOpen(false);
          setEditing(undefined);
          return true;
        }}
        params={{ editing, drawerMode }}
      >
        {drawerOpen && (
          <>
            <ProFormText
              name="name"
              label={t('name')}
              rules={[{ required: true, message: t('nameRequired') }]}
              initialValue={editing?.name}
            />
            <ProFormList
              name="stages"
              label={t('stages')}
              copyIconProps={false}
              initialValue={editing?.stages?.map((st) => ({
                distributionId: st.distributionId,
                env: st.env,
                name: st.name,
              }))}
            >
              <Space align="baseline" wrap>
                <ProFormDigit name="distributionId" label={t('distributionId')} min={1} width="sm" />
                <ProFormText name="name" label={t('stageName')} width="xs" />
                <ProFormText name="env" label={t('stageEnv')} width="xs" placeholder="staging" />
              </Space>
            </ProFormList>
            <ProFormTextArea name="description" label={t('description')} fieldProps={{ rows: 2, allowClear: true }} initialValue={editing?.description} />
          </>
        )}
      </DrawerForm>

      <Modal
        title={`${t('promote')}: ${promoting?.name ?? ''}`}
        open={!!promoting}
        onCancel={() => setPromoting(undefined)}
        footer={null}
        width={520}
      >
        <PromoteForm
          pipeline={promoting}
          submitting={promoteMutation.isPending}
          onSubmit={(stageIndex, historyId) =>
            promoting?.id &&
            promoteMutation.mutate({ id: promoting.id, stageIndex, historyId })
          }
        />
      </Modal>
    </>
  );
};

const PromoteForm: React.FC<{
  pipeline?: Pipeline;
  submitting: boolean;
  onSubmit: (stageIndex: number, historyId?: number) => void;
}> = ({ pipeline, submitting, onSubmit }) => {
  const { t } = useTranslation('promotion-pipeline');
  const [stageIndex, setStageIndex] = useState(0);
  const [historyId, setHistoryId] = useState<number | undefined>();

  return (
    <Space direction="vertical" style={{ width: '100%' }} size="middle">
      <div>
        <div style={{ marginBottom: 8 }}>{t('stageLabel')}</div>
        <Space wrap>
          {(pipeline?.stages ?? []).map((st, i) => (
            <Tag.CheckableTag key={i} checked={stageIndex === i} onChange={() => setStageIndex(i)}>
              {st.name || st.env} (#{st.distributionId})
            </Tag.CheckableTag>
          ))}
        </Space>
      </div>
      <div>
        <div style={{ marginBottom: 8 }}>{t('historyIdLabel')}</div>
        <InputNumber
          min={1}
          value={historyId}
          onChange={(v) => setHistoryId(v ?? undefined)}
          style={{ width: 220 }}
          placeholder={t('historyIdPlaceholder')}
        />
      </div>
      <Button
        type="primary"
        icon={<RocketOutlined />}
        loading={submitting}
        onClick={() => onSubmit(stageIndex, historyId)}
      >
        {t('promoteGo')}
      </Button>
    </Space>
  );
};

export default PromotionPipelineManagement;
