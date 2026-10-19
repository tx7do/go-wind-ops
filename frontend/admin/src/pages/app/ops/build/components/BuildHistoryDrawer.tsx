import { useEffect, useRef, useState } from 'react';
import { Modal, Spin, Table, Tag, Select, App } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { DownloadOutlined, UndoOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type {
  opsservicev1_Build as Build,
  opsservicev1_BuildHistory as BuildHistory,
} from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import {
  fetchListBuildHistories,
  fetchArtifactDownloadUrl,
  fetchBuildHistoryLog,
} from '@/api/hooks/build-history';
import { fetchListDistributions, useTriggerDistribution } from '@/api/hooks/distribution';

interface BuildHistoryDrawerProps {
  build?: Build;
  onClose: () => void;
}

/**
 * 构建历史抽屉:列出该构建配置的执行历史,支持看日志/下载产物/回滚分发。
 * 回滚 = 用该构建号的产物重新触发一次关联的分发配置。
 */
const BuildHistoryDrawer: React.FC<BuildHistoryDrawerProps> = ({ build, onClose }) => {
  const { t } = useTranslation('build-history');
  const { message } = App.useApp();
  const [items, setItems] = useState<BuildHistory[]>([]);
  const [loading, setLoading] = useState(false);
  const [logRecord, setLogRecord] = useState<BuildHistory | undefined>();

  // 日志增量 tail:打开弹窗全量拉一次,运行中每 2s 追加新片段直至终态
  const [logView, setLogView] = useState<{ text: string; offset: number; status?: string; error?: string }>();
  const logPreRef = useRef<HTMLPreElement>(null);
  const openLog = (record: BuildHistory) => {
    setLogRecord(record);
    setLogView(undefined);
  };
  useEffect(() => {
    if (!logRecord?.id) return;
    let alive = true;
    fetchBuildHistoryLog(logRecord.id, 0)
      .then((resp) => {
        if (!alive) return;
        setLogView({
          text: resp.delta || '',
          offset: resp.newOffset ?? 0,
          status: resp.status,
          error: resp.error,
        });
      })
      .catch(() => { /* 网络抖动静默,由轮询兜底 */ });
    return () => { alive = false; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [logRecord?.id]);
  useEffect(() => {
    if (!logRecord?.id) return;
    if (logView?.status === 'SUCCESS' || logView?.status === 'FAILED') return;
    const timer = setInterval(async () => {
      try {
        const resp = await fetchBuildHistoryLog(logRecord.id!, logView?.offset ?? 0);
        setLogView((p) => ({
          text: (p?.text || '') + (resp.delta || ''),
          offset: resp.newOffset ?? p?.offset ?? 0,
          status: resp.status,
          error: resp.error,
        }));
      } catch { /* 静默重试 */ }
    }, 2000);
    return () => clearInterval(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [logRecord?.id, logView?.offset, logView?.status]);
  // 自动滚动到底
  useEffect(() => {
    if (logPreRef.current) logPreRef.current.scrollTop = logPreRef.current.scrollHeight;
  }, [logView?.text]);

  // 回滚:选分发配置
  const [rollbackFor, setRollbackFor] = useState<BuildHistory | undefined>();
  const [distOptions, setDistOptions] = useState<{ label: string; value: number }[]>([]);
  const [selectedDistId, setSelectedDistId] = useState<number | undefined>();
  const [rollbackLoading, setRollbackLoading] = useState(false);
  const triggerMutation = useTriggerDistribution({
    onSuccess: (resp) => {
      message.success(t('rollbackSent', { recordId: resp.recordId }));
      setRollbackFor(undefined);
    },
    onError: (e: Error) => message.error(e.message || t('rollbackFailed')),
  });

  useEffect(() => {
    if (!build?.id) return;
    setLoading(true);
    fetchListBuildHistories(new PaginationQuery({ paging: { page: 1, pageSize: 50 } }))
      .then((resp) => {
        setItems(
          (resp.items ?? [])
            .filter((h) => h.buildId === build.id)
            .sort((a, b) => (b.buildNo ?? 0) - (a.buildNo ?? 0)),
        );
      })
      .finally(() => setLoading(false));
  }, [build?.id]);

  // 打开回滚弹窗:拉取关联的分发配置
  const openRollback = async (record: BuildHistory) => {
    setRollbackFor(record);
    try {
      const resp = await fetchListDistributions(
        new PaginationQuery({ paging: { page: 1, pageSize: 100 } }),
      );
      const matching = (resp.items ?? []).filter((d) => d.buildId === build?.id);
      setDistOptions(matching.map((d) => ({ label: d.name || String(d.id), value: d.id ?? 0 })));
      if (matching.length > 0) setSelectedDistId(matching[0].id);
      else setSelectedDistId(undefined);
    } catch {
      setDistOptions([]);
    }
  };

  const confirmRollback = () => {
    if (!rollbackFor?.id || !selectedDistId) {
      message.warning(t('selectDistRequired'));
      return;
    }
    setRollbackLoading(true);
    triggerMutation.mutate(
      { id: selectedDistId, historyId: rollbackFor.id },
      { onSettled: () => setRollbackLoading(false) },
    );
  };

  const download = async (record: BuildHistory) => {
    try {
      const resp = await fetchArtifactDownloadUrl(record.id ?? 0);
      window.open(resp.url, '_blank');
    } catch (e) {
      console.error('artifact url failed:', e);
    }
  };

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
        return <Tag>{t('statusUnknown')}</Tag>;
    }
  };

  const columns: ColumnsType<BuildHistory> = [
    {
      title: t('buildNo'),
      dataIndex: 'buildNo',
      width: 90,
      render: (v: number) => <Tag color="blue">#{v}</Tag>,
    },
    {
      title: t('status'),
      dataIndex: 'status',
      width: 100,
      render: (v: string) => statusTag(v),
    },
    { title: t('branch'), dataIndex: 'branch', width: 110, ellipsis: true },
    {
      title: t('artifactSize'),
      dataIndex: 'artifactSize',
      width: 100,
      render: (v: number) => (v ? `${Math.ceil(v / 1024)} KB` : '-'),
    },
    { title: t('createdAt'), dataIndex: 'createdAt', width: 170, render: fmtTime },
    { title: t('finishedAt'), dataIndex: 'finishedAt', width: 170, render: fmtTime },
    {
      title: t('action'),
      width: 210,
      render: (_: any, record: BuildHistory) => [
        <a key="log" onClick={() => openLog(record)}>
          {t('viewLog')}
        </a>,
        record.status === 'SUCCESS' && record.artifactObjectKey ? (
          <a key="dl" onClick={() => download(record)}>
            <DownloadOutlined /> {t('download')}
          </a>
        ) : null,
        record.status === 'SUCCESS' && record.artifactObjectKey ? (
          <a key="rollback" onClick={() => openRollback(record)}>
            <UndoOutlined /> {t('rollback')}
          </a>
        ) : null,
      ],
    },
  ];

  return (
    <>
      <Modal
        title={t('title', { name: build?.name })}
        open={!!build}
        onCancel={onClose}
        footer={null}
        width={1080}
        destroyOnHidden
      >
        {loading ? (
          <Spin style={{ display: 'block', margin: '32px auto' }} />
        ) : (
          <Table<BuildHistory>
            rowKey="id"
            size="small"
            columns={columns}
            dataSource={items}
            pagination={false}
            scroll={{ y: 420 }}
          />
        )}
      </Modal>

      {/* 日志查看弹窗 */}
      <Modal
        title={t('logTitle', { no: logRecord?.buildNo })}
        open={!!logRecord}
        onCancel={() => setLogRecord(undefined)}
        footer={null}
        width={900}
        destroyOnHidden
      >
        <pre
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
          {logRecord?.log || t('logEmpty')}
          {logRecord?.error ? `\n[ERROR] ${logRecord.error}` : ''}
        </pre>
      </Modal>

      {/* 回滚分发弹窗:选分发配置 → 用该构建号的产物重新分发 */}
      <Modal
        title={t('rollbackTitle', { no: rollbackFor?.buildNo })}
        open={!!rollbackFor}
        onCancel={() => setRollbackFor(undefined)}
        onOk={confirmRollback}
        okText={t('rollbackConfirm')}
        confirmLoading={rollbackLoading}
        width={420}
        destroyOnHidden
      >
        <div style={{ marginBottom: 8 }}>{t('rollbackDistLabel')}</div>
        <Select
          style={{ width: '100%' }}
          placeholder={t('rollbackDistPlaceholder')}
          value={selectedDistId}
          onChange={setSelectedDistId}
          options={distOptions}
        />
        {distOptions.length === 0 && (
          <div style={{ marginTop: 8, color: 'var(--ant-color-error)', fontSize: 12 }}>
            {t('noMatchingDist')}
          </div>
        )}
      </Modal>
    </>
  );
};

function fmtTime(v: any): string {
  if (!v) return '-';
  return String(v).replace('T', ' ').slice(0, 19);
}

export default BuildHistoryDrawer;
