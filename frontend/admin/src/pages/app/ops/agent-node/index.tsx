import { useRef, useState } from 'react';
import type { ProColumns, ActionType } from '@ant-design/pro-components';
import ListTable from '@/components/common/ListTable';
import { Input, Modal, Popconfirm, Progress, Select, Space, Tag, Tooltip, App } from 'antd';
import { DeleteOutlined, EditOutlined, GlobalOutlined, LineChartOutlined } from '@ant-design/icons';
import { useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_AgentNode as AgentNode } from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { ENV_PRESETS, ENV_TAG_COLOR, TABLE } from '@/config/constants';
import { fetchListAgentNodes, useDeleteAgentNode, useSendAgentCommand, useUpdateAgentNode } from '@/api/hooks/agent-node';
import { useProTableScrollY } from '@/hooks/useProTableScrollY';
import ContentContainer from '@/layouts/components/PageContainer/ContentContainer';
import MetricsTrendDrawer from './components/MetricsTrendDrawer';

/**
 * Agent 节点管理页面
 * 节点由 agent 经 MQTT 准入自动注册(TOFU),本页提供查看/编辑/连通性探测/删除。
 */
const AgentNodeManagement = () => {
  const { t } = useTranslation('agent-node');
  const actionRef = useRef<ActionType>(null);
  const queryClient = useQueryClient();
  const { message } = App.useApp();

  const containerRef = useRef<HTMLDivElement>(null);
  const tableScrollY = useProTableScrollY(containerRef);

  const deleteMutation = useDeleteAgentNode({
    onSuccess: () => {
      message.success(t('deleteSuccess'));
      actionRef.current?.reload();
      queryClient.invalidateQueries({ queryKey: ['listAgentNodes'] });
    },
    onError: (error: Error) => {
      message.error(error.message || t('deleteFailed'));
    },
  });

  const pingMutation = useSendAgentCommand({
    onSuccess: (resp) => {
      message.success(t('pingSent', { requestId: resp.requestId }));
    },
    onError: (error: Error) => {
      message.error(error.message || t('pingFailed'));
    },
  });

  const [trendNode, setTrendNode] = useState<AgentNode | undefined>();
  const [editing, setEditing] = useState<AgentNode | undefined>();

  const updateMutation = useUpdateAgentNode({
    onSuccess: () => {
      message.success(t('updateSuccess'));
      setEditing(undefined);
      actionRef.current?.reload();
      queryClient.invalidateQueries({ queryKey: ['listAgentNodes'] });
    },
    onError: (error: Error) => {
      message.error(error.message || t('updateFailed'));
    },
  });

  const renderPercent = (value: number | undefined) => {
    const percent = Math.round(value ?? 0);
    const status = percent >= 90 ? 'exception' : percent >= 70 ? 'active' : 'normal';
    return <Progress type="line" percent={percent} size="small" status={status as any} />;
  };

  const columns: ProColumns<AgentNode>[] = [
    {
      title: t('name'),
      dataIndex: 'name',
      width: 140,
      render: (_, record) => (
        <span style={{ fontWeight: 600 }}>{record.name || record.hostname || '-'}</span>
      ),
    },
    {
      title: t('status'),
      dataIndex: 'status',
      width: 90,
      render: (_, record) =>
        record.status === 'ONLINE' ? (
          <Tag color="success">{t('statusOnline')}</Tag>
        ) : (
          <Tag color="default">{t('statusOffline')}</Tag>
        ),
    },
    {
      title: t('env'),
      dataIndex: 'env',
      width: 100,
      valueType: 'select',
      fieldProps: { options: ENV_PRESETS.map((v) => ({ label: v, value: v })) },
      render: (_, record) => {
        const env = record.env || 'default';
        return <Tag color={ENV_TAG_COLOR[env] ?? 'default'}>{env}</Tag>;
      },
    },
    {
      title: t('hostname'),
      dataIndex: 'hostname',
      width: 130,
      hideInSearch: true,
    },
    {
      title: t('platform'),
      width: 170,
      hideInSearch: true,
      render: (_, record) => {
        const text = [record.platform, record.arch].filter(Boolean).join(' / ');
        return text ? (
          <Tooltip title={record.os ? `${record.os} · ${record.kernelVersion ?? ''}` : ''}>
            <span>{text}</span>
          </Tooltip>
        ) : (
          '-'
        );
      },
    },
    {
      title: t('agentVersion'),
      dataIndex: 'agentVersion',
      width: 90,
      hideInSearch: true,
    },
    {
      title: t('ips'),
      width: 170,
      hideInSearch: true,
      ellipsis: true,
      render: (_, record) => {
        const ipv4 = (record.ips ?? []).find((ip) => !ip.includes(':'));
        const total = (record.ips ?? []).length;
        return ipv4 ? (
          <Tooltip title={(record.ips ?? []).join('\n')}>
            <span>
              {ipv4}
              {total > 1 && ` (+${total - 1})`}
            </span>
          </Tooltip>
        ) : (
          '-'
        );
      },
    },
    {
      title: 'CPU',
      width: 120,
      hideInSearch: true,
      render: (_, record) => renderPercent(record.cpuPercent),
    },
    {
      title: t('memory'),
      width: 120,
      hideInSearch: true,
      render: (_, record) => renderPercent(record.memoryUsedPercent),
    },
    {
      title: t('disk'),
      width: 120,
      hideInSearch: true,
      render: (_, record) => renderPercent(record.diskUsedPercent),
    },
    {
      title: t('lastHeartbeatAt'),
      dataIndex: 'lastHeartbeatAt',
      width: 180,
      valueType: 'dateTime',
      hideInSearch: true,
    },
    {
      title: t('action'),
      valueType: 'option',
      width: 110,
      fixed: 'right',
      render: (_, record) => [
        <Tooltip key="edit" title={t('edit')}>
          <a onClick={() => setEditing(record)}>
            <EditOutlined />
          </a>
        </Tooltip>,
        <Tooltip key="trend" title={t('trendTooltip')}>
          <a onClick={() => setTrendNode(record)}>
            <LineChartOutlined />
          </a>
        </Tooltip>,
        <Tooltip key="ping" title={t('pingTooltip')}>
          <a
            onClick={() =>
              record.id &&
              pingMutation.mutate({ id: record.id, action: 'ping' })
            }
          >
            <GlobalOutlined />
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
        <ListTable<AgentNode>
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

            const response = await fetchListAgentNodes(query);

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
          toolBarRender={() => []}
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
            x: 1400,
          }}
        />
      </div>
    </ContentContainer>

      <MetricsTrendDrawer data={trendNode} onClose={() => setTrendNode(undefined)} />
    </>
  );
};

export default AgentNodeManagement;
