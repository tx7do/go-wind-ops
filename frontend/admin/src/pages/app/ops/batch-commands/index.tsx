import { useCallback, useMemo, useState } from 'react';
import { App, Button, Input, InputNumber, Select, Space, Table, Tag, Typography } from 'antd';
import { SendOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useQuery } from '@tanstack/react-query';
import { PaginationQuery } from '@/core';
import { fetchListAgentNodes } from '@/api/hooks/agent-node';
import { useGetScriptBatch, useBatchExecScript } from '@/api/hooks/script-batch';

/**
 * 批量命令:多台机器即时执行同一条命令。复用脚本批次链路(即席命令模式
 * command+agentIds),执行后轮询批次详情展示逐机状态与输出。
 */
const BatchCommandsPage = () => {
  const { t } = useTranslation('batch-commands');
  const { message } = App.useApp();

  const [agentIds, setAgentIds] = useState<string[]>([]);
  const [command, setCommand] = useState('');
  const [timeoutSec, setTimeoutSec] = useState<number>(60);
  const [batchId, setBatchId] = useState<number>();

  const { data: nodesResp } = useQuery({
    queryKey: ['listAgentNodesForBatch', 'all'],
    queryFn: () => fetchListAgentNodes(new PaginationQuery({ paging: { page: 1, pageSize: 200 } })),
  });
  const agentOptions = useMemo(
    () =>
      (nodesResp?.items || []).map((n) => ({
        label: n.hostname ? `${n.hostname} (${n.agentId})` : n.agentId,
        value: n.agentId || '',
        disabled: n.status === 'OFFLINE',
      })),
    [nodesResp],
  );
  const agentName = useMemo(() => {
    const m = new Map<string, string>();
    for (const n of nodesResp?.items || []) m.set(n.agentId || '', n.hostname || n.agentId || '');
    return m;
  }, [nodesResp]);

  // 批次详情轮询:RUNNING 期间 2s 一次
  const detail = useGetScriptBatch(
    { id: batchId },
    {
      enabled: Boolean(batchId),
      refetchInterval: (q) =>
        (q.state.data?.batch?.status || '').toUpperCase() === 'RUNNING' ? 2000 : false,
    },
  );

  const exec = useBatchExecScript({
    onSuccess: (resp) => {
      message.success(t('dispatched', { n: resp.dispatched ?? 0, failed: resp.failed ?? 0 }));
      if (resp.batchId) setBatchId(resp.batchId);
    },
    onError: (e) => message.error(e.message || t('execFailed')),
  });

  const onExec = useCallback(() => {
    if (!agentIds.length) {
      message.warning(t('pickAgentsFirst'));
      return;
    }
    if (!command.trim()) {
      message.warning(t('commandRequired'));
      return;
    }
    exec.mutate({ command: command, agentIds, timeoutSeconds: timeoutSec } as never);
  }, [agentIds, command, timeoutSec, exec, message, t]);

  const items = detail.data?.items || [];
  const batch = detail.data?.batch;
  const running = (batch?.status || '').toUpperCase() === 'RUNNING';

  const columns = useMemo(
    () => [
      {
        title: t('machine'),
        dataIndex: 'agentId',
        render: (v: string) => agentName.get(v) || v,
      },
      {
        title: t('statusCol'),
        dataIndex: 'status',
        width: 110,
        render: (v: string) => {
          const s = (v || '').toUpperCase();
          const color = s === 'SUCCESS' ? 'success' : s === 'FAILED' ? 'error' : 'processing';
          const text = s === 'SUCCESS' ? t('stSuccess') : s === 'FAILED' ? t('stFailed') : t('stRunning');
          return <Tag color={color}>{text}</Tag>;
        },
      },
      { title: t('exitCode'), dataIndex: 'exitCode', width: 90 },
      { title: t('duration'), dataIndex: 'duration', width: 120 },
      {
        title: t('output'),
        dataIndex: 'output',
        ellipsis: true,
        render: (v: string, r: { error?: string }) =>
          v || r.error || '-',
      },
    ],
    [agentName, t],
  );

  return (
    <div style={{ padding: 16, height: '100%', display: 'flex', flexDirection: 'column', overflow: 'auto' }}>
      <Space direction="vertical" style={{ width: '100%' }} size="middle">
        <Space wrap>
          <Select
            mode="multiple"
            showSearch
            optionFilterProp="label"
            placeholder={t('selectMachines')}
            style={{ minWidth: 420 }}
            maxTagCount="responsive"
            value={agentIds}
            options={agentOptions}
            onChange={setAgentIds}
          />
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {t('timeoutLabel')}
          </Typography.Text>
          <InputNumber
            min={1}
            max={3600}
            value={timeoutSec}
            onChange={(v) => setTimeoutSec(Number(v) || 60)}
            addonAfter="s"
            style={{ width: 140 }}
          />
        </Space>
        <Space.Compact style={{ width: '100%' }}>
          <Input.TextArea
            rows={3}
            placeholder={t('commandPlaceholder')}
            value={command}
            onChange={(e) => setCommand(e.target.value)}
          />
        </Space.Compact>
        <Space>
          <Button
            type="primary"
            icon={<SendOutlined />}
            loading={exec.isPending}
            onClick={onExec}
          >
            {t('exec')}
          </Button>
          {batch && (
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              {t('batchNo', { id: batchId })} · {t('progress', { ok: batch.successCount ?? 0, failed: batch.failedCount ?? 0, total: batch.total ?? 0 })}
            </Typography.Text>
          )}
        </Space>

        {batchId ? (
          <Table
            rowKey="requestId"
            size="small"
            columns={columns as never}
            dataSource={items}
            loading={running && !items.length}
            pagination={false}
            locale={{ emptyText: running ? t('waitingResults') : t('noResults') }}
          />
        ) : (
          <Typography.Paragraph type="secondary" style={{ fontSize: 12 }}>
            {t('hint')}
          </Typography.Paragraph>
        )}
      </Space>
    </div>
  );
};

export default BatchCommandsPage;
