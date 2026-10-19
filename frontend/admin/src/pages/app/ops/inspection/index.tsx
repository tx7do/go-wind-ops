import { useRef, useState } from 'react';
import type { ProColumns, ActionType } from '@ant-design/pro-components';
import ListTable from '@/components/common/ListTable';
import { Button, Tag, Tooltip } from 'antd';
import { EyeOutlined, ThunderboltOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_Inspection as Inspection } from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { TABLE } from '@/config/constants';
import { fetchListInspections, useRunInspection } from '@/api/hooks/inspection';
import { useProTableScrollY } from '@/hooks/useProTableScrollY';
import ContentContainer from '@/layouts/components/PageContainer/ContentContainer';
import InspectionReportDrawer from './components/InspectionReportDrawer';

/**
 * 平台巡检报告页:发起巡检 + 历史报告查看
 */
const InspectionManagement = () => {
  const { t } = useTranslation('inspection');
  const actionRef = useRef<ActionType>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const tableScrollY = useProTableScrollY(containerRef);
  const [reportId, setReportId] = useState<number | undefined>();

  const runMutation = useRunInspection({
    onSuccess: () => {
      actionRef.current?.reload();
    },
  });

  const columns: ProColumns<Inspection>[] = [
    { title: t('id'), dataIndex: 'id', width: 90, hideInSearch: true },
    {
      title: t('status'),
      dataIndex: 'status',
      width: 110,
      hideInSearch: true,
      render: (_, r) =>
        r.status === 'FINISHED' ? (
          <Tag color="success">{t('statusFinished')}</Tag>
        ) : r.status === 'FAILED' ? (
          <Tag color="error">{t('statusFailed')}</Tag>
        ) : (
          <Tag color="processing">{t('statusRunning')}</Tag>
        ),
    },
    {
      title: t('nodes'),
      width: 120,
      hideInSearch: true,
      render: (_, r) => `${r.nodeOnline ?? 0}/${r.nodeTotal ?? 0}`,
    },
    {
      title: t('projects'),
      width: 120,
      hideInSearch: true,
      render: (_, r) => `${r.projectRunning ?? 0}/${r.projectTotal ?? 0}`,
    },
    {
      title: t('alertOpen'),
      dataIndex: 'alertOpen',
      width: 110,
      hideInSearch: true,
      render: (_, r) => (r.alertOpen ? <Tag color="warning">{r.alertOpen}</Tag> : <Tag>0</Tag>),
    },
    {
      title: t('certExpiring'),
      dataIndex: 'certExpiring',
      width: 130,
      hideInSearch: true,
      render: (_, r) => (r.certExpiring ? <Tag color="error">{r.certExpiring}</Tag> : <Tag>0</Tag>),
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
        <Tooltip key="view" title={t('viewReport')}>
          <a onClick={() => setReportId(r.id)}>
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
          <ListTable<Inspection>
            actionRef={actionRef}
            columns={columns}
            rowKey="id"
            request={async (params) => {
              const q = new PaginationQuery({
                paging: { page: params.current || 1, pageSize: params.pageSize || 10 },
                orderBy: ['-createdAt'],
              });
              const resp = await fetchListInspections(q);
              return { data: resp.items || [], total: resp.total || 0, success: true };
            }}
            search={false}
            pagination={{ defaultPageSize: TABLE.DEFAULT_PAGE_SIZE, showSizeChanger: true }}
            toolBarRender={() => [
              <Button
                key="run"
                type="primary"
                icon={<ThunderboltOutlined />}
                loading={runMutation.isPending}
                onClick={() => runMutation.mutate(undefined as never)}
              >
                {t('runNow')}
              </Button>,
            ]}
            options={{ density: true, fullScreen: true, setting: true, reload: true }}
            size="middle"
            bordered
            scroll={{ y: tableScrollY, x: 900 }}
          />
        </div>
      </ContentContainer>

      <InspectionReportDrawer inspectionId={reportId} onClose={() => setReportId(undefined)} />
    </>
  );
};

export default InspectionManagement;
