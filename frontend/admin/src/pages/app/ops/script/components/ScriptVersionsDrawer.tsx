import { useState } from 'react';
import { Drawer, Button, Modal, Space, Tag, Typography, App } from 'antd';
import { useTranslation } from 'react-i18next';
import { useQueryClient } from '@tanstack/react-query';
import {
  useListScriptVersions,
  useRestoreScriptVersion,
} from '@/api/hooks/script';

/**
 * 脚本版本历史抽屉:新→旧列表 + 内容查看 + 一键恢复
 * (恢复前服务端自动快照当前内容,历史链条永不丢)。
 */
const ScriptVersionsDrawer: React.FC<{
  scriptId?: number;
  scriptName?: string;
  onClose: () => void;
}> = ({ scriptId, scriptName, onClose }) => {
  const { t } = useTranslation('script');
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const { data, isLoading } = useListScriptVersions(scriptId);
  const [viewing, setViewing] = useState<{ id: number; content: string } | undefined>();

  const restoreMutation = useRestoreScriptVersion({
    onSuccess: () => {
      message.success(t('versionRestored'));
      queryClient.invalidateQueries({ queryKey: ['scriptVersions', scriptId] });
      queryClient.invalidateQueries({ queryKey: ['listScripts'] });
    },
    onError: (e: Error) => message.error(e.message || t('versionRestoreFailed')),
  });

  return (
    <Drawer
      title={t('versionsTitle', { name: scriptName })}
      open={!!scriptId}
      onClose={onClose}
      width={640}
      destroyOnHidden
    >
      <Space direction="vertical" style={{ width: '100%' }} size="small">
        {(data?.items ?? []).map((v) => (
          <div
            key={v.id}
            style={{ border: '1px solid var(--ant-color-split)', borderRadius: 8, padding: '8px 12px' }}
          >
            <Space style={{ width: '100%', justifyContent: 'space-between' }}>
              <Space size={8}>
                <Typography.Text strong>#{v.id}</Typography.Text>
                <Tag color={v.remark === 'restore' ? 'purple' : 'default'}>
                  {v.remark === 'restore' ? t('versionFromRestore') : t('versionFromAuto')}
                </Tag>
                <Typography.Text type="secondary">
                  {(v.createdAt || '').replace('T', ' ').slice(0, 19)}
                </Typography.Text>
              </Space>
              <Space size={4}>
                <Button size="small" onClick={() => setViewing({ id: v.id || 0, content: v.content || '' })}>
                  {t('versionView')}
                </Button>
                <Button
                  size="small"
                  type="primary"
                  onClick={() => scriptId && v.id && restoreMutation.mutate({ scriptId, versionId: v.id })}
                >
                  {t('versionRestore')}
                </Button>
              </Space>
            </Space>
          </div>
        ))}
        {!isLoading && (data?.items ?? []).length === 0 && (
          <Typography.Text type="secondary">{t('versionsEmpty')}</Typography.Text>
        )}
      </Space>

      <Modal
        title={t('versionViewTitle', { id: viewing?.id ?? '' })}
        open={!!viewing}
        onCancel={() => setViewing(undefined)}
        footer={<Button onClick={() => setViewing(undefined)}>{t('common:button.ok')}</Button>}
        width={760}
      >
        <pre
          style={{
            maxHeight: 460,
            overflow: 'auto',
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-all',
            fontSize: 12,
            background: 'var(--ant-color-bg-layout)',
            padding: 12,
            borderRadius: 8,
          }}
        >
          {viewing?.content}
        </pre>
      </Modal>
    </Drawer>
  );
};

export default ScriptVersionsDrawer;
