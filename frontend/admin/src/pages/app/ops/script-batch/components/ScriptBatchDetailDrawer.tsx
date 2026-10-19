import { Descriptions, Drawer, Tag } from 'antd';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_ScriptBatchItem as ScriptBatchItem } from '@/api/generated/admin/service/v1';
import { useGetScriptBatch } from '@/api/hooks/script-batch';

interface Props {
  batchId?: number;
  onClose: () => void;
}

/**
 * 批次详情抽屉:批次概览 + 逐节点结果矩阵;批次进行中每 3s 轮询刷新。
 */
const ScriptBatchDetailDrawer: React.FC<Props> = ({ batchId, onClose }) => {
  const { t } = useTranslation('script-batch');

  // enabled 由 batchId 驱动;RUNNING 批次轮询,终态停止
  const { data, isFetching } = useGetScriptBatch(
    { id: batchId ?? 0 },
    {
      enabled: !!batchId,
      refetchInterval: (query) =>
        String(query.state.data?.batch?.status ?? '') === 'RUNNING' ? 3000 : false,
    },
  );

  const batch = data?.batch;

  const statusTag = (s?: string) => {
    if (s === 'SUCCESS') return <Tag color="success">{t('itemSuccess')}</Tag>;
    if (s === 'FAILED') return <Tag color="error">{t('itemFailed')}</Tag>;
    return <Tag color="processing">{t('itemPending')}</Tag>;
  };

  return (
    <Drawer
      title={t('detailTitle', { id: batchId })}
      open={!!batchId}
      onClose={onClose}
      size={860}
      destroyOnHidden
    >
      <Descriptions size="small" column={3} bordered style={{ marginBottom: 16 }}>
        <Descriptions.Item label={t('scriptName')}>{batch?.scriptName || '-'}</Descriptions.Item>
        <Descriptions.Item label={t('total')}>{batch?.total ?? '-'}</Descriptions.Item>
        <Descriptions.Item label={t('status')}>
          {batch?.status === 'FINISHED' ? (
            <Tag color="processing">{t('statusFinished')}</Tag>
          ) : (
            <Tag color="warning">{t('statusRunning')}</Tag>
          )}
        </Descriptions.Item>
        <Descriptions.Item label={t('successCount')}>
          <Tag color="success">{batch?.successCount ?? 0}</Tag>
        </Descriptions.Item>
        <Descriptions.Item label={t('failedCount')}>
          <Tag color="error">{batch?.failedCount ?? 0}</Tag>
        </Descriptions.Item>
        <Descriptions.Item label={t('createdAt')}>{batch?.createdAt || '-'}</Descriptions.Item>
      </Descriptions>

      <div style={{ overflowX: 'auto' }}>
        <table className="ant-table" style={{ width: '100%', fontSize: 13, borderCollapse: 'collapse' }}>
          <thead>
            <tr style={{ textAlign: 'left', borderBottom: '1px solid var(--ant-color-split)' }}>
              <th style={cellTh}>{t('nodeName')}</th>
              <th style={cellTh}>{t('itemStatus')}</th>
              <th style={cellTh}>{t('exitCode')}</th>
              <th style={cellTh}>{t('duration')}</th>
              <th style={cellTh}>{t('output')}</th>
            </tr>
          </thead>
          <tbody>
            {(data?.items ?? []).map((item: ScriptBatchItem, idx) => (
              <tr key={item.requestId || idx} style={{ borderBottom: '1px solid var(--ant-color-split)' }}>
                <td style={cellTd}>{item.agentId || item.nodeId || '-'}</td>
                <td style={cellTd}>{statusTag(item.status)}</td>
                <td style={cellTd}>
                  {item.exitCode === undefined || item.exitCode === null
                    ? '-'
                    : item.exitCode === 0
                      ? <span style={{ color: 'var(--ant-color-success)' }}>0</span>
                      : <span style={{ color: 'var(--ant-color-error)' }}>{item.exitCode}</span>}
                </td>
                <td style={cellTd}>{item.duration || '-'}</td>
                <td style={{ ...cellTd, maxWidth: 360 }}>
                  {item.output ? (
                    <pre
                      style={{
                        margin: 0,
                        whiteSpace: 'pre-wrap',
                        wordBreak: 'break-all',
                        maxHeight: 160,
                        overflowY: 'auto',
                        fontSize: 12,
                      }}
                    >
                      {item.output}
                    </pre>
                  ) : (
                    item.error || '-'
                  )}
                </td>
              </tr>
            ))}
            {!isFetching && (data?.items?.length ?? 0) === 0 && (
              <tr>
                <td style={cellTd} colSpan={5}>
                  {t('noItems')}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </Drawer>
  );
};

const cellTh: React.CSSProperties = {
  padding: '8px 12px',
  fontWeight: 600,
  whiteSpace: 'nowrap',
};

const cellTd: React.CSSProperties = {
  padding: '8px 12px',
  verticalAlign: 'top',
};

export default ScriptBatchDetailDrawer;
