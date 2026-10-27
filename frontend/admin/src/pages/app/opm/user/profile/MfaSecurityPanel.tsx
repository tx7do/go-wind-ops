import { useState } from 'react';
import { Alert, Button, Input, QRCode, Space, Tag, Typography, App } from 'antd';
import { SafetyCertificateOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useQueryClient } from '@tanstack/react-query';
import type { identityservicev1_User as User } from '@/api/generated/admin/service/v1';
import { useGetUserProfile } from '@/api/hooks/user-profile';
import {
  useMfaSetupStart,
  useMfaSetupConfirm,
  useMfaDisable,
  useMfaBackupCodesRegenerate,
} from '@/api/hooks/auth';
import { useAuthStore } from '@/stores';

/**
 * MFA(TOTP) 二步验证面板:未启用→扫码/手输密钥+验证码确认绑定;
 * 已启用→验证码解绑。绑定与解绑都要求当前有效验证码,防劫持操作。
 */
const MfaSecurityPanel: React.FC = () => {
  const { t } = useTranslation('profile');
  const { message } = App.useApp();
  const queryClient = useQueryClient();
  const setUserInfo = useAuthStore((s) => s.setUserInfo);

  const { data: userData } = useGetUserProfile();
  const user = userData as User | undefined | null;
  const mfaEnabled = !!user?.mfaEnabled;

  const [setup, setSetup] = useState<{ secret: string; otpauthUri: string } | undefined>();
  const [code, setCode] = useState('');
  const [backupCodes, setBackupCodes] = useState<string[]>([]);

  const refreshUser = () => {
    queryClient.invalidateQueries({ queryKey: ['getMe'] });
    queryClient.fetchQuery({ queryKey: ['getMe'] }).then((info) => {
      if (info) setUserInfo(info as unknown as UserInfo);
    });
  };

  const setupMutation = useMfaSetupStart({
    onSuccess: (resp) => {
      setSetup({ secret: resp.secret || '', otpauthUri: resp.otpauthUri || '' });
    },
    onError: (e: Error) => message.error(e.message || t('mfaSetupStartFailed')),
  });

  const confirmMutation = useMfaSetupConfirm({
    onSuccess: () => {
      message.success(t('mfaEnabledSuccess'));
      setSetup(undefined);
      setCode('');
      refreshUser();
    },
    onError: (e: Error) => message.error(e.message || t('mfaCodeInvalid')),
  });

  const backupMutation = useMfaBackupCodesRegenerate({
    onSuccess: (resp) => setBackupCodes(resp.codes ?? []),
    onError: (e: Error) => message.error(e.message || t('mfaSetupStartFailed')),
  });

  const disableMutation = useMfaDisable({
    onSuccess: () => {
      message.success(t('mfaDisabledSuccess'));
      setCode('');
      refreshUser();
    },
    onError: (e: Error) => message.error(e.message || t('mfaCodeInvalid')),
  });

  const validCode = code.trim().length === 6 && /^\d+$/.test(code.trim());

  return (
    <div style={{ maxWidth: 520 }}>
      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        <Space>
          <SafetyCertificateOutlined style={{ fontSize: 20 }} />
          <Typography.Text strong>{t('mfaTitle')}</Typography.Text>
          {mfaEnabled ? (
            <Tag color="success">{t('mfaEnabled')}</Tag>
          ) : (
            <Tag color="warning">{t('mfaNotEnabled')}</Tag>
          )}
        </Space>

        <Typography.Text type="secondary">{t('mfaIntro')}</Typography.Text>

        {!mfaEnabled && !setup && (
          <Button
            type="primary"
            loading={setupMutation.isPending}
            onClick={() => setupMutation.mutate({})}
          >
            {t('mfaSetupStart')}
          </Button>
        )}

        {!mfaEnabled && setup && (
          <>
            <Alert type="info" showIcon message={t('mfaScanHint')} />
            <div style={{ display: 'flex', gap: 24, alignItems: 'center', flexWrap: 'wrap' }}>
              <QRCode value={setup.otpauthUri} size={160} />
              <div>
                <Typography.Paragraph type="secondary" style={{ marginBottom: 4 }}>
                  {t('mfaSecretLabel')}
                </Typography.Paragraph>
                <Typography.Text code copyable style={{ fontSize: 16 }}>
                  {setup.secret}
                </Typography.Text>
              </div>
            </div>
            <Input
              value={code}
              onChange={(e) => setCode(e.target.value)}
              placeholder={t('mfaCodePlaceholder')}
              maxLength={6}
              style={{ width: 200 }}
              inputMode="numeric"
            />
            <Space>
              <Button
                type="primary"
                disabled={!validCode}
                loading={confirmMutation.isPending}
                onClick={() => confirmMutation.mutate({ code: code.trim() })}
              >
                {t('mfaConfirmBind')}
              </Button>
              <Button onClick={() => setSetup(undefined)}>{t('mfaCancelBind')}</Button>
            </Space>
          </>
        )}

        {mfaEnabled && (
          <>
            <div>
              <Button
                size="small"
                loading={backupMutation.isPending}
                onClick={() => backupMutation.mutate()}
                style={{ marginBottom: 8 }}
              >
                {backupCodes.length ? t('mfaBackupRegenerate') : t('mfaBackupGenerate')}
              </Button>
              {backupCodes.length > 0 && (
                <Alert
                  type="warning"
                  showIcon
                  message={t('mfaBackupOnceHint')}
                  description={
                    <Typography.Paragraph
                      code
                      copyable={{ text: backupCodes.join('\n') }}
                      style={{ marginBottom: 0, whiteSpace: 'pre-wrap' }}
                    >
                      {backupCodes.join('  ')}
                    </Typography.Paragraph>
                  }
                />
              )}
            </div>
            <Input
              value={code}
              onChange={(e) => setCode(e.target.value)}
              placeholder={t('mfaCodePlaceholder')}
              maxLength={6}
              style={{ width: 200 }}
              inputMode="numeric"
            />
            <Button
              danger
              disabled={!validCode}
              loading={disableMutation.isPending}
              onClick={() => disableMutation.mutate({ code: code.trim() })}
            >
              {t('mfaDisableBtn')}
            </Button>
          </>
        )}
      </Space>
    </div>
  );
};

export default MfaSecurityPanel;
