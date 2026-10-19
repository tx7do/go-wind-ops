import { useRef, useState } from 'react';
import type { ProColumns, ActionType } from '@ant-design/pro-components';
import ListTable from '@/components/common/ListTable';
import { Button, Drawer, Form, Input, Modal, Popconfirm, Select, Tag, Tooltip, App } from 'antd';
import {
  DeleteOutlined,
  EditOutlined,
  HistoryOutlined,
  PlusOutlined,
  SendOutlined,
} from '@ant-design/icons';
import { useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import type {
  opsservicev1_AgentNode as AgentNode,
  opsservicev1_ConfigDistribution as ConfigDistribution,
  opsservicev1_ConfigTemplate as ConfigTemplate,
} from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { TABLE } from '@/config/constants';
import {
  fetchListConfigTemplates,
  useCreateConfigTemplate,
  useDeleteConfigTemplate,
  useDistributeConfig,
  useListConfigDistributions,
  useUpdateConfigTemplate,
} from '@/api/hooks/config';
import { fetchListAgentNodes } from '@/api/hooks/agent-node';
import { useProTableScrollY } from '@/hooks/useProTableScrollY';
import ContentContainer from '@/layouts/components/PageContainer/ContentContainer';

const ENV_OPTIONS = ['default', 'dev', 'staging', 'prod'].map((e) => ({ label: e, value: e }));

/**
 * 配置文件中心:模板 CRUD + 下发到节点 + 下发记录
 */
const ConfigCenter = () => {
  const { t } = useTranslation('config-center');
  const actionRef = useRef<ActionType>(null);
  const queryClient = useQueryClient();
  const { message } = App.useApp();
  const containerRef = useRef<HTMLDivElement>(null);
  const tableScrollY = useProTableScrollY(containerRef);
  const [form] = Form.useForm();

  const [editorOpen, setEditorOpen] = useState(false);
  const [editorMode, setEditorMode] = useState<'create' | 'edit'>('create');
  const [selected, setSelected] = useState<ConfigTemplate | undefined>();

  const [distributeTpl, setDistributeTpl] = useState<ConfigTemplate | undefined>();
  const [distributeNodeId, setDistributeNodeId] = useState<number | undefined>();
  const [nodeOptions, setNodeOptions] = useState<{ label: string; value: number }[]>([]);

  const [historyTpl, setHistoryTpl] = useState<ConfigTemplate | undefined>();

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['listConfigTemplates'] });

  const createMutation = useCreateConfigTemplate({
    onSuccess: () => { message.success(t('createSuccess')); setEditorOpen(false); invalidate(); actionRef.current?.reload(); },
    onError: (e: Error) => message.error(e.message || t('createFailed')),
  });
  const updateMutation = useUpdateConfigTemplate({
    onSuccess: () => { message.success(t('updateSuccess')); setEditorOpen(false); invalidate(); actionRef.current?.reload(); },
    onError: (e: Error) => message.error(e.message || t('updateFailed')),
  });
  const deleteMutation = useDeleteConfigTemplate({
    onSuccess: () => { message.success(t('deleteSuccess')); invalidate(); actionRef.current?.reload(); },
    onError: (e: Error) => message.error(e.message || t('deleteFailed')),
  });
  const distributeMutation = useDistributeConfig({
    onSuccess: (resp) => {
      message.success(t('distributeSent', { status: resp.status }));
      setDistributeTpl(undefined);
      if (historyTpl?.id === distributeTpl?.id) queryClient.invalidateQueries({ queryKey: ['listConfigDistributions'] });
    },
    onError: (e: Error) => message.error(e.message || t('distributeFailed')),
  });

  const openCreate = () => {
    setEditorMode('create');
    setSelected(undefined);
    form.resetFields();
    form.setFieldsValue({ env: 'default' });
    setEditorOpen(true);
  };

  const openEdit = (row: ConfigTemplate) => {
    setEditorMode('edit');
    setSelected(row);
    form.setFieldsValue({
      name: row.name,
      env: row.env || 'default',
      targetPath: row.targetPath,
      content: row.content,
      description: row.description,
    });
    setEditorOpen(true);
  };

  const submitEditor = async () => {
    const values = await form.validateFields();
    if (editorMode === 'create') await createMutation.mutateAsync(values);
    else if (selected?.id) await updateMutation.mutateAsync({ id: selected.id, values });
  };

  const openDistribute = async (row: ConfigTemplate) => {
    setDistributeTpl(row);
    setDistributeNodeId(undefined);
    try {
      const r = await fetchListAgentNodes(new PaginationQuery({ paging: { page: 1, pageSize: 100 } }));
      setNodeOptions((r.items ?? []).map((n: AgentNode) => ({
        label: (n.name || n.hostname || n.agentId || String(n.id)) + (n.status === 'ONLINE' ? '' : ` (${t('nodeOffline')})`),
        value: n.id ?? 0,
      })));
    } catch {
      setNodeOptions([]);
    }
  };

  const columns: ProColumns<ConfigTemplate>[] = [
    {
      title: t('name'),
      dataIndex: 'name',
      width: 180,
      render: (_, r) => <span style={{ fontWeight: 600 }}>{r.name}</span>,
    },
    {
      title: t('env'),
      dataIndex: 'env',
      width: 100,
      render: (_, r) => <Tag color="blue">{r.env}</Tag>,
    },
    { title: t('targetPath'), dataIndex: 'targetPath', ellipsis: true, width: 260 },
    { title: t('description'), dataIndex: 'description', ellipsis: true, hideInSearch: true },
    { title: t('updatedAt'), dataIndex: 'updatedAt', width: 165, valueType: 'dateTime', hideInSearch: true },
    {
      title: t('action'),
      valueType: 'option',
      width: 160,
      fixed: 'right',
      render: (_, r) => [
        <Tooltip key="distribute" title={t('distribute')}>
          <a onClick={() => openDistribute(r)}>
            <SendOutlined />
          </a>
        </Tooltip>,
        <Tooltip key="history" title={t('history')}>
          <a onClick={() => setHistoryTpl(r)}>
            <HistoryOutlined />
          </a>
        </Tooltip>,
        <Tooltip key="edit" title={t('edit')}>
          <a onClick={() => openEdit(r)}>
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
          <ListTable<ConfigTemplate>
            actionRef={actionRef}
            columns={columns}
            rowKey="id"
            request={async (params) => {
              const q = new PaginationQuery({
                paging: { page: params.current || 1, pageSize: params.pageSize || 10 },
                formValues: Object.fromEntries(
                  Object.entries(params).filter(([key]) => !['current', 'pageSize'].includes(key)),
                ),
                orderBy: ['-createdAt'],
              });
              const resp = await fetchListConfigTemplates(q);
              return { data: resp.items || [], total: resp.total || 0, success: true };
            }}
            search={{ labelWidth: 'auto' }}
            pagination={{ defaultPageSize: TABLE.DEFAULT_PAGE_SIZE, showSizeChanger: true }}
            toolBarRender={() => [
              <Button key="create" type="primary" icon={<PlusOutlined />} onClick={openCreate}>
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

      {/* 模板编辑抽屉 */}
      <Drawer
        title={editorMode === 'create' ? t('create') : t('edit')}
        open={editorOpen}
        onClose={() => setEditorOpen(false)}
        size={640}
        destroyOnHidden
        extra={
          <Button type="primary" loading={createMutation.isPending || updateMutation.isPending} onClick={submitEditor}>
            {t('common:button.ok')}
          </Button>
        }
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label={t('name')} rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="env" label={t('env')} rules={[{ required: true }]}>
            <Select options={ENV_OPTIONS} />
          </Form.Item>
          <Form.Item
            name="targetPath"
            label={t('targetPath')}
            rules={[{ required: true }]}
            extra={t('targetPathHint')}
          >
            <Input placeholder="/opt/app/config.yaml" />
          </Form.Item>
          <Form.Item
            name="content"
            label={t('content')}
            rules={[{ required: true }]}
            extra={t('contentHint')}
          >
            <Input.TextArea rows={14} style={{ fontFamily: 'monospace' }} />
          </Form.Item>
          <Form.Item name="description" label={t('description')}>
            <Input.TextArea rows={2} />
          </Form.Item>
        </Form>
      </Drawer>

      {/* 下发弹窗 */}
      <Modal
        title={t('distributeTitle', { name: distributeTpl?.name })}
        open={!!distributeTpl}
        onCancel={() => setDistributeTpl(undefined)}
        onOk={() => {
          if (!distributeTpl?.id) return;
          if (!distributeNodeId) {
            message.warning(t('nodeRequired'));
            return;
          }
          distributeMutation.mutate({ templateId: distributeTpl.id, nodeId: distributeNodeId });
        }}
        okText={t('distribute')}
        confirmLoading={distributeMutation.isPending}
        width={440}
        destroyOnHidden
      >
        <div style={{ marginBottom: 8 }}>{t('targetNode')}</div>
        <Select
          style={{ width: '100%' }}
          placeholder={t('nodePlaceholder')}
          value={distributeNodeId}
          onChange={setDistributeNodeId}
          options={nodeOptions}
          showSearch
          optionFilterProp="label"
        />
        <div style={{ marginTop: 8, fontSize: 12, color: 'var(--ant-color-text-secondary)' }}>
          {t('targetPathLabel')}: {distributeTpl?.targetPath}
        </div>
      </Modal>

      {/* 下发记录抽屉 */}
      <DistributionHistoryDrawer template={historyTpl} onClose={() => setHistoryTpl(undefined)} />
    </>
  );
};

// 下发记录抽屉
const DistributionHistoryDrawer = ({ template, onClose }: { template?: ConfigTemplate; onClose: () => void }) => {
  const { t } = useTranslation('config-center');
  const containerRef = useRef<HTMLDivElement>(null);
  const tableScrollY = useProTableScrollY(containerRef);
  const { data: rows = [], isFetching } = useListConfigDistributions(template?.id);

  const columns: ProColumns<ConfigDistribution>[] = [
    { title: t('distTime'), dataIndex: 'createdAt', width: 165, valueType: 'dateTime' },
    { title: t('distNode'), dataIndex: 'nodeId', width: 90 },
    { title: t('distPath'), dataIndex: 'targetPath', ellipsis: true },
    {
      title: t('distStatus'),
      dataIndex: 'status',
      width: 100,
      render: (_, r) =>
        r.status === 'SUCCESS' ? (
          <Tag color="success">{t('distSuccess')}</Tag>
        ) : r.status === 'FAILED' ? (
          <Tag color="error" title={r.error || undefined}>{t('distFailed')}</Tag>
        ) : (
          <Tag color="processing">{t('distPending')}</Tag>
        ),
    },
    { title: t('distError'), dataIndex: 'error', ellipsis: true, render: (_, r) => r.error || '-' },
  ];

  return (
    <Drawer
      title={t('historyTitle', { name: template?.name })}
      open={!!template}
      onClose={onClose}
      size={820}
      destroyOnHidden
    >
      <div ref={containerRef} className="page-container-content">
        <ListTable<ConfigDistribution>
          columns={columns}
          rowKey="id"
          dataSource={rows}
          loading={isFetching}
          search={false}
          pagination={{ defaultPageSize: TABLE.DEFAULT_PAGE_SIZE }}
          options={{ density: true, reload: true }}
          size="middle"
          bordered
          scroll={{ y: tableScrollY }}
        />
      </div>
    </Drawer>
  );
};

export default ConfigCenter;
