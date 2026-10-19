import { useRef, useState } from 'react';
import type { ProColumns, ActionType } from '@ant-design/pro-components';
import ListTable from '@/components/common/ListTable';
import { Button, Modal, Popconfirm, Select, Tooltip, App } from 'antd';
import {
  DeleteOutlined,
  EditOutlined,
  PlayCircleOutlined,
  PlusOutlined,
  TeamOutlined,
} from '@ant-design/icons';
import { useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import type {
  opsservicev1_AgentNode as AgentNode,
  opsservicev1_Script as Script,
} from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { TABLE } from '@/config/constants';
import {
  fetchListScripts,
  useDeleteScript,
  useExecuteScript,
  isDangerousConfirmError,
} from '@/api/hooks/script';
import { useBatchExecScript } from '@/api/hooks/script-batch';
import { fetchListAgentNodes } from '@/api/hooks/agent-node';
import { useProTableScrollY } from '@/hooks/useProTableScrollY';
import ContentContainer from '@/layouts/components/PageContainer/ContentContainer';
import ScriptDrawer from './components/ScriptDrawer';

/**
 * 脚本管理页面
 */
const ScriptManagement = () => {
  const { t } = useTranslation('script');
  const actionRef = useRef<ActionType>(null);
  const queryClient = useQueryClient();
  const { message, modal } = App.useApp();
  const navigate = useNavigate();

  const containerRef = useRef<HTMLDivElement>(null);
  const tableScrollY = useProTableScrollY(containerRef);

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerMode, setDrawerMode] = useState<'create' | 'edit'>('create');
  const [selectedScript, setSelectedScript] = useState<Script | undefined>();

  // 执行弹窗:选目标在线节点
  const [execScript, setExecScript] = useState<Script | undefined>();
  const [execNodeId, setExecNodeId] = useState<number | undefined>();
  const [execLoading, setExecLoading] = useState(false);
  const [nodeOptions, setNodeOptions] = useState<{ label: string; value: number }[]>([]);

  // 批量执行弹窗:多选节点
  const [batchScript, setBatchScript] = useState<Script | undefined>();
  const [batchNodeIds, setBatchNodeIds] = useState<number[]>([]);
  const [batchNodeOptions, setBatchNodeOptions] = useState<{ label: string; value: number }[]>([]);
  const [batchLoading, setBatchLoading] = useState(false);

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['listScripts'] });

  const deleteMutation = useDeleteScript({
    onSuccess: () => {
      message.success(t('deleteSuccess'));
      invalidate();
      actionRef.current?.reload();
    },
    onError: (error: Error) => message.error(error.message || t('deleteFailed')),
  });

  const executeMutation = useExecuteScript({
    onSuccess: (resp) => {
      message.success(t('executeSent', { requestId: resp.requestId }));
      setExecScript(undefined);
    },
    onError: (error: Error) => {
      if (isDangerousConfirmError(error)) {
        modal.confirm({
          title: t('dangerousTitle'),
          content: t('dangerousContent'),
          okText: t('dangerousConfirm'),
          okButtonProps: { danger: true },
          cancelText: t('dangerousCancel'),
          onOk: () => {
            if (!execScript?.id) return;
            executeMutation.mutate(
              { id: execScript.id, nodeId: execNodeId, confirmDangerous: true },
              { onSettled: () => setExecLoading(false) },
            );
          },
        });
        return;
      }
      message.error(error.message || t('executeFailed'));
    },
  });

  const batchExecMutation = useBatchExecScript({
    onSuccess: (resp) => {
      message.success(t('batchSent', { dispatched: resp.dispatched ?? 0, failed: resp.failed ?? 0 }));
      setBatchScript(undefined);
      navigate('/ops/script-batches');
    },
    onError: (error: Error) => {
      if (isDangerousConfirmError(error)) {
        modal.confirm({
          title: t('dangerousTitle'),
          content: t('dangerousContent'),
          okText: t('dangerousConfirm'),
          okButtonProps: { danger: true },
          cancelText: t('dangerousCancel'),
          onOk: () => {
            if (!batchScript?.id || batchNodeIds.length === 0) return;
            batchExecMutation.mutate(
              { scriptId: batchScript.id, nodeIds: batchNodeIds, confirmDangerous: true },
              { onSettled: () => setBatchLoading(false) },
            );
          },
        });
        return;
      }
      message.error(error.message || t('executeFailed'));
    },
  });

  const confirmExecute = () => {
    if (!execScript?.id || !execNodeId) {
      message.warning(t('nodeRequired'));
      return;
    }
    setExecLoading(true);
    executeMutation.mutate(
      { id: execScript.id, nodeId: execNodeId },
      { onSettled: () => setExecLoading(false) },
    );
  };

  const openExecute = async (script: Script) => {
    setExecScript(script);
    setExecNodeId(undefined);
    try {
      // 拉在线节点供选择,默认选第一个
      const resp = await fetchListAgentNodes(
        new PaginationQuery({ paging: { page: 1, pageSize: 100 } }),
      );
      const online: AgentNode[] = (resp.items ?? []).filter((n) => n.status === 'ONLINE');
      setNodeOptions(
        online.map((n) => ({
          label: n.name || n.hostname || n.agentId || String(n.id),
          value: n.id ?? 0,
        })),
      );
      if (online.length > 0) setExecNodeId(online[0].id);
    } catch {
      setNodeOptions([]);
    }
  };

  // 批量执行:展示全部节点(离线标注),由用户自选目标集
  const openBatchExec = async (script: Script) => {
    setBatchScript(script);
    setBatchNodeIds([]);
    try {
      const resp = await fetchListAgentNodes(
        new PaginationQuery({ paging: { page: 1, pageSize: 100 } }),
      );
      setBatchNodeOptions(
        (resp.items ?? []).map((n: AgentNode) => ({
          label:
            (n.name || n.hostname || n.agentId || String(n.id)) +
            (n.status === 'ONLINE' ? '' : ` (${t('nodeOffline')})`),
          value: n.id ?? 0,
        })),
      );
    } catch {
      setBatchNodeOptions([]);
    }
  };

  const confirmBatchExec = () => {
    if (!batchScript?.id || batchNodeIds.length === 0) {
      message.warning(t('batchNodeRequired'));
      return;
    }
    setBatchLoading(true);
    batchExecMutation.mutate(
      { scriptId: batchScript.id, nodeIds: batchNodeIds },
      { onSettled: () => setBatchLoading(false) },
    );
  };

  const columns: ProColumns<Script>[] = [
    {
      title: t('name'),
      dataIndex: 'name',
      width: 180,
      render: (_, record) => (
        <span style={{ fontWeight: 600 }}>{record.name || '-'}</span>
      ),
    },
    {
      title: t('description'),
      dataIndex: 'description',
      width: 220,
      hideInSearch: true,
      ellipsis: true,
    },
    {
      title: t('timeoutSeconds'),
      dataIndex: 'timeoutSeconds',
      width: 110,
      hideInSearch: true,
      render: (_, record) => `${record.timeoutSeconds ?? 60}s`,
    },
    {
      title: t('updatedAt'),
      dataIndex: 'updatedAt',
      width: 170,
      valueType: 'dateTime',
      hideInSearch: true,
    },
    {
      title: t('action'),
      valueType: 'option',
      width: 160,
      fixed: 'right',
      render: (_, record) => [
        <Tooltip key="exec" title={t('execute')}>
          <a onClick={() => openExecute(record)}>
            <PlayCircleOutlined />
          </a>
        </Tooltip>,
        <Tooltip key="batch" title={t('batchExec')}>
          <a onClick={() => openBatchExec(record)}>
            <TeamOutlined />
          </a>
        </Tooltip>,
        <Tooltip key="edit" title={t('edit')}>
          <a
            onClick={() => {
              setDrawerMode('edit');
              setSelectedScript(record);
              setDrawerOpen(true);
            }}
          >
            <EditOutlined />
          </a>
        </Tooltip>,
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
          <ListTable<Script>
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

              const response = await fetchListScripts(query);

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
                  setSelectedScript(undefined);
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
              x: 900,
            }}
          />
        </div>
      </ContentContainer>

      <ScriptDrawer
        open={drawerOpen}
        mode={drawerMode}
        data={selectedScript}
        onClose={() => {
          setDrawerOpen(false);
          setSelectedScript(undefined);
        }}
        onSuccess={() => {
          invalidate();
          actionRef.current?.reload();
        }}
      />

      {/* 执行弹窗:选目标在线节点 */}
      <Modal
        title={t('executeTitle', { name: execScript?.name })}
        open={!!execScript}
        onCancel={() => setExecScript(undefined)}
        onOk={confirmExecute}
        okText={t('execute')}
        confirmLoading={execLoading}
        width={420}
        destroyOnHidden
      >
        <div style={{ marginBottom: 8 }}>{t('executeNodeLabel')}</div>
        <Select
          style={{ width: '100%' }}
          placeholder={t('executeNodePlaceholder')}
          value={execNodeId}
          onChange={setExecNodeId}
          options={nodeOptions}
        />
        <div style={{ marginTop: 8, fontSize: 12, color: 'var(--ant-color-text-secondary)' }}>
          {t('executeHint')}
        </div>
      </Modal>

      {/* 批量执行弹窗:多选目标节点 */}
      <Modal
        title={t('batchTitle', { name: batchScript?.name })}
        open={!!batchScript}
        onCancel={() => setBatchScript(undefined)}
        onOk={confirmBatchExec}
        okText={t('batchExec')}
        confirmLoading={batchLoading}
        width={480}
        destroyOnHidden
      >
        <div style={{ marginBottom: 8 }}>{t('batchNodeLabel')}</div>
        <Select
          style={{ width: '100%' }}
          mode="multiple"
          placeholder={t('batchNodePlaceholder')}
          value={batchNodeIds}
          onChange={setBatchNodeIds}
          options={batchNodeOptions}
          maxTagCount="responsive"
        />
        <div style={{ marginTop: 8, fontSize: 12, color: 'var(--ant-color-text-secondary)' }}>
          {t('batchHint')}
        </div>
      </Modal>
    </>
  );
};

export default ScriptManagement;
