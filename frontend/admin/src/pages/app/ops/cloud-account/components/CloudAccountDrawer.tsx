import { useRef } from 'react';
import type { ProFormInstance } from '@ant-design/pro-components';
import {
  DrawerForm,
  ProFormSelect,
  ProFormSwitch,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { App } from 'antd';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_CloudAccount as CloudAccount } from '@/api/generated/admin/service/v1';
import {
  useCreateCloudAccount,
  useUpdateCloudAccount,
} from '@/api/hooks/cloud-account';

const PROVIDERS = ['ALIYUN', 'TENCENT_CLOUD', 'HUAWEI_CLOUD', 'AWS'] as const;

interface CloudAccountDrawerProps {
  open: boolean;
  mode: 'create' | 'edit';
  data?: CloudAccount;
  onClose: () => void;
  onSuccess: () => void;
}

/**
 * 云账号创建/编辑抽屉:编辑时 SK 留空表示不变更,明文仅提交时经请求字段传入。
 */
const CloudAccountDrawer: React.FC<CloudAccountDrawerProps> = ({
  open,
  mode,
  data,
  onClose,
  onSuccess,
}) => {
  const { t } = useTranslation('cloud-account');
  const formRef = useRef<ProFormInstance>(null);
  const { message } = App.useApp();

  const createMutation = useCreateCloudAccount({
    onSuccess: () => {
      message.success(t('createSuccess'));
      onSuccess();
      onClose();
    },
    onError: (e: Error) => message.error(e.message || t('createFailed')),
  });
  const updateMutation = useUpdateCloudAccount({
    onSuccess: () => {
      message.success(t('updateSuccess'));
      onSuccess();
      onClose();
    },
    onError: (e: Error) => message.error(e.message || t('updateFailed')),
  });

  const handleSubmit = async (values: any) => {
    const payload = {
      name: values.name,
      provider: values.provider,
      accessKey: values.accessKey,
      region: values.region || undefined,
      description: values.description || undefined,
      status: (values.status === false ? 'DISABLED' : 'ENABLED') as never,
    };
    if (mode === 'create') {
      await createMutation.mutateAsync({ data: payload, secretKey: values.secretKey });
    } else if (data?.id) {
      await updateMutation.mutateAsync({
        id: data.id,
        data: payload,
        secretKey: values.secretKey || undefined,
      });
    }
  };

  return (
    <DrawerForm
      title={mode === 'create' ? t('create') : t('edit')}
      formRef={formRef}
      open={open}
      drawerProps={{ destroyOnHidden: true, onClose, size: 560 }}
      onFinish={handleSubmit}
      params={{ data }}
    >
      {open && (
        <>
          <ProFormText
            name="name"
            label={t('name')}
            rules={[{ required: true, message: t('nameRequired') }]}
            initialValue={data?.name}
          />
          <ProFormSelect
            name="provider"
            label={t('provider')}
            rules={[{ required: true }]}
            initialValue={data?.provider ?? 'ALIYUN'}
            options={PROVIDERS.map((p) => ({ value: p, label: t(`provider_${p}`) }))}
          />
          <ProFormText
            name="accessKey"
            label={t('accessKey')}
            rules={[{ required: true, message: t('accessKeyRequired') }]}
            initialValue={data?.accessKey}
          />
          <ProFormText.Password
            name="secretKey"
            label={mode === 'create' ? t('secretKey') : t('secretKeyKeep')}
            rules={mode === 'create' ? [{ required: true, message: t('secretKeyRequired') }] : undefined}
            fieldProps={{ autoComplete: 'new-password' }}
          />
          <ProFormText name="region" label={t('region')} initialValue={data?.region} />
          {mode === 'edit' && (
            <ProFormSwitch
              name="status"
              label={t('status')}
              checkedChildren={t('statusEnabled')}
              unCheckedChildren={t('statusDisabled')}
              initialValue={data?.status !== 'DISABLED'}
            />
          )}
          <ProFormTextArea
            name="description"
            label={t('description')}
            fieldProps={{ rows: 2, allowClear: true }}
            initialValue={data?.description}
          />
        </>
      )}
    </DrawerForm>
  );
};

export default CloudAccountDrawer;
