import { useRef, useState } from 'react';
import type { ProColumns, ActionType } from '@ant-design/pro-components';
import ListTable from '@/components/common/ListTable';
import { Tag, Tooltip } from 'antd';
import { EyeOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_ScriptBatch as ScriptBatch } from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { TABLE } from '@/config/constants';
import { fetchListScriptBatches } from '@/api/hooks/script-batch';
import { useProTableScrollY } from '@/hooks/useProTableScrollY';
import ContentContainer from '@/layouts/components/PageContainer/ContentContainer';
import ScriptBatchDetailDrawer from './components/ScriptBatchDetailDrawer';

/**
 * 脚本批量执行页面:批次列表 + 逐节点结果矩阵
 */
const ScriptBatchManagement = () => {
  const { t } = useTranslation('script-batch');
  const actionRef = useRef<ActionType>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const tableScrollY = useProTableScrollY(containerRef);
  const [detailId, setDetailId] = useState<number | undefined>();

  const columns: ProColumns<ScriptBatch>[] = [
    {
      title: t('id'),
      dataIndex: 'id',
      width: 80,
      hideInSearch: true,
    },
    {
      title: t('scriptName'),
      dataIndex: 'scriptName',
      width: 180,
      render: (_, r) => <span style={{ fontWeight: 600 }}>{r.scriptName || '-'}</span>,
    },
    {
      title: t('total'),
      dataIndex: 'total',
      width: 90,
      hideInSearch: true,
    },
    {
      title: t('successCount'),
      dataIndex: 'successCount',
      width: 100,
      hideInSearch: true,
      render: (_, r) => (r.successCount ? <Tag color="success">{r.successCount}</Tag> : <Tag>0</Tag>),
    },
    {
      title: t('failedCount'),
      dataIndex: 'failedCount',
      width: 100,
      hideInSearch: true,
      render: (_, r) => (r.failedCount ? <Tag color="error">{r.failedCount}</Tag> : <Tag>0</Tag>),
    },
    {
      title: t('status'),
      dataIndex: 'status',
      width: 110,
      hideInSearch: true,
      render: (_, r) =>
        r.status === 'FINISHED' ? (
          <Tag color="processing">{t('statusFinished')}</Tag>
        ) : (
          <Tag color="warning">{t('statusRunning')}</Tag>
        ),
    },
    {
      title: t('createdAt'),
      dataIndex: 'createdAt',
      width: 170,
      valueType: 'dateTime',
      hideInSearch: true,
    },
    {
      title: t('action'),
      valueType: 'option',
      width: 90,
      fixed: 'right',
      render: (_, r) => [
        <Tooltip key="detail" title={t('detail')}>
          <a onClick={() => setDetailId(r.id)}>
            <EyeOutlined />
          </a>
        </Tooltip>,
      ],
    },
  ];

  return (
    <>
      <ContentContainer heightMode="fixed" padding="16px" bottomMargin={0}>
        <div ref={containerRef} className="page-container-content">
          <ListTable<ScriptBatch>
            actionRef={actionRef}
            columns={columns}
            rowKey="id"
            request={async (params, sorter) => {
              const q = new PaginationQuery({
                paging: { page: params.current || 1, pageSize: params.pageSize || 10 },
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
              const resp = await fetchListScriptBatches(q);
              return { data: resp.items || [], total: resp.total || 0, success: true };
            }}
            search={{ labelWidth: 'auto' }}
            pagination={{
              defaultPageSize: TABLE.DEFAULT_PAGE_SIZE,
              showSizeChanger: true,
            }}
            options={{ density: true, fullScreen: true, setting: true, reload: true }}
            size="middle"
            bordered
            scroll={{ y: tableScrollY, x: 900 }}
          />
        </div>
      </ContentContainer>

      <ScriptBatchDetailDrawer
        batchId={detailId}
        onClose={() => setDetailId(undefined)}
      />
    </>
  );
};

export default ScriptBatchManagement;
