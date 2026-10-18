import { useCallback, useEffect, useRef, useState } from 'react';
import { App, Modal, Space, Spin, Table, Tag } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import type {
  opsservicev1_Pipeline as Pipeline,
  opsservicev1_PipelineRun as PipelineRun,
} from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { fetchListPipelineRuns, fetchPipelineRunLog } from '@/api/hooks/pipeline';

interface PipelineRunsModalProps {
  pipeline?: Pipeline;
  onClose: () => void;
}

/** 流水线运行记录弹窗:列表 + 增量日志 tail(RUNNING 每 2s 追加)。 */
const PipelineRunsModal: React.FC<PipelineRunsModalProps> = ({ pipeline, onClose }) => {
  const { t } = useTranslation('pipeline');
  const { message } = App.useApp();
  const [items, setItems] = useState<PipelineRun[]>([]);
  const [loading, setLoading] = useState(false);
  const [logRun, setLogRun] = useState<PipelineRun>();
  const [logView, setLogView] = useState<{ text: string; offset: number; status?: string; error?: string }>();
  const logPreRef = useRef<HTMLPreElement>(null);

  const refresh = useCallback(async () => {
    if (!pipeline?.id) return;
    setLoading(true);
    try {
      const resp = await fetchListPipelineRuns(new PaginationQuery({ paging: { page: 1, pageSize: 100 } }));
      setItems(
        (resp.items ?? [])
          .filter((r) => r.pipelineId === pipeline.id)
          .sort((a, b) => (b.id ?? 0) - (a.id ?? 0)),
      );
    } finally {
      setLoading(false);
    }
  }, [pipeline?.id]);

  useEffect(() => {
    if (!pipeline?.id) return;
    refresh();
  }, [pipeline?.id, refresh]);

  // 有 RUNNING 时 5s 轮询列表
  useEffect(() => {
    if (!pipeline?.id) return;
    const hasRunning = items.some((r) => r.status === 'RUNNING' || r.status === 'PENDING');
    if (!hasRunning) return;
    const timer = setInterval(refresh, 5000);
    return () => clearInterval(timer);
  }, [pipeline?.id, items, refresh]);

  // 日志 tail
  const openLog = (run: PipelineRun) => {
    setLogRun(run);
    setLogView(undefined);
  };
  useEffect(() => {
    if (!logRun?.id) return;
    let alive = true;
    fetchPipelineRunLog(logRun.id, 0)
      .then((resp) => {
        if (!alive) return;
        setLogView({ text: resp.delta || '', offset: resp.newOffset ?? 0, status: resp.status, error: resp.error });
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [logRun?.id]);
  useEffect(() => {
    if (!logRun?.id) return;
    if (logView?.status === 'SUCCESS' || logView?.status === 'FAILED') return;
    const timer = setInterval(async () => {
      try {
        const resp = await fetchPipelineRunLog(logRun.id!, logView?.offset ?? 0);
        setLogView((p) => ({
          text: (p?.text || '') + (resp.delta || ''),
          offset: resp.newOffset ?? p?.offset ?? 0,
          status: resp.status,
          error: resp.error,
        }));
      } catch {
        /* 静默重试 */
      }
    }, 2000);
    return () => clearInterval(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [logRun?.id, logView?.offset, logView?.status]);
  useEffect(() => {
    if (logPreRef.current) logPreRef.current.scrollTop = logPreRef.current.scrollHeight;
  }, [logView?.text]);

  const statusTag = (status: string | undefined) => {
    switch (status) {
      case 'SUCCESS':
        return <Tag color="success">{t('statusSuccess')}</Tag>;
      case 'FAILED':
        return <Tag color="error">{t('statusFailed')}</Tag>;
      case 'RUNNING':
        return <Tag color="processing">{t('statusRunning')}</Tag>;
      case 'PENDING':
        return <Tag color="warning">{t('statusPending')}</Tag>;
      default:
        return <Tag>{status || '-'}</Tag>;
    }
  };

  const columns: ColumnsType<PipelineRun> = [
    { title: '#', dataIndex: 'id', width: 70, render: (v) => <Tag>#{v}</Tag> },
    { title: t('runStatus'), dataIndex: 'status', width: 100, render: (v) => statusTag(v) },
    {
      title: t('currentStep'),
      dataIndex: 'currentStep',
      width: 110,
      render: (v, r) => (r.status === 'RUNNING' ? `${(v ?? 0) + 1}/${r.steps?.length ?? '-'}` : '-'),
    },
    {
      title: t('stepResults'),
      dataIndex: 'stepResults',
      render: (_, r) => (
        <Space size={4} wrap>
          {(r.stepResults ?? []).map((sr, i) => (
            <Tag key={i} color={sr.status === 'SUCCESS' ? 'success' : sr.status === 'FAILED' ? 'error' : 'default'}>
              {sr.type}:{sr.status}
            </Tag>
          ))}
        </Space>
      ),
    },
    { title: t('startedAt'), dataIndex: 'startedAt', width: 165, render: fmtTime },
    { title: t('finishedAt'), dataIndex: 'finishedAt', width: 165, render: fmtTime },
    {
      title: t('action'),
      width: 110,
      render: (_, record) => [
        <a key="log" onClick={() => openLog(record)}>
          {t('viewLog')}
        </a>,
      ],
    },
  ];

  return (
    <>
      <Modal
        title={t('runsTitle', { name: pipeline?.name })}
        open={!!pipeline}
        onCancel={onClose}
        footer={null}
        width={1000}
        destroyOnClose
      >
        {loading && items.length === 0 ? (
          <Spin style={{ display: 'block', margin: '32px auto' }} />
        ) : (
          <Table<PipelineRun>
            rowKey="id"
            size="small"
            columns={columns}
            dataSource={items}
            pagination={false}
            scroll={{ y: 420 }}
            locale={{ emptyText: t('noRuns') }}
          />
        )}
      </Modal>

      <Modal
        title={
          <Space>
            {t('logTitle', { id: logRun?.id })}
            {logView?.status === 'RUNNING' || logView?.status === 'PENDING' ? (
              <Tag color="processing">{t('statusRunning')}</Tag>
            ) : null}
          </Space>
        }
        open={!!logRun}
        onCancel={() => setLogRun(undefined)}
        footer={null}
        width={900}
        destroyOnClose
      >
        <pre
          ref={logPreRef}
          style={{
            background: '#f6f6f6',
            border: '1px solid #e8e8e8',
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
          {logView?.text || t('logEmpty')}
        </pre>
      </Modal>
    </>
  );
};

function fmtTime(v: any): string {
  if (!v) return '-';
  return String(v).replace('T', ' ').slice(0, 19);
}

export default PipelineRunsModal;
