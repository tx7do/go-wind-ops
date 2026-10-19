import { useRef, useState } from 'react';
import type { ProColumns, ActionType } from '@ant-design/pro-components';
import ListTable from '@/components/common/ListTable';
import { Button, Popconfirm, Tag, Tooltip, App } from 'antd';
import {
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
  SafetyCertificateOutlined,
} from '@ant-design/icons';
import { useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_Cert as Cert } from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { TABLE } from '@/config/constants';
import { fetchListCerts, useDeleteCert, useProbeCert } from '@/api/hooks/cert';
import { useProTableScrollY } from '@/hooks/useProTableScrollY';
import ContentContainer from '@/layouts/components/PageContainer/ContentContainer';
import CertDrawer from './components/CertDrawer';

/**
 * 证书到期台账页面:证书 CRUD + 立即探测(回填有效期)
 */
const CertManagement = () => {
  const { t } = useTranslation('cert');
  const actionRef = useRef<ActionType>(null);
  const queryClient = useQueryClient();
  const { message } = App.useApp();
  const containerRef = useRef<HTMLDivElement>(null);
  const tableScrollY = useProTableScrollY(containerRef);

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerMode, setDrawerMode] = useState<'create' | 'edit'>('create');
  const [selected, setSelected] = useState<Cert | undefined>();

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['listCerts'] });

  const deleteMutation = useDeleteCert({
    onSuccess: () => {
      message.success(t('deleteSuccess'));
      invalidate();
      actionRef.current?.reload();
    },
    onError: (e: Error) => message.error(e.message || t('deleteFailed')),
  });

  const probeMutation = useProbeCert({
    onSuccess: (resp) => {
      if (resp.error) {
        message.error(t('probeFailedWithReason', { reason: resp.error }));
      } else {
        message.success(t('probeSuccess', { days: resp.remainingDays ?? 0 }));
      }
      invalidate();
      actionRef.current?.reload();
    },
    onError: (e: Error) => message.error(e.message || t('probeFailed')),
  });

  const expiryCell = (r: Cert) => {
    if (!r.notAfter) return '-';
    const days = Math.floor((new Date(r.notAfter).getTime() - Date.now()) / 86400000);
    if (days < 0) return <Tag color="error">{t('expired')}</Tag>;
    if (days <= (r.daysBeforeExpire ?? 30)) return <Tag color="warning">{t('daysLeft', { days })}</Tag>;
    return <Tag color="success">{t('daysLeft', { days })}</Tag>;
  };

  const columns: ProColumns<Cert>[] = [
    {
      title: t('name'),
      dataIndex: 'name',
      width: 150,
      render: (_, r) => <span style={{ fontWeight: 600 }}>{r.name}</span>,
    },
    {
      title: t('target'),
      width: 200,
      hideInSearch: true,
      render: (_, r) => `${r.host}:${r.port ?? 443}`,
    },
    { title: t('host'), dataIndex: 'host', hideInSearch: true, ellipsis: true },
    {
      title: t('issuer'),
      dataIndex: 'issuer',
      width: 160,
      hideInSearch: true,
      ellipsis: true,
      render: (_, r) => r.issuer || '-',
    },
    {
      title: t('notAfter'),
      dataIndex: 'notAfter',
      width: 170,
      hideInSearch: true,
      render: (_, r) => (r.notAfter ? r.notAfter.replace('T', ' ').slice(0, 19) : '-'),
    },
    {
      title: t('remaining'),
      width: 120,
      hideInSearch: true,
      render: (_, r) => expiryCell(r),
    },
    {
      title: t('daysBeforeExpire'),
      dataIndex: 'daysBeforeExpire',
      width: 110,
      hideInSearch: true,
      render: (_, r) => t('daysBeforeValue', { days: r.daysBeforeExpire ?? 30 }),
    },
    {
      title: t('probeCron'),
      dataIndex: 'probeCron',
      width: 120,
      hideInSearch: true,
      render: (_, r) => r.probeCron || <Tag>{t('probeManual')}</Tag>,
    },
    {
      title: t('enabled'),
      width: 80,
      hideInSearch: true,
      render: (_, r) => (r.enabled ? <Tag color="success">{t('yes')}</Tag> : <Tag>{t('no')}</Tag>),
    },
    {
      title: t('action'),
      valueType: 'option',
      width: 130,
      fixed: 'right',
      render: (_, r) => [
        <Tooltip key="probe" title={t('probeNow')}>
          <a onClick={() => r.id && probeMutation.mutate({ id: r.id })}>
            <SafetyCertificateOutlined />
          </a>
        </Tooltip>,
        <Tooltip key="edit" title={t('edit')}>
          <a
            onClick={() => {
              setDrawerMode('edit');
              setSelected(r);
              setDrawerOpen(true);
            }}
          >
            <EditOutlined />
          </a>
        </Tooltip>,
        <Popconfirm
          key="delete"
          title={t('deleteConfirmTitle')}
          onConfirm={() => r.id && deleteMutation.mutate({ id: r.id })}
          okText={t('common:button.ok')}
          cancelText={t('common:button.cancel')}
        >
          <a style={{ color: 'var(--ant-color-error)' }}>
            <DeleteOutlined />
          </a>
        </Popconfirm>,
      ],
    },
  ];

  return (
    <>
      <ContentContainer heightMode="fixed" padding="16px" bottomMargin={0}>
        <div ref={containerRef} className="page-container-content">
          <ListTable<Cert>
            actionRef={actionRef}
            columns={columns}
            rowKey="id"
            request={async (params, sorter) => {
              const q = new PaginationQuery({
                paging: { page: params.current || 1, pageSize: params.pageSize || 10 },
                formValues: Object.fromEntries(
                  Object.entries(params).filter(([key]) => !['current', 'pageSize'].includes(key)),
                ),
                orderBy:
                  sorter && Object.keys(sorter).length > 0
                    ? Object.entries(sorter).map(([key, value]) => (value === 'ascend' ? key : `-${key}`))
                    : ['-createdAt'],
              });
              const resp = await fetchListCerts(q);
              return { data: resp.items || [], total: resp.total || 0, success: true };
            }}
            search={{ labelWidth: 'auto' }}
            pagination={{ defaultPageSize: TABLE.DEFAULT_PAGE_SIZE, showSizeChanger: true }}
            toolBarRender={() => [
              <Button
                key="create"
                type="primary"
                icon={<PlusOutlined />}
                onClick={() => {
                  setDrawerMode('create');
                  setSelected(undefined);
                  setDrawerOpen(true);
                }}
              >
                {t('create')}
              </Button>,
            ]}
            options={{ density: true, fullScreen: true, setting: true, reload: true }}
            size="middle"
            bordered
            scroll={{ y: tableScrollY, x: 1200 }}
          />
        </div>
      </ContentContainer>

      <CertDrawer
        open={drawerOpen}
        mode={drawerMode}
        data={selected}
        onClose={() => {
          setDrawerOpen(false);
          setSelected(undefined);
        }}
        onSuccess={() => {
          invalidate();
          actionRef.current?.reload();
        }}
      />
    </>
  );
};

export default CertManagement;
