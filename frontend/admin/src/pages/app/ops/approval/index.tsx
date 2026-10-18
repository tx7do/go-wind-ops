import { useRef } from 'react';
import type { ProColumns, ActionType } from '@ant-design/pro-components';
import ListTable from '@/components/common/ListTable';
import { Popconfirm, Tag, App } from 'antd';
import { useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_Approval as Approval } from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { TABLE } from '@/config/constants';
import { fetchListApprovals, useApproveApproval, useRejectApproval } from '@/api/hooks/approval';
import { useProTableScrollY } from '@/hooks/useProTableScrollY';
import ContentContainer from '@/layouts/components/PageContainer/ContentContainer';

const ApprovalManagement = () => {
  const { t } = useTranslation('approval');
  const actionRef = useRef<ActionType>(null);
  const queryClient = useQueryClient();
  const { message } = App.useApp();
  const containerRef = useRef<HTMLDivElement>(null);
  const tableScrollY = useProTableScrollY(containerRef);

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['listApprovals'] });

  const approveMutation = useApproveApproval({
    onSuccess: () => { message.success(t('approveSuccess')); invalidate(); actionRef.current?.reload(); },
    onError: (e: Error) => message.error(e.message || t('approveFailed')),
  });
  const rejectMutation = useRejectApproval({
    onSuccess: () => { message.success(t('rejectSuccess')); invalidate(); actionRef.current?.reload(); },
    onError: (e: Error) => message.error(e.message || t('rejectFailed')),
  });

  const statusTag = (v: string | undefined) => {
    switch (v) {
      case 'APPROVED': return <Tag color="success">{t('statusApproved')}</Tag>;
      case 'REJECTED': return <Tag color="error">{t('statusRejected')}</Tag>;
      case 'PENDING': return <Tag color="processing">{t('statusPending')}</Tag>;
      default: return <Tag>{v}</Tag>;
    }
  };

  const columns: ProColumns<Approval>[] = [
    { title: t('operationType'), dataIndex: 'operationType', width: 130, render: (_, r) => <Tag>{r.operationType}</Tag> },
    { title: t('operationName'), dataIndex: 'operationName', width: 200, ellipsis: true },
    { title: t('operationRefId'), dataIndex: 'operationRefId', width: 110, hideInSearch: true },
    { title: t('status'), dataIndex: 'status', width: 100, render: (_, r) => statusTag(r.status) },
    { title: t('requesterId'), dataIndex: 'requesterId', width: 90, hideInSearch: true },
    { title: t('approverId'), dataIndex: 'approverId', width: 90, hideInSearch: true },
    { title: t('rejectReason'), dataIndex: 'rejectReason', width: 180, hideInSearch: true, ellipsis: true },
    { title: t('createdAt'), dataIndex: 'createdAt', width: 165, valueType: 'dateTime' },
    { title: t('resolvedAt'), dataIndex: 'resolvedAt', width: 165, valueType: 'dateTime', hideInSearch: true },
    {
      title: t('action'), valueType: 'option', width: 150,
      render: (_, r) => r.status === 'PENDING' ? [
        <Popconfirm key="approve" title={t('approveConfirm')} onConfirm={() => r.id && approveMutation.mutate({ id: r.id })} okText="OK" cancelText="Cancel">
          <a style={{ color: 'var(--ant-color-success)' }}>{t('approve')}</a>
        </Popconfirm>,
        <Popconfirm key="reject" title={t('rejectConfirm')} onConfirm={() => r.id && rejectMutation.mutate({ id: r.id })} okText="OK" cancelText="Cancel">
          <a style={{ color: 'var(--ant-color-error)' }}>{t('reject')}</a>
        </Popconfirm>,
      ] : [],
    },
  ];

  return (
    <ContentContainer heightMode="fixed" padding="16px" bottomMargin={0}>
      <div ref={containerRef} className="page-container-content">
        <ListTable<Approval>
          actionRef={actionRef} columns={columns} rowKey="id"
          headerTitle={t('moduleName')}
          request={async (params, sorter) => {
            const q = new PaginationQuery({
              paging: { page: params.current || 1, pageSize: params.pageSize || 10 },
              orderBy: sorter && Object.keys(sorter).length > 0 ? Object.entries(sorter).map(([k, v]) => v === 'ascend' ? k : `-${k}`) : ['-createdAt'],
            });
            const resp = await fetchListApprovals(q);
            return { data: resp.items || [], total: resp.total || 0, success: true };
          }}
          pagination={{ defaultPageSize: TABLE.DEFAULT_PAGE_SIZE, showSizeChanger: true }}
          toolBarRender={() => []}
          options={{ density: true, fullScreen: true, setting: true, reload: true }}
          size="middle" bordered scroll={{ y: tableScrollY, x: 1300 }}
        />
      </div>
    </ContentContainer>
  );
};
export default ApprovalManagement;
