import { useEffect, useState } from 'react';
import { Modal, Spin, Table, Tag } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { DownloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type {
  opsservicev1_Build as Build,
  opsservicev1_BuildHistory as BuildHistory,
} from '@/api/generated/admin/service/v1';
import { fetchListBuildHistories, fetchArtifactDownloadUrl } from '@/api/hooks/build-history';
import { PaginationQuery } from '@/core';

interface BuildHistoryDrawerProps {
  build?: Build;
  onClose: () => void;
}

/**
 * 构建历史抽屉:列出该构建配置的执行历史,支持看日志/下载产物。
 * 数据按 buildId 客户端过滤(台账量小;后续可加服务端过滤参数)。
 */
const BuildHistoryDrawer: React.FC<BuildHistoryDrawerProps> = ({ build, onClose }) => {
  const { t } = useTranslation('build-history');
  const [items, setItems] = useState<BuildHistory[]>([]);
  const [loading, setLoading] = useState(false);
  const [logRecord, setLogRecord] = useState<BuildHistory | undefined>();

  // 加载 + RUNNING 时自动轮询(3s)
  const load = async () => {
    if (!build?.id) return;
    fetchListBuildHistories(new PaginationQuery({ paging: { page: 1, pageSize: 50 } }))
      .then((resp) => {
        setItems(
          (resp.items ?? [])
            .filter((h) => h.buildId === build.id)
            .sort((a, b) => (b.buildNo ?? 0) - (a.buildNo ?? 0)),
        );
      })
      .finally(() => setLoading(false));
  };

  useEffect(() => { load(); }, [build?.id]);

  useEffect(() => {
    if (!build?.id) return;
    const hasActive = items.some((h) => h.status === 'PENDING' || h.status === 'RUNNING');
    if (!hasActive) return;
    const timer = setInterval(load, 3000);
    return () => clearInterval(timer);
  }, [build?.id, items]);

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

  const download = async (record: BuildHistory) => {
    try {
      const resp = await fetchArtifactDownloadUrl(record.id ?? 0);
      window.open(resp.url, '_blank');
    } catch (e) {
      console.error('artifact url failed:', e);
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
      width: 150,
      render: (_, record) => [
        <a key="log" onClick={() => setLogRecord(record)}>
          {t('viewLog')}
        </a>,
        record.status === 'SUCCESS' && record.artifactObjectKey ? (
          <a key="dl" onClick={() => download(record)}>
            <DownloadOutlined /> {t('download')}
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
        destroyOnClose
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
        destroyOnClose
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
    </>
  );
};

function fmtTime(v: any): string {
  if (!v) return '-';
  return String(v).replace('T', ' ').slice(0, 19);
}

export default BuildHistoryDrawer;
