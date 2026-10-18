import { useMemo, useState } from 'react';
import { App, Button, Drawer, Input, InputNumber, Modal, Select, Space, Spin, Table, Tag, Tooltip, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { CaretRightOutlined, ReloadOutlined } from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_AgentPlugin as AgentPlugin } from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { fetchListAgentNodes } from '@/api/hooks/agent-node';
import { useExecAgentPlugin, useListAgentPlugins } from '@/api/hooks/plugin';

const KIND_COLOR: Record<string, string> = {
  bat: 'purple',
  ps1: 'purple',
  sh: 'green',
  python: 'blue',
  binary: 'default',
};

const PluginPage = () => {
  const { t } = useTranslation('plugin');
  const { message } = App.useApp();

  const [agentId, setAgentId] = useState<string>();
  const [execTarget, setExecTarget] = useState<AgentPlugin | undefined>();
  const [execArgs, setExecArgs] = useState('');
  const [execTimeout, setExecTimeout] = useState<number>(60);
  const [result, setResult] = useState<opsservicev1_ExecAgentPluginResponse>();

  const { data: nodesResp } = useQuery({
    queryKey: ['listAgentNodesForPlugin', 'all'],
    queryFn: () => fetchListAgentNodes(new PaginationQuery({ paging: { page: 1, pageSize: 200 } })),
  });
  const agentOptions = useMemo(
    () => (nodesResp?.items || []).map((n) => ({ label: n.hostname ? `${n.hostname} (${n.agentId})` : n.agentId, value: n.agentId || '' })),
    [nodesResp],
  );

  const listing = useListAgentPlugins({ agentId });
  const plugins: AgentPlugin[] = useMemo(() => listing.data?.plugins || [], [listing.data]);

  const execMutation = useExecAgentPlugin({
    onSuccess: (resp) => {
      setExecTarget(undefined);
      setExecArgs('');
      setResult(resp);
    },
    onError: (e: Error) => message.error(e.message || t('execFailed')),
  });

  const columns: ColumnsType<AgentPlugin> = [
    {
      title: t('action'),
      dataIndex: 'action',
      width: 220,
      render: (v) => <Typography.Text code>{v}</Typography.Text>,
    },
    { title: t('kind'), dataIndex: 'kind', width: 100, render: (v) => <Tag color={KIND_COLOR[v] ?? 'default'}>{v}</Tag> },
    { title: t('file'), dataIndex: 'file', ellipsis: true },
    { title: t('size'), dataIndex: 'size', width: 110, render: (v) => `${Math.ceil((v ?? 0) / 1024)} KB` },
    { title: t('modified'), dataIndex: 'modified', width: 170, render: (v) => String(v || '-').replace('T', ' ').slice(0, 19) },
    {
      title: t('actionCol'),
      width: 100,
      render: (_, record) => (
        <a onClick={() => setExecTarget(record)}>
          <CaretRightOutlined /> {t('exec')}
        </a>
      ),
    },
  ];

  return (
    <div style={{ padding: 16 }}>
      <Space direction="vertical" style={{ width: '100%' }} size="middle">
        <Space wrap>
          <Select
            showSearch
            optionFilterProp="label"
            placeholder={t('selectAgent')}
            style={{ minWidth: 260 }}
            value={agentId}
            options={agentOptions}
            onChange={(v) => setAgentId(v)}
          />
          <Button onClick={() => listing.refetch()} disabled={!agentId} loading={listing.isFetching}>
            <ReloadOutlined /> {t('refresh')}
          </Button>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {t('hint')}
          </Typography.Text>
        </Space>

        {!agentId ? (
          <div style={{ padding: '48px 0', textAlign: 'center', color: 'var(--ant-color-text-tertiary)' }}>
            {t('pickAgentFirst')}
          </div>
        ) : listing.isFetching ? (
          <Spin style={{ display: 'block', margin: '48px auto' }} />
        ) : (
          <Table
            rowKey="action"
            size="small"
            columns={columns}
            dataSource={plugins}
            pagination={false}
            locale={{ emptyText: t('emptyPlugins') }}
          />
        )}
      </Space>

      <Drawer
        title={`${t('execTitle')}: ${execTarget?.action || ''}`}
        width={520}
        open={!!execTarget}
        onClose={() => setExecTarget(undefined)}
        extra={
          <Button
            type="primary"
            loading={execMutation.isPending}
            disabled={!agentId}
            onClick={() =>
              agentId &&
              execTarget &&
              execMutation.mutate({
                agentId,
                action: execTarget.action || '',
                args: execArgs.split('\n').map((x) => x.trim()).filter(Boolean),
                timeoutSeconds: execTimeout,
              })
            }
          >
            {t('exec')}
          </Button>
        }
      >
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Typography.Text type="secondary">{t('argsHint')}</Typography.Text>
          <Input.TextArea
            rows={6}
            value={execArgs}
            onChange={(e) => setExecArgs(e.target.value)}
            styles={{ textarea: { fontFamily: 'monospace' } }}
          />
          <Space>
            <span>{t('timeout')}:</span>
            <InputNumber min={1} max={600} value={execTimeout} onChange={(v) => setExecTimeout(v ?? 60)} />
            <span>s</span>
          </Space>
        </Space>
      </Drawer>

      <Modal
        title={
          <Space>
            {t('resultTitle', { action: result?.action })}
            {result && (
              <Tooltip title={result.duration}>
                <Tag color={result.exitCode === 0 ? 'success' : 'error'}>
                  exit={result.exitCode}
                </Tag>
              </Tooltip>
            )}
          </Space>
        }
        open={!!result}
        onCancel={() => setResult(undefined)}
        footer={null}
        width={820}
        destroyOnClose
      >
        <pre
          style={{
            background: '#141414',
            color: '#d4d4d4',
            borderRadius: 8,
            padding: 12,
            maxHeight: '60vh',
            overflow: 'auto',
            fontSize: 12,
            lineHeight: 1.6,
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-all',
            margin: 0,
          }}
        >
          {result?.output || t('emptyOutput')}
        </pre>
      </Modal>
    </div>
  );
};

export default PluginPage;
