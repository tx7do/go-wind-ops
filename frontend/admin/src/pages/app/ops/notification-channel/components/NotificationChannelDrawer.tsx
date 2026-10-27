import { useRef } from 'react';
import type { ProFormInstance } from '@ant-design/pro-components';
import { DrawerForm, ProFormDigit, ProFormSelect, ProFormText, ProFormTextArea } from '@ant-design/pro-components';
import { App } from 'antd';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_NotificationChannel as Channel } from '@/api/generated/admin/service/v1';
import { useCreateNotificationChannel, useUpdateNotificationChannel } from '@/api/hooks/notification-channel';

interface Props { open: boolean; mode: 'create' | 'edit'; data?: Channel; onClose: () => void; onSuccess: () => void; }

const NotificationChannelDrawer: React.FC<Props> = ({ open, mode, data, onClose, onSuccess }) => {
  const { t } = useTranslation('notification-channel');
  const formRef = useRef<ProFormInstance>(null);
  const { message } = App.useApp();
  const createMutation = useCreateNotificationChannel({
    onSuccess: () => { message.success(t('createSuccess')); onSuccess(); onClose(); },
    onError: (e: Error) => message.error(e.message || t('createFailed')),
  });
  const updateMutation = useUpdateNotificationChannel({
    onSuccess: () => { message.success(t('updateSuccess')); onSuccess(); onClose(); },
    onError: (e: Error) => message.error(e.message || t('updateFailed')),
  });



  return (
    <DrawerForm title={mode === 'create' ? t('create') : t('edit')} formRef={formRef} open={open}
      drawerProps={{ onClose, destroyOnHidden: true, width: 520 }}
      onFinish={async (values: any) => {
        if (mode === 'create') await createMutation.mutateAsync(values);
        else if (data?.id) await updateMutation.mutateAsync({ id: data.id, values });
      }}
      params={{ data }}
    >
      {open && (<>
        <ProFormText name="name" label={t('name')} rules={[{ required: true }]} initialValue={data?.name} />
        <ProFormSelect name="type" label={t('type')} rules={[{ required: true }]}
          options={[
            { label: t('typeWebhook'), value: 'WEBHOOK' },
            { label: t('typeEmail'), value: 'EMAIL' },
          ]}
          initialValue={data?.type ?? 'WEBHOOK'}
        />
        <ProFormText name="webhookUrl" label={t('webhookUrl')} placeholder="https://..." initialValue={data?.webhookUrl} />
        <ProFormText name="webhookSecret" label={t('webhookSecret')} placeholder={t('webhookSecretPlaceholder')} initialValue={data?.webhookSecret} />
        <ProFormSelect name="webhookSignStyle" label={t('webhookSignStyle')}
          options={[
            { label: t('signStyleOptions.none'), value: 'NONE' },
            { label: t('signStyleOptions.dingtalk'), value: 'DINGTALK' },
            { label: t('signStyleOptions.feishu'), value: 'FEISHU' },
            { label: t('signStyleOptions.wecom'), value: 'WECOM' },
          ]}
          initialValue={data?.webhookSignStyle ?? 'NONE'}
        />
        <ProFormText name="smtpHost" label={t('smtpHost')} initialValue={data?.smtpHost} />
        <ProFormDigit name="smtpPort" label={t('smtpPort')} min={1} max={65535} initialValue={data?.smtpPort ?? 587} />
        <ProFormText name="smtpUsername" label={t('smtpUsername')} initialValue={data?.smtpUsername} />
        <ProFormText.Password name="smtpPassword" label={t('smtpPassword')} initialValue={data?.smtpPassword} />
        <ProFormText name="smtpFrom" label={t('smtpFrom')} initialValue={data?.smtpFrom} />
        <ProFormTextArea name="description" label={t('description')} fieldProps={{ rows: 2 }} initialValue={data?.description} />
        <ProFormSelect name="enabled" label={t('enabled')} options={[{ label: t('on'), value: true }, { label: t('off'), value: false }]} initialValue={data?.enabled ?? true} />
      </>)}
    </DrawerForm>
  );
};
export default NotificationChannelDrawer;
