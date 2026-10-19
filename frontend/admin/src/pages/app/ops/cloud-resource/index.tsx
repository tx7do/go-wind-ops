import { useMemo, useRef, useState, useEffect } from 'react';
import type { ProColumns, ActionType } from '@ant-design/pro-components';
import ListTable from '@/components/common/ListTable';
import { App, Button, Tag, Typography } from 'antd';
import { CloudDownloadOutlined, SyncOutlined } from '@ant-design/icons';
import { useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_CloudInstance as CloudInstance } from '@/api/generated/admin/service/v1';
import { TABLE } from '@/config/constants';
import { PaginationQuery } from '@/core';
import { fetchListCloudAccounts } from '@/api/hooks/cloud-account';
import { fetchListCloudInstances, useSyncCloudAccountInstances } from '@/api/hooks/cloud-instance';
import { useProTableScrollY } from '@/hooks/useProTableScrollY';
import ContentContainer from '@/layouts/components/PageContainer/ContentContainer';

/**
 * 云资源实例清单(只读):数据来自"云账号 → 同步资源"的全地域拉取,
 * 已匹配列 = 实例 IP 命中了某台 agent 节点。
 */
const CloudResourceManagement = () => {
  const { t } = useTranslation('cloud-resource');
  const { t: tAccount } = useTranslation('cloud-account');
  const actionRef = useRef<ActionType>(null);
  const queryClient = useQueryClient();
  const { message } = App.useApp();
  const containerRef = useRef<HTMLDivElement>(null);
  const tableScrollY = useProTableScrollY(containerRef);

  const [syncingAll, setSyncingAll] = useState(false);
  const [accountOptions, setAccountOptions] = useState<{ label: string; value: number }[]>([]);

  useEffect(() => {
    fetchListCloudAccounts(new PaginationQuery({ paging: { page: 1, pageSize: 500 } }))
      .then((r) => setAccountOptions((r.items ?? []).map((a) => ({ label: a.name || String(a.id), value: a.id ?? 0 }))))
      .catch(() => setAccountOptions([]));
  }, []);

  const syncMutation = useSyncCloudAccountInstances();

  const providerOptions = useMemo(
    () => ['ALIYUN', 'TENCENT_CLOUD', 'HUAWEI_CLOUD', 'AWS'].map((p) => ({ label: tAccount(`provider_${p}`), value: p })),
    [tAccount],
  );

  const providerTag = (provider?: string) => {
    const color: Record<string, string> = {
      ALIYUN: 'orange',
      TENCENT_CLOUD: 'blue',
      HUAWEI_CLOUD: 'red',
      AWS: 'gold',
    };
    return <Tag color={color[provider || ''] || 'default'}>{tAccount(`provider_${provider || 'UNKNOWN'}`)}</Tag>;
  };

  const statusTag = (status?: string) => {
    const s = (status || '').toLowerCase();
    const color = s.includes('run') ? 'success' : s.includes('stop') || s.includes('shut') ? 'error' : 'default';
    return <Tag color={color}>{status || '-'}</Tag>;
  };

  const columns: ProColumns<CloudInstance>[] = [
    {
      title: t('cloudAccount'),
      dataIndex: 'cloudAccountId',
      width: 150,
      valueType: 'select',
      fieldProps: { showSearch: true, options: accountOptions, allowClear: true },
      render: (_, r) => <span style={{ fontWeight: 600 }}>{r.cloudAccountName || r.cloudAccountId || '-'}</span>,
    },
    {
      title: tAccount('provider'),
      dataIndex: 'provider',
      width: 110,
      valueType: 'select',
      fieldProps: { options: providerOptions },
      render: (_, r) => providerTag(r.provider),
    },
    { title: t('region'), dataIndex: 'region', width: 130 },
    { title: t('zone'), dataIndex: 'zone', width: 130, hideInSearch: true, render: (v) => v || '-' },
    {
      title: t('instanceId'),
      dataIndex: 'keyword',
      width: 180,
      copyable: true,
      renderText: (_, r) => r.instanceId || '-',
      render: (_, r) => (r.instanceId ? <Typography.Text copyable={{ text: r.instanceId }}>{r.instanceId}</Typography.Text> : '-'),
      fieldProps: { placeholder: t('keywordPlaceholder') },
    },
    { title: t('instanceName'), dataIndex: 'instanceName', width: 160, hideInSearch: true, ellipsis: true, render: (v) => v || '-' },
    { title: t('statusCol'), dataIndex: 'statusCol', width: 110, hideInSearch: true, render: (_, r) => statusTag(r.status) },
    { title: t('instanceType'), dataIndex: 'instanceType', width: 150, hideInSearch: true, render: (v) => v || '-' },
    { title: t('privateIp'), dataIndex: 'privateIp', width: 140, hideInSearch: true, render: (v) => (v ? <Typography.Text code>{String(v)}</Typography.Text> : '-') },
    { title: t('publicIp'), dataIndex: 'publicIp', width: 140, hideInSearch: true, render: (v) => (v ? <Typography.Text code>{String(v)}</Typography.Text> : '-') },
    { title: t('expiredAt'), dataIndex: 'expiredAt', width: 165, hideInSearch: true, render: fmtTime },
    {
      title: t('matchedNode'),
      dataIndex: 'matched',
      width: 150,
      valueType: 'select',
      fieldProps: {
        options: [
          { label: t('matchedYes'), value: 'true' },
          { label: t('matchedNo'), value: 'false' },
        ],
      },
      render: (_, r) =>
        r.matched ? (
          <Tag color="success">{r.agentNodeName || r.agentNodeId}</Tag>
        ) : (
          <Tag>{t('matchedNo')}</Tag>
        ),
    },
    { title: t('syncedAt'), dataIndex: 'syncedAt', width: 165, hideInSearch: true, render: fmtTime },
  ];

  const syncAll = async () => {
    setSyncingAll(true);
    try {
      const accounts = await fetchListCloudAccounts(new PaginationQuery({ paging: { page: 1, pageSize: 500 } }));
      const items = accounts.items || [];
      if (!items.length) {
        message.warning(t('noAccounts'));
        return;
      }
      let total = 0;
      let matched = 0;
      const errors: string[] = [];
      for (const acc of items) {
        if (!acc.id) continue;
        try {
          const resp = await syncMutation.mutateAsync({ cloudAccountId: acc.id });
          total += resp.total || 0;
          matched += resp.matched || 0;
          if (resp.errors?.length) errors.push(...resp.errors);
        } catch (e) {
          errors.push(`${acc.name || acc.id}: ${(e as Error).message}`);
        }
      }
      if (errors.length) {
        message.warning(`${t('syncPartial')}${total} / ${t('matchedCol')} ${matched};${errors.slice(0, 2).join(' | ')}`);
      } else {
        message.success(`${t('syncDone')}: ${total} (${t('matchedCol')} ${matched})`);
      }
      actionRef.current?.reload();
    } finally {
      setSyncingAll(false);
    }
  };

  return (
    <ContentContainer heightMode="fixed" padding="16px" bottomMargin={0}>
      <div ref={containerRef} className="page-container-content">
        <ListTable<CloudInstance>
          actionRef={actionRef}
          columns={columns}
          headerTitle={t('moduleName')}
          request={async (params) => {
            const page = params.current || 1;
            const pageSize = params.pageSize || 10;
            const resp = await fetchListCloudInstances({
              page,
              pageSize,
              cloudAccountId: params.cloudAccountId || undefined,
              provider: params.provider || undefined,
              region: params.region || undefined,
              matched: params.matched === 'true' ? true : params.matched === 'false' ? false : undefined,
              keyword: params.keyword || undefined,
            });
            return { data: resp.items || [], total: resp.total || 0, success: true };
          }}
          rowKey="id"
          search={{ labelWidth: 'auto', defaultColsNumber: 3 }}
          pagination={{ defaultPageSize: TABLE.DEFAULT_PAGE_SIZE, showSizeChanger: true }}
          toolBarRender={() => [
            <Button key="sync" icon={<CloudDownloadOutlined />} loading={syncingAll} onClick={syncAll}>
              {t('syncAll')}
            </Button>,
            <Button key="reload" icon={<SyncOutlined />} onClick={() => actionRef.current?.reload()}>
              {t('refresh')}
            </Button>,
          ]}
          size="middle"
          bordered
          scroll={{ y: tableScrollY, x: 1600 }}
        />
      </div>
    </ContentContainer>
  );
};

function fmtTime(v: any): string {
  if (!v) return '-';
  return String(v).replace('T', ' ').slice(0, 19);
}

export default CloudResourceManagement;
