import { useRef, useState } from 'react';
import type { ProColumns, ActionType } from '@ant-design/pro-components';
import ListTable from '@/components/common/ListTable';
import { Button, Input, Modal, Popconfirm, Select, Tag, Tooltip, App, Space } from 'antd';
import {
  DeleteOutlined,
  EyeOutlined,
  SearchOutlined,
  SyncOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type {
  opsservicev1_LogSnapshot as LogSnapshot,
} from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { TABLE } from '@/config/constants';
import {
  fetchListLogSnapshots,
  useCaptureLogSnapshot,
  useDeleteLogSnapshot,
  useSearchLogSnapshots,
} from '@/api/hooks/log-snapshot';
import { fetchListProjects } from '@/api/hooks';
import { useProTableScrollY } from '@/hooks/useProTableScrollY';
import ContentContainer from '@/layouts/components/PageContainer/ContentContainer';

/**
 * 日志快照页:采集项目日志尾部 + 跨项目关键字检索
 */
const LogSnapshotManagement = () => {
  const { t } = useTranslation('log-snapshot');
  const actionRef = useRef<ActionType>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const tableScrollY = useProTableScrollY(containerRef);
  const { message, modal } = App.useApp();

  const [captureOpen, setCaptureOpen] = useState(false);
  const [captureProjectId, setCaptureProjectId] = useState<number | undefined>();
  const [projectOptions, setProjectOptions] = useState<{ label: string; value: number }[]>([]);
  const [keyword, setKeyword] = useState('');
  const [hits, setHits] = useState<{ snapshotId?: number; projectName?: string; excerpt?: string; matchCount?: number; capturedAt?: string }[]>([]);
  const [searching, setSearching] = useState(false);

  const captureMutation = useCaptureLogSnapshot({
    onSuccess: (resp) => {
      message.success(t('captureSuccess', { name: resp.projectName }));
      setCaptureOpen(false);
      actionRef.current?.reload();
    },
    onError: (e: Error) => message.error(e.message || t('captureFailed')),
  });

  const deleteMutation = useDeleteLogSnapshot({
    onSuccess: () => {
      message.success(t('deleteSuccess'));
      actionRef.current?.reload();
    },
    onError: (e: Error) => message.error(e.message || t('deleteFailed')),
  });

  const searchMutation = useSearchLogSnapshots({
    onSuccess: (resp) => {
      setHits((resp.hits || []).map((h) => ({
        snapshotId: h.snapshotId,
        projectName: h.projectName,
        excerpt: h.excerpt,
        matchCount: h.matchCount,
        capturedAt: h.capturedAt,
      })));
      setSearching(false);
    },
    onError: (e: Error) => {
      message.error(e.message || t('searchFailed'));
      setSearching(false);
    },
  });

  const openCapture = async () => {
    setCaptureOpen(true);
    try {
      const r = await fetchListProjects(new PaginationQuery({ paging: { page: 1, pageSize: 100 } }));
      setProjectOptions((r.items ?? []).map((p: any) => ({ label: p.name || String(p.id), value: p.id ?? 0 })));
    } catch {
      setProjectOptions([]);
    }
  };

  const doSearch = () => {
    if (!keyword.trim()) {
      message.warning(t('keywordRequired'));
      return;
    }
    setSearching(true);
    searchMutation.mutate({ keyword: keyword.trim() });
  };

  const showContent = (row: LogSnapshot) => {
    modal.info({
      title: t('contentTitle', { name: row.projectName }),
      width: 860,
      content: (
        <pre
          style={{
            maxHeight: 480,
            overflow: 'auto',
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-all',
            fontSize: 12,
            background: 'var(--ant-color-bg-layout)',
            padding: 12,
            borderRadius: 8,
          }}
        >
          {row.content || t('noContent')}
        </pre>
      ),
      okText: t('common:button.ok'),
    });
  };

  const columns: ProColumns<LogSnapshot>[] = [
    { title: t('projectName'), dataIndex: 'projectName', width: 160 },
    { title: t('nodeId'), dataIndex: 'nodeId', width: 90, hideInSearch: true },
    {
      title: t('size'),
      dataIndex: 'sizeBytes',
      width: 110,
      hideInSearch: true,
      render: (_, r) => `${Math.ceil((r.sizeBytes ?? 0) / 1024)} KB`,
    },
    {
      title: t('trigger'),
      dataIndex: 'trigger',
      width: 100,
      hideInSearch: true,
      render: (_, r) => (r.trigger === 'SCHEDULED' ? <Tag color="blue">{t('triggerScheduled')}</Tag> : <Tag>{t('triggerManual')}</Tag>),
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
      width: 120,
      fixed: 'right',
      render: (_, r) => [
        <Tooltip key="view" title={t('view')}>
          <a onClick={() => showContent(r)}>
            <EyeOutlined />
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
          <Space.Compact style={{ marginBottom: 12, width: '100%' }}>
            <Input
              placeholder={t('searchPlaceholder')}
              value={keyword}
              onChange={(e) => setKeyword(e.target.value)}
              onPressEnter={doSearch}
              style={{ maxWidth: 420 }}
              prefix={<SearchOutlined />}
            />
            <Button type="primary" loading={searching} onClick={doSearch}>
              {t('search')}
            </Button>
          </Space.Compact>
          {hits.length > 0 && (
            <div style={{ marginBottom: 12 }}>
              {hits.map((h, i) => (
                <div
                  key={h.snapshotId || i}
                  style={{
                    border: '1px solid var(--ant-color-border)',
                    borderRadius: 8,
                    padding: '8px 12px',
                    marginBottom: 8,
                    fontSize: 12,
                  }}
                >
                  <div style={{ fontWeight: 600, marginBottom: 4 }}>
                    {h.projectName} · {h.matchCount} {t('hits')} · {h.capturedAt?.replace('T', ' ').slice(0, 19)}
                  </div>
                  <pre style={{ margin: 0, whiteSpace: 'pre-wrap', wordBreak: 'break-all', maxHeight: 120, overflow: 'auto' }}>
                    {h.excerpt}
                  </pre>
                </div>
              ))}
            </div>
          )}
          <ListTable<LogSnapshot>
            actionRef={actionRef}
            columns={columns}
            rowKey="id"
            request={async (params) => {
              const q = new PaginationQuery({
                paging: { page: params.current || 1, pageSize: params.pageSize || 10 },
                orderBy: ['-createdAt'],
              });
              const resp = await fetchListLogSnapshots(q);
              return { data: resp.items || [], total: resp.total || 0, success: true };
            }}
            search={false}
            pagination={{ defaultPageSize: TABLE.DEFAULT_PAGE_SIZE, showSizeChanger: true }}
            toolBarRender={() => [
              <Button key="capture" type="primary" icon={<SyncOutlined />} onClick={openCapture}>
                {t('capture')}
              </Button>,
            ]}
            options={{ density: true, fullScreen: true, setting: true, reload: true }}
            size="middle"
            bordered
            scroll={{ y: tableScrollY, x: 800 }}
          />
        </div>
      </ContentContainer>

      <Modal
        title={t('capture')}
        open={captureOpen}
        onCancel={() => setCaptureOpen(false)}
        onOk={() => {
          if (!captureProjectId) {
            message.warning(t('projectRequired'));
            return;
          }
          captureMutation.mutate({ projectId: captureProjectId });
        }}
        confirmLoading={captureMutation.isPending}
        width={420}
        destroyOnHidden
      >
        <div style={{ marginBottom: 8 }}>{t('projectLabel')}</div>
        <Select
          style={{ width: '100%' }}
          placeholder={t('projectPlaceholder')}
          value={captureProjectId}
          onChange={setCaptureProjectId}
          options={projectOptions}
          showSearch
          optionFilterProp="label"
        />
        <div style={{ marginTop: 8, fontSize: 12, color: 'var(--ant-color-text-secondary)' }}>
          {t('captureHint')}
        </div>
      </Modal>
    </>
  );
};

export default LogSnapshotManagement;
