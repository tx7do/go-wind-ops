import { useRef } from 'react';
import type { ProFormInstance } from '@ant-design/pro-components';
import { DrawerForm, ProFormSwitch, ProFormText, ProFormTextArea } from '@ant-design/pro-components';
import { App } from 'antd';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_ApiKey as ApiKey } from '@/api/generated/admin/service/v1';
import { useCreateApiKey, useUpdateApiKey } from '@/api/hooks/api-key';

interface ApiKeyDrawerProps {
  open: boolean;
  mode: 'create' | 'edit';
  data?: ApiKey;
  onClose: () => void;
  /** create 成功时回传仅展示一次的明文 key */
  onSuccess?: (plainKey?: string) => void;
}

/**
 * API 密钥创建/编辑抽屉:创建仅需名称+描述,编辑仅状态+描述。
 */
const ApiKeyDrawer: React.FC<ApiKeyDrawerProps> = ({ open, mode, data, onClose, onSuccess }) => {
  const { t } = useTranslation('api-key');
  const formRef = useRef<ProFormInstance>(null);
  const { message } = App.useApp();

  const createMutation = useCreateApiKey({
    onSuccess: (resp) => {
      onSuccess?.(resp.plainKey || '');
      onClose();
    },
    onError: (e: Error) => message.error(e.message || t('createFailed')),
  });
  const updateMutation = useUpdateApiKey({
    onSuccess: () => {
      message.success(t('updateSuccess'));
      onSuccess?.();
      onClose();
    },
    onError: (e: Error) => message.error(e.message || t('updateFailed')),
  });

  const handleSubmit = async (values: any) => {
    if (mode === 'create') {
      await createMutation.mutateAsync({
        data: { name: values.name, description: values.description || undefined },
      });
    } else if (data?.id) {
      await updateMutation.mutateAsync({
        id: data.id,
        data: {
          status: values.status === false ? 'DISABLED' : 'ENABLED',
          description: values.description || undefined,
        },
      });
    }
  };

  return (
    <DrawerForm
      title={mode === 'create' ? t('create') : t('edit')}
      formRef={formRef}
      open={open}
      drawerProps={{ destroyOnHidden: true, onClose, size: 480 }}
      onFinish={handleSubmit}
      params={{ data, mode }}
    >
      {open && (
        <>
          {mode === 'create' && (
            <ProFormText
              name="name"
              label={t('name')}
              rules={[{ required: true, message: t('nameRequired') }]}
            />
          )}
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

export default ApiKeyDrawer;
