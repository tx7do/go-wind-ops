import { useEffect, useMemo, useRef, useState } from 'react';
import ReactECharts from 'echarts-for-react';
import type { ProColumns, ActionType } from '@ant-design/pro-components';
import ListTable from '@/components/common/ListTable';
import { App, Button, Input, Modal, Popconfirm, Select, Space, Tag, Typography } from 'antd';
import {
  DeleteOutlined,
  DownloadOutlined,
  PlusOutlined,
  UploadOutlined,
} from '@ant-design/icons';
import { useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';
import type { opsservicev1_CloudCost as CloudCost } from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { TABLE } from '@/config/constants';
import { fetchListCloudAccounts } from '@/api/hooks/cloud-account';
import {
  fetchListCloudCosts,
  useCloudCostSummary,
  useDeleteCloudCost,
  useFetchCloudCostFromProvider,
  useImportCloudCosts,
} from '@/api/hooks/cloud-cost';
import { useProTableScrollY } from '@/hooks/useProTableScrollY';
import ContentContainer from '@/layouts/components/PageContainer/ContentContainer';
import CloudCostDrawer from './components/CloudCostDrawer';

/** 近 N 个年份选项(图表与列表过滤用)。 */
const yearOptions = () => {
  const y = dayjs().year();
  return [0, 1, 2, 3].map((i) => ({ label: String(y - i), value: String(y - i) }));
};

const CloudCostManagement = () => {
  const { t } = useTranslation('cloud-cost');
  const { t: tAccount } = useTranslation('cloud-account');
  const actionRef = useRef<ActionType>(null);
  const queryClient = useQueryClient();
  const { message } = App.useApp();
  const containerRef = useRef<HTMLDivElement>(null);
  const tableScrollY = useProTableScrollY(containerRef);

  const [chartYear, setChartYear] = useState<string>(dayjs().format('YYYY'));
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editing, setEditing] = useState<CloudCost | undefined>();
  const [importOpen, setImportOpen] = useState(false);
  const [csvText, setCsvText] = useState('');
  const [accountOptions, setAccountOptions] = useState<{ label: string; value: number }[]>([]);
  const [fetchOpen, setFetchOpen] = useState(false);
  const [fetchAccountId, setFetchAccountId] = useState<number>();
  const [fetchMonth, setFetchMonth] = useState<string>(dayjs().subtract(1, 'month').format('YYYY-MM'));

  useEffect(() => {
    fetchListCloudAccounts(new PaginationQuery({ paging: { page: 1, pageSize: 500 } }))
      .then((r) => setAccountOptions((r.items ?? []).map((a) => ({ label: a.name || String(a.id), value: a.id ?? 0 }))))
      .catch(() => setAccountOptions([]));
  }, []);

  const summary = useCloudCostSummary({ year: chartYear });

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['listCloudCosts'] });
    queryClient.invalidateQueries({ queryKey: ['cloudCostSummary'] });
  };

  const deleteMutation = useDeleteCloudCost({
    onSuccess: () => {
      message.success(t('deleteSuccess'));
      invalidate();
      actionRef.current?.reload();
    },
    onError: (e: Error) => message.error(e.message || t('deleteFailed')),
  });
  const importMutation = useImportCloudCosts({
    onSuccess: (resp) => {
      setImportOpen(false);
      setCsvText('');
      if (resp.errors?.length) {
        message.warning(`${t('importPartial')}${resp.imported || 0}${t('importUnits')}${resp.errors.slice(0, 2).join(' | ')}`);
      } else {
        message.success(`${t('importDone')}: ${resp.imported || 0}`);
      }
      invalidate();
      actionRef.current?.reload();
    },
    onError: (e: Error) => message.error(e.message || t('importFailed')),
  });
  const fetchMutation = useFetchCloudCostFromProvider({
    onSuccess: (resp) => {
      message.success(`${t('fetchDone')}: ${resp.data?.cloudAccountName || ''} ${resp.data?.month} = ${resp.data?.amount ?? 0} ${resp.data?.currency || 'CNY'}`);
      setFetchOpen(false);
      invalidate();
      actionRef.current?.reload();
    },
    onError: (e: Error) => message.error(e.message || t('fetchFailed')),
  });

  // CSV 文本 → 结构化行(格式: 月份,账号名,金额[,币种][,备注];首行为表头自动跳过)
  const parseCsv = (text: string) => {
    const rows: { cloudAccountName?: string; month: string; amount: number; currency?: string; remark?: string }[] = [];
    const errors: string[] = [];
    text
      .split(/\r?\n/)
      .map((l) => l.trim())
      .filter(Boolean)
      .forEach((line, idx) => {
        if (idx === 0 && /^month/i.test(line)) return;
        const parts = line.split(',');
        if (parts.length < 3) {
          errors.push(String(idx + 1));
          return;
        }
        const month = parts[0].trim();
        const amount = Number(parts[2].trim());
        if (!/^\d{4}-(0[1-9]|1[0-2])$/.test(month) || Number.isNaN(amount)) {
          errors.push(String(idx + 1));
          return;
        }
        rows.push({
          cloudAccountName: parts[1].trim(),
          month,
          amount,
          currency: parts[3]?.trim() || 'CNY',
          remark: parts.slice(4).join(',').trim() || undefined,
        });
      });
    return { rows, errors };
  };

  const doImport = () => {
    const { rows, errors } = parseCsv(csvText);
    if (!rows.length) {
      message.warning(`${t('csvNoRows')}${errors.length ? ` (${t('csvBadLines')}: ${errors.join(',')})` : ''}`);
      return;
    }
    importMutation.mutate({ rows });
  };

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

  // 趋势图:12 个月 × 厂商;同币种堆叠、不同币种并列(不换汇,不混加)
  const chartOption = useMemo(() => {
    const items = summary.data?.items || [];
    const months = Array.from({ length: 12 }, (_, i) => `${chartYear}-${String(i + 1).padStart(2, '0')}`);
    const seriesKeys = Array.from(new Set(items.map((i) => `${i.provider}|${i.currency}`)));
    const byKey: Record<string, number[]> = {};
    seriesKeys.forEach((k) => {
      byKey[k] = months.map(() => 0);
    });
    items.forEach((it) => {
      const k = `${it.provider}|${it.currency}`;
      const idx = months.indexOf(it.month);
      if (idx >= 0) byKey[k][idx] += it.total || 0;
    });
    return {
      tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
      legend: { type: 'scroll', top: 0 },
      grid: { left: 56, right: 16, top: 36, bottom: 28 },
      xAxis: { type: 'category', data: months },
      yAxis: { type: 'value' },
      series: seriesKeys.map((k) => {
        const [provider, currency] = k.split('|');
        return {
          name: `${tAccount(`provider_${provider}`)} (${currency})`,
          type: 'bar',
          stack: currency,
          data: byKey[k],
          barMaxWidth: 26,
        };
      }),
    };
  }, [summary.data, chartYear, tAccount]);

  const columns: ProColumns<CloudCost>[] = [
    {
      title: t('month'),
      dataIndex: 'month',
      width: 120,
      hideInSearch: true,
      render: (_, r) => <span style={{ fontWeight: 600 }}>{r.month}</span>,
    },
    {
      title: t('year'),
      dataIndex: 'year',
      width: 100,
      valueType: 'select',
      fieldProps: { options: yearOptions(), allowClear: true },
      hideInTable: true,
    },
    {
      title: t('cloudAccount'),
      dataIndex: 'cloudAccountId',
      width: 160,
      valueType: 'select',
      fieldProps: { showSearch: true, options: accountOptions, allowClear: true },
      render: (_, r) => <span>{r.cloudAccountName || r.cloudAccountId}</span>,
    },
    {
      title: tAccount('provider'),
      dataIndex: 'provider',
      width: 110,
      valueType: 'select',
      fieldProps: { options: providerOptions },
      render: (_, r) => providerTag(r.provider),
    },
    {
      title: t('amount'),
      dataIndex: 'amount',
      width: 130,
      hideInSearch: true,
      render: (_, r) => (
        <Typography.Text strong>
          {(r.amount ?? 0).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
        </Typography.Text>
      ),
    },
    { title: t('currency'), dataIndex: 'currency', width: 90, hideInSearch: true, render: (v) => <Tag>{String(v || 'CNY')}</Tag> },
    { title: t('remark'), dataIndex: 'remark', ellipsis: true, hideInSearch: true, render: (v) => v || '-' },
    { title: t('updatedAt'), dataIndex: 'updatedAt', width: 165, hideInSearch: true, render: fmtTime },
    {
      title: t('action'),
      valueType: 'option',
      width: 130,
      fixed: 'right',
      render: (_, record) => [
        <a key="edit" onClick={() => { setEditing(record); setDrawerOpen(true); }}>
          {t('edit')}
        </a>,
        <Popconfirm
          key="delete"
          title={t('deleteConfirmTitle')}
          onConfirm={() => record.id && deleteMutation.mutate({ id: record.id })}
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
    <ContentContainer heightMode="fixed" padding="16px" bottomMargin={0}>
      <div
        ref={containerRef}
        className="page-container-content page-container-content--block-above"
        style={{ overflow: 'auto' }}
      >
        {/* 趋势图:当年 12 个月按厂商堆叠(多币种并列);flex:none 固定自然高度,表格占剩余高度 */}
        <div style={{ flex: 'none' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
            <Typography.Text strong>{t('trendTitle')}</Typography.Text>
            <Button.Group size="small">
              {yearOptions().map((y) => (
                <Button key={y.value} type={chartYear === y.value ? 'primary' : 'default'} onClick={() => setChartYear(y.value)}>
                  {y.label}
                </Button>
              ))}
            </Button.Group>
          </div>
          <ReactECharts option={chartOption} style={{ height: 260 }} notMerge lazyUpdate />
        </div>

        <ListTable<CloudCost>
          actionRef={actionRef}
          columns={columns}
          headerTitle={t('moduleName')}
          request={async (params) => {
            const resp = await fetchListCloudCosts({
              page: params.current || 1,
              pageSize: params.pageSize || 10,
              cloudAccountId: params.cloudAccountId || undefined,
              year: params.year || undefined,
              provider: params.provider || undefined,
            });
            return { data: resp.items || [], total: resp.total || 0, success: true };
          }}
          rowKey="id"
          search={{ labelWidth: 'auto', defaultColsNumber: 3 }}
          pagination={{ defaultPageSize: TABLE.DEFAULT_PAGE_SIZE, showSizeChanger: true }}
          toolBarRender={() => [
            <Button key="fetch" icon={<DownloadOutlined />} onClick={() => {
              setFetchMonth(dayjs().subtract(1, 'month').format('YYYY-MM'));
              setFetchOpen(true);
            }}>
              {t('fetch')}
            </Button>,
            <Button key="import" icon={<UploadOutlined />} onClick={() => setImportOpen(true)}>
              {t('import')}
            </Button>,
            <Button
              key="create"
              type="primary"
              icon={<PlusOutlined />}
              onClick={() => {
                setEditing(undefined);
                setDrawerOpen(true);
              }}
            >
              {t('entry')}
            </Button>,
          ]}
          size="middle"
          bordered
          scroll={{ y: tableScrollY, x: 1100 }}
        />
      </div>

      <CloudCostDrawer
        open={drawerOpen}
        data={editing}
        accountOptions={accountOptions}
        onClose={() => {
          setDrawerOpen(false);
          setEditing(undefined);
        }}
        onSuccess={invalidate}
      />

      {/* 从云端拉取:选账号+月份,调厂商账单 API 后直接进台账 */}
      <Modal
        title={t('fetch')}
        open={fetchOpen}
        onCancel={() => setFetchOpen(false)}
        onOk={() => {
          if (!fetchAccountId) {
            message.warning(t('accountRequired'));
            return;
          }
          if (!/^\d{4}-(0[1-9]|1[0-2])$/.test(fetchMonth)) {
            message.warning(t('monthPattern'));
            return;
          }
          fetchMutation.mutate({ cloudAccountId: fetchAccountId, month: fetchMonth });
        }}
        confirmLoading={fetchMutation.isPending}
        width={480}
        destroyOnClose
      >
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Select
            style={{ width: '100%' }}
            placeholder={t('cloudAccount')}
            showSearch
            optionFilterProp="label"
            value={fetchAccountId}
            onChange={setFetchAccountId}
            options={accountOptions}
          />
          <Input
            value={fetchMonth}
            onChange={(e) => setFetchMonth(e.target.value)}
            placeholder="2026-09"
          />
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {t('fetchHint')}
          </Typography.Text>
        </Space>
      </Modal>

      {/* CSV 导入:格式 月份,账号名,金额[,币种][,备注];按账号名匹配 */}
      <Modal
        title={t('import')}
        open={importOpen}
        onCancel={() => setImportOpen(false)}
        onOk={doImport}
        confirmLoading={importMutation.isPending}
        width={640}
        destroyOnClose
      >
        <Space direction="vertical" style={{ width: '100%' }} size="small">
          <Typography.Text type="secondary">{t('csvFormat')}</Typography.Text>
          <Typography.Text code style={{ whiteSpace: 'pre-wrap' }}>
            {`2026-09,主账号,1234.56
2026-09,海外账号,200,USD
2026-10,主账号,987.65,CNY,预算内`}
          </Typography.Text>
          <Typography.Text type="secondary">{t('csvNote')}</Typography.Text>
          <Input.TextArea
            rows={8}
            value={csvText}
            onChange={(e) => setCsvText(e.target.value)}
            placeholder={t('csvPlaceholder')}
          />
        </Space>
      </Modal>
    </ContentContainer>
  );
};

function fmtTime(v: any): string {
  if (!v) return '-';
  return String(v).replace('T', ' ').slice(0, 19);
}

export default CloudCostManagement;
