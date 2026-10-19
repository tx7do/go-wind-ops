import { useRef } from 'react';
import type { ProFormInstance } from '@ant-design/pro-components';
import { DrawerForm, ProFormDigit, ProFormSwitch, ProFormText, ProFormTextArea } from '@ant-design/pro-components';
import { App } from 'antd';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_Cert as Cert } from '@/api/generated/admin/service/v1';
import { useCreateCert, useUpdateCert } from '@/api/hooks/cert';

interface Props {
  open: boolean;
  mode: 'create' | 'edit';
  data?: Cert;
  onClose: () => void;
  onSuccess: () => void;
}

const CertDrawer: React.FC<Props> = ({ open, mode, data, onClose, onSuccess }) => {
  const { t } = useTranslation('cert');
  const formRef = useRef<ProFormInstance>(null);
  const { message } = App.useApp();

  const createMutation = useCreateCert({
    onSuccess: () => {
      message.success(t('createSuccess'));
      onSuccess();
      onClose();
    },
    onError: (e: Error) => message.error(e.message || t('createFailed')),
  });
  const updateMutation = useUpdateCert({
    onSuccess: () => {
      message.success(t('updateSuccess'));
      onSuccess();
      onClose();
    },
    onError: (e: Error) => message.error(e.message || t('updateFailed')),
  });

  return (
    <DrawerForm
      title={mode === 'create' ? t('create') : t('edit')}
      formRef={formRef}
      open={open}
      drawerProps={{ onClose, destroyOnHidden: true, size: 560 }}
      onFinish={async (values: any) => {
        const payload = {
          ...values,
          port: values.port ?? 443,
          daysBeforeExpire: values.daysBeforeExpire ?? 30,
        };
        if (mode === 'create') await createMutation.mutateAsync(payload);
        else if (data?.id) await updateMutation.mutateAsync({ id: data.id, values: payload });
      }}
      params={{ data }}
    >
      {open && (
        <>
          <ProFormText name="name" label={t('name')} rules={[{ required: true }]} initialValue={data?.name} />
          <ProFormText
            name="host"
            label={t('host')}
            rules={[{ required: true }]}
            placeholder="example.com"
            extra={t('hostHint')}
            initialValue={data?.host}
          />
          <ProFormDigit
            name="port"
            label={t('port')}
            min={1}
            max={65535}
            fieldProps={{ precision: 0 }}
            initialValue={data?.port ?? 443}
          />
          <ProFormDigit
            name="daysBeforeExpire"
            label={t('daysBeforeExpire')}
            min={1}
            max={365}
            fieldProps={{ precision: 0 }}
            extra={t('daysBeforeHint')}
            initialValue={data?.daysBeforeExpire ?? 30}
          />
          <ProFormText
            name="probeCron"
            label={t('probeCron')}
            placeholder="0 9 * * *"
            extra={t('probeCronHint')}
            initialValue={data?.probeCron}
          />
          <ProFormSwitch name="enabled" label={t('enabled')} initialValue={data?.enabled ?? true} />
          <ProFormTextArea name="description" label={t('description')} fieldProps={{ rows: 2 }} initialValue={data?.description} />
        </>
      )}
    </DrawerForm>
  );
};
export default CertDrawer;
