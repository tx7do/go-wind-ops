import { useState } from 'react';
import { Modal, Tag } from 'antd';
import { useTranslation } from 'react-i18next';
import type {
  opsservicev1_Distribution as Distribution,
  opsservicev1_DistributionRecord as DistributionRecord,
} from '@/api/generated/admin/service/v1';
import { useListDistributionRecords } from '@/api/hooks/distribution';
import { PaginationQuery } from '@/core';

interface DistributionRecordDrawerProps {
  distribution?: Distribution;
  onClose: () => void;
}

const statusTag = (status: string | undefined, t: (k: string) => string) => {
  switch (status) {
    case 'SUCCESS':
      return <Tag color="success">{t('statusSuccess')}</Tag>;
    case 'FAILED':
      return <Tag color="error">{t('statusFailed')}</Tag>;
    case 'PENDING':
      return <Tag color="warning">{t('statusPending')}</Tag>;
    default:
      return <Tag>{status ?? '-'}</Tag>;
  }
};

/**
 * 分发执行记录抽屉:该分发配置的触发历史(轮询刷新跟进 PENDING 终态)。
 */
const DistributionRecordDrawer: React.FC<DistributionRecordDrawerProps> = ({
  distribution,
  onClose,
}) => {
  const { t } = useTranslation('distribution');
  const [detail, setDetail] = useState<DistributionRecord | undefined>();

  const query = useListDistributionRecords(
    new PaginationQuery({ paging: { page: 1, pageSize: 50 } }),
    {
      enabled: !!distribution,
      refetchInterval: (q: any) =>
        (q.state.data?.items ?? []).some((r: DistributionRecord) => r.status === 'PENDING')
          ? 4000
          : false,
    },
  );

  const records = (query.data?.items ?? []).filter(
    (r) => !distribution?.id || r.distributionId === distribution.id,
  );

  return (
    <>
      <Modal
        title={t('recordsTitle', { name: distribution?.name })}
        open={!!distribution}
        onCancel={onClose}
        footer={null}
        width={860}
        destroyOnClose
      >
        {records.length === 0 ? (
          <div style={{ textAlign: 'center', padding: 24, color: '#999' }}>{t('noRecords')}</div>
        ) : (
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
            <thead>
              <tr style={{ textAlign: 'left', color: '#999' }}>
                <th style={cellStyle}>#{t('buildHistoryId')}</th>
                <th style={cellStyle}>{t('status')}</th>
                <th style={cellStyle}>{t('error')}</th>
                <th style={cellStyle}>{t('createdAt')}</th>
                <th style={cellStyle}></th>
              </tr>
            </thead>
            <tbody>
              {records.map((r) => (
                <tr
                  key={r.id}
                  style={{ borderTop: '1px solid var(--ant-color-border)', cursor: 'pointer' }}
                  onClick={() => setDetail(r)}
                >
                  <td style={cellStyle}>#{r.buildHistoryId}</td>
                  <td style={cellStyle}>{statusTag(r.status, t)}</td>
                  <td style={{ ...cellStyle, maxWidth: 260, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {r.error || '-'}
                  </td>
                  <td style={cellStyle}>{formatTime(r.createdAt)}</td>
                  <td style={cellStyle}>
                    <a onClick={(e) => { e.stopPropagation(); setDetail(r); }}>{t('detail')}</a>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Modal>

      {/* 记录详情(含 result JSON) */}
      <Modal
        title={t('detailTitle', { id: detail?.id })}
        open={!!detail}
        onCancel={() => setDetail(undefined)}
        footer={null}
        width={720}
        destroyOnClose
      >
        {detail && (
          <>
            <div style={{ marginBottom: 8 }}>
              {statusTag(detail.status, t)}
              {detail.error && <Tag color="error">{detail.error}</Tag>}
            </div>
            <pre
              style={{
                background: '#f6f6f6',
                border: '1px solid #e8e8e8',
                borderRadius: 8,
                padding: 12,
                maxHeight: '50vh',
                overflow: 'auto',
                fontSize: 12,
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-all',
                margin: 0,
              }}
            >
              {prettyJSON(detail.result) || t('noResult')}
            </pre>
          </>
        )}
      </Modal>
    </>
  );
};

const cellStyle: React.CSSProperties = { padding: '8px 6px' };

function formatTime(v: any): string {
  if (!v) return '-';
  const s = typeof v === 'string' ? v : String(v);
  return s.replace('T', ' ').slice(0, 19);
}

function prettyJSON(raw: string | undefined): string {
  if (!raw) return '';
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}

export default DistributionRecordDrawer;
