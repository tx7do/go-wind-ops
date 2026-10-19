import { useRef, useState } from 'react';
import type { ProFormInstance } from '@ant-design/pro-components';
import {
  DrawerForm,
  ProFormDigit,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { App } from 'antd';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_Script as Script } from '@/api/generated/admin/service/v1';
import { useCreateScript, useUpdateScript } from '@/api/hooks/script';

interface ScriptDrawerProps {
  open: boolean;
  mode: 'create' | 'edit';
  data?: Script;
  onClose: () => void;
  onSuccess: () => void;
}

/**
 * 脚本编辑/创建抽屉
 */
const ScriptDrawer: React.FC<ScriptDrawerProps> = ({
  open,
  mode,
  data,
  onClose,
  onSuccess,
}) => {
  const { t } = useTranslation('script');
  const formRef = useRef<ProFormInstance>(null);
  const { message } = App.useApp();

  const [confirmLoading, setConfirmLoading] = useState(false);

  const createMutation = useCreateScript({
    onSuccess: () => {
      message.success(t('createSuccess'));
      onSuccess();
      onClose();
    },
    onError: (error: Error) => message.error(error.message || t('createFailed')),
  });

  const updateMutation = useUpdateScript({
    onSuccess: () => {
      message.success(t('updateSuccess'));
      onSuccess();
      onClose();
    },
    onError: (error: Error) => message.error(error.message || t('updateFailed')),
  });

  const handleSubmit = async (values: any) => {
    setConfirmLoading(true);
    try {
      if (mode === 'create') {
        await createMutation.mutateAsync(values);
      } else if (data?.id) {
        await updateMutation.mutateAsync({ id: data.id, values });
      }
    } finally {
      setConfirmLoading(false);
    }
  };

  return (
    <DrawerForm
      title={mode === 'create' ? t('create') : t('edit')}
      formRef={formRef}
      open={open}
      drawerProps={{
        onClose,
        destroyOnHidden: true,
        width: 640,
      }}
      submitter={{ submitButtonProps: { loading: confirmLoading } }}
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
          <ProFormTextArea
            name="description"
            label={t('description')}
            fieldProps={{ rows: 2, allowClear: true }}
            initialValue={data?.description}
          />
          <ProFormTextArea
            name="content"
            label={t('content')}
            rules={[{ required: true, message: t('contentRequired') }]}
            fieldProps={{
              rows: 16,
              allowClear: true,
              style: { fontFamily: 'Consolas, Monaco, monospace', fontSize: 13 },
            }}
            placeholder={t('contentPlaceholder')}
            extra={t('contentLimitHint')}
            initialValue={data?.content}
          />
          <ProFormDigit
            name="timeoutSeconds"
            label={t('timeoutSeconds')}
            min={1}
            max={600}
            fieldProps={{ precision: 0 }}
            initialValue={60}
          />
          <ProFormText
            name="cron"
            label={t('cron')}
            placeholder="*/30 * * * *"
            extra={t('cronHint')}
            initialValue={data?.cron}
          />
          <ProFormSelect
            name="cronNodeId"
            label={t('cronNode')}
            placeholder={t('cronNodePlaceholder')}
            allowClear
            options={[]}
            initialValue={data?.cronNodeId}
          />
        </>
      )}
    </DrawerForm>
  );
};

export default ScriptDrawer;
