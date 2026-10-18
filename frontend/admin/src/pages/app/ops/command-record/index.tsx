import { useRef, useState } from 'react';
import type { ProColumns, ActionType } from '@ant-design/pro-components';
import ListTable from '@/components/common/ListTable';
import { Tag } from 'antd';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_CommandRecord as CommandRecord } from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { TABLE } from '@/config/constants';
import { fetchListCommandRecords } from '@/api/hooks/command-record';
import { useProTableScrollY } from '@/hooks/useProTableScrollY';
import ContentContainer from '@/layouts/components/PageContainer/ContentContainer';

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
 * 指令台账页面(只读):所有经 MQTT 下发指令的审计与结果回执。
 */
const CommandRecordManagement = () => {
  const { t } = useTranslation('command-record');
  const actionRef = useRef<ActionType>(null);

  const containerRef = useRef<HTMLDivElement>(null);
  const tableScrollY = useProTableScrollY(containerRef);

  const [detail, setDetail] = useState<CommandRecord | undefined>();

  const columns: ProColumns<CommandRecord>[] = [
    {
      title: t('requestId'),
      dataIndex: 'requestId',
      width: 290,
      ellipsis: true,
    },
    {
      title: t('nodeId'),
      dataIndex: 'nodeId',
      width: 80,
      hideInSearch: true,
      render: (_, r) => r.nodeId || '-',
    },
    {
      title: t('agentId'),
      dataIndex: 'agentId',
      width: 150,
      hideInSearch: true,
      ellipsis: true,
    },
    {
      title: t('action'),
      dataIndex: 'action',
      width: 150,
      render: (_, r) => <Tag>{r.action}</Tag>,
    },
    {
      title: t('status'),
      dataIndex: 'status',
      width: 100,
      render: (_, r) => statusTag(r.status, t),
    },
    {
      title: t('error'),
      dataIndex: 'error',
      width: 200,
      hideInSearch: true,
      ellipsis: true,
      render: (_, r) => r.error || '-',
    },
    {
      title: t('createdAt'),
      dataIndex: 'createdAt',
      width: 165,
      valueType: 'dateTime',
      hideInSearch: true,
      sorter: true,
    },
    {
      title: t('finishedAt'),
      dataIndex: 'finishedAt',
      width: 165,
      valueType: 'dateTime',
      hideInSearch: true,
    },
  ];

  return (
    <>
      <ContentContainer heightMode="fixed" padding="16px" bottomMargin={0}>
        <div ref={containerRef} className="page-container-content">
          <ListTable<CommandRecord>
            headerTitle={t('moduleName')}
            actionRef={actionRef}
            columns={columns}
            onRow={(record) => ({
              onClick: () => setDetail(record),
              style: { cursor: 'pointer' },
            })}
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
                    : ['-createdAt'],
              });

              const response = await fetchListCommandRecords(query);

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

      {/* 详情弹窗(载荷/结果 JSON) */}
      {detail && (
        <Modalish record={detail} onClose={() => setDetail(undefined)} />
      )}
    </>
  );
};

function Modalish({ record, onClose }: { record: CommandRecord; onClose: () => void }) {
  const { t } = useTranslation('command-record');
  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: 1000,
        background: 'rgba(0,0,0,0.45)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
      }}
      onClick={onClose}
    >
      <div
        style={{
          background: 'var(--ant-color-bg-container)',
          borderRadius: 8,
          padding: 20,
          width: 720,
          maxHeight: '80vh',
          overflow: 'auto',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <h3 style={{ marginTop: 0 }}>
          {t('detailTitle')} #{record.id}
        </h3>
        <table style={{ width: '100%', fontSize: 13, borderCollapse: 'collapse' }}>
          <tbody>
            <tr><td style={tdStyle}>{t('requestId')}</td><td style={tdStyle}>{record.requestId}</td></tr>
            <tr><td style={tdStyle}>{t('nodeId')}</td><td style={tdStyle}>{record.nodeId ?? '-'}</td></tr>
            <tr><td style={tdStyle}>{t('agentId')}</td><td style={tdStyle}>{record.agentId}</td></tr>
            <tr><td style={tdStyle}>{t('action')}</td><td style={tdStyle}>{record.action}</td></tr>
            <tr><td style={tdStyle}>{t('status')}</td><td style={tdStyle}>{statusTag(record.status, t)}</td></tr>
            <tr><td style={tdStyle}>{t('operatorId')}</td><td style={tdStyle}>{record.operatorId ?? '-'}</td></tr>
            <tr><td style={tdStyle}>{t('createdAt')}</td><td style={tdStyle}>{record.createdAt ?? '-'}</td></tr>
            <tr><td style={tdStyle}>{t('finishedAt')}</td><td style={tdStyle}>{record.finishedAt ?? '-'}</td></tr>
            <tr><td style={tdStyle}>{t('error')}</td><td style={tdStyle}>{record.error || '-'}</td></tr>
          </tbody>
        </table>
        <div style={{ marginTop: 12, fontWeight: 600 }}>{t('payload')}</div>
        <pre style={preStyle}>{prettyJSON(record.payload) || '-'}</pre>
        <div style={{ marginTop: 12, fontWeight: 600 }}>{t('result')}</div>
        <pre style={preStyle}>{prettyJSON(record.result) || '-'}</pre>
      </div>
    </div>
  );
}

const tdStyle: React.CSSProperties = {
  padding: '6px 8px',
  borderTop: '1px solid var(--ant-color-border)',
  verticalAlign: 'top',
};

const preStyle: React.CSSProperties = {
  background: 'var(--ant-color-bg-layout, #f6f6f6)',
  border: '1px solid var(--ant-color-border)',
  borderRadius: 6,
  padding: 10,
  maxHeight: 220,
  overflow: 'auto',
  fontSize: 12,
  whiteSpace: 'pre-wrap',
  wordBreak: 'break-all',
  margin: '4px 0 0',
};

function prettyJSON(raw: string | undefined): string {
  if (!raw) return '';
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}

export default CommandRecordManagement;
