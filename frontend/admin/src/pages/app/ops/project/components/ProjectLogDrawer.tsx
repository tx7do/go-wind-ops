import { useEffect, useRef, useState } from 'react';
import { Button, Drawer, Popconfirm, Spin, Tag, App } from 'antd';
import { ClearOutlined, ReloadOutlined } from '@ant-design/icons';
import { useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_Project as Project } from '@/api/generated/admin/service/v1';
import { useClearProjectLog, useGetProjectLog } from '@/api/hooks/project';

interface ProjectLogDrawerProps {
  data?: Project;
  onClose: () => void;
}

const LOG_VIEW_LIMIT = 64 * 1024;

/**
 * 项目运行日志抽屉:尾部查看 + 刷新 + 清空。
 */
const ProjectLogDrawer: React.FC<ProjectLogDrawerProps> = ({ data, onClose }) => {
  const { t } = useTranslation('project');
  const queryClient = useQueryClient();
  const { message } = App.useApp();
  const preRef = useRef<HTMLPreElement>(null);

  const [autoScroll, setAutoScroll] = useState(true);

  const query = useGetProjectLog(
    { id: data?.id, maxBytes: LOG_VIEW_LIMIT },
    {
      enabled: !!data?.id,
      refetchInterval: 5000, // 5s 轮询跟进新输出
    },
  );

  const clearMutation = useClearProjectLog({
    onSuccess: () => {
      message.success(t('logClearSuccess'));
      queryClient.invalidateQueries({ queryKey: ['getProjectLog'] });
    },
    onError: (error: Error) => message.error(error.message || t('logClearFailed')),
  });

  useEffect(() => {
    if (autoScroll && preRef.current) {
      preRef.current.scrollTop = preRef.current.scrollHeight;
    }
  }, [query.data?.content, autoScroll]);

  const content = query.data?.content ?? '';
  const size = query.data?.size ?? 0;

  return (
    <Drawer
      title={
        <span>
          {t('logTitle')}
          {data?.name && <Tag style={{ marginLeft: 8 }}>{data.name}</Tag>}
          <Tag>{t('logSize', { size })}</Tag>
        </span>
      }
      open={!!data}
      onClose={onClose}
      size={820}
      destroyOnClose
      extra={
        <span>
          <Button
            size="small"
            icon={<ReloadOutlined />}
            style={{ marginRight: 8 }}
            onClick={() => query.refetch()}
            loading={query.isFetching}
          >
            {t('logRefresh')}
          </Button>
          <Popconfirm
            title={t('logClearConfirmTitle')}
            onConfirm={() => data?.id && clearMutation.mutate({ id: data.id })}
            okText={t('common:button.ok')}
            cancelText={t('common:button.cancel')}
          >
            <Button size="small" danger icon={<ClearOutlined />} loading={clearMutation.isPending}>
              {t('logClear')}
            </Button>
          </Popconfirm>
        </span>
      }
    >
      <div style={{ marginBottom: 8 }}>
        <label style={{ fontSize: 12, color: 'var(--ant-color-text-secondary)' }}>
          <input
            type="checkbox"
            checked={autoScroll}
            onChange={(e) => setAutoScroll(e.target.checked)}
            style={{ marginRight: 4 }}
          />
          {t('logAutoScroll')}
        </label>
      </div>
      {query.isFetching && !query.data ? (
        <Spin style={{ display: 'block', margin: '40px auto' }} />
      ) : (
        <pre
          ref={preRef}
          style={{
            background: 'var(--ant-color-bg-layout, #f6f6f6)',
            border: '1px solid var(--ant-color-border)',
            borderRadius: 8,
            padding: 12,
            maxHeight: 'calc(100vh - 220px)',
            overflow: 'auto',
            fontSize: 12,
            lineHeight: 1.6,
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-all',
            margin: 0,
          }}
        >
          {content || t('logEmpty')}
        </pre>
      )}
    </Drawer>
  );
};

export default ProjectLogDrawer;
