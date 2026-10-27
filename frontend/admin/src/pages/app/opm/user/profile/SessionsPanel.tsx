import { Alert, Button, List, Typography, App } from 'antd';
import { useTranslation } from 'react-i18next';
import { useQueryClient } from '@tanstack/react-query';
import {
  useGetAccessTokens,
  useRevokeTokenById,
} from '@/api/hooks/auth';

/**
 * 我的会话面板:列出当前用户的 access token(jti),支持远程吊销。
 * 后端仅返回裸 token id(无设备元数据),v1 以短 id 呈现。
 */
const SessionsPanel: React.FC = () => {
  const { t } = useTranslation('profile');
  const { message } = App.useApp();
  const queryClient = useQueryClient();

  const { data, isLoading } = useGetAccessTokens();
  const tokens = data?.accessTokens ?? [];

  const revokeMutation = useRevokeTokenById({
    onSuccess: () => {
      message.success(t('sessionRevoked'));
      queryClient.invalidateQueries({ queryKey: ['myAccessTokens'] });
    },
    onError: (e: Error) => message.error(e.message || t('sessionRevokeFailed')),
  });

  return (
    <div style={{ maxWidth: 560 }}>
      <Typography.Text strong>{t('sessionsTitle')}</Typography.Text>
      <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
        {t('sessionsIntro')}
      </Typography.Paragraph>
      {tokens.length === 0 && !isLoading ? (
        <Alert type="info" showIcon message={t('sessionsEmpty')} />
      ) : (
        <List
          size="small"
          loading={isLoading}
          dataSource={tokens}
          renderItem={(jti) => (
            <List.Item
              actions={[
                <Button
                  key="revoke"
                  size="small"
                  danger
                  onClick={() => revokeMutation.mutate({ jti, reason: 'user revoke' })}
                >
                  {t('sessionRevoke')}
                </Button>,
              ]}
            >
              <Typography.Text code style={{ fontSize: 12 }}>
                {jti.length > 24 ? `${jti.slice(0, 24)}…` : jti}
              </Typography.Text>
            </List.Item>
          )}
        />
      )}
    </div>
  );
};

export default SessionsPanel;
