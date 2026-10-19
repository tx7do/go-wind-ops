import { useRef } from 'react';
import type { ProFormInstance } from '@ant-design/pro-components';
import {
  DrawerForm,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { App } from 'antd';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_Repository as Repository } from '@/api/generated/admin/service/v1';
import { useCreateRepository, useUpdateRepository } from '@/api/hooks/repository';

interface RepositoryDrawerProps {
  open: boolean;
  mode: 'create' | 'edit';
  data?: Repository;
  onClose: () => void;
  onSuccess: () => void;
}

/**
 * 仓库编辑/创建抽屉
 */
const RepositoryDrawer: React.FC<RepositoryDrawerProps> = ({
  open,
  mode,
  data,
  onClose,
  onSuccess,
}) => {
  const { t } = useTranslation('repository');
  const formRef = useRef<ProFormInstance>(null);
  const { message } = App.useApp();

  const createMutation = useCreateRepository({
    onSuccess: () => {
      message.success(t('createSuccess'));
      onSuccess();
      onClose();
    },
    onError: (error: Error) => message.error(error.message || t('createFailed')),
  });

  const updateMutation = useUpdateRepository({
    onSuccess: () => {
      message.success(t('updateSuccess'));
      onSuccess();
      onClose();
    },
    onError: (error: Error) => message.error(error.message || t('updateFailed')),
  });

  const handleSubmit = async (values: any) => {
    if (mode === 'create') {
      await createMutation.mutateAsync(values);
    } else if (data?.id) {
      // 密码为掩码/空时不更新(后端处理),其余字段全量提交
      await updateMutation.mutateAsync({ id: data.id, values });
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
        width: 560,
      }}
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
          <ProFormText
            name="gitUrl"
            label={t('gitUrl')}
            rules={[{ required: true, message: t('gitUrlRequired') }]}
            placeholder="https://github.com/user/repo.git"
            extra={t('gitUrlHint')}
            initialValue={data?.gitUrl}
          />
          <ProFormText
            name="branch"
            label={t('branch')}
            placeholder="main"
            initialValue={data?.branch}
          />
          <ProFormText
            name="username"
            label={t('username')}
            initialValue={data?.username}
          />
          <ProFormText.Password
            name="password"
            label={t('password')}
            placeholder={mode === 'edit' ? t('passwordEditPlaceholder') : t('passwordPlaceholder')}
            initialValue={mode === 'create' ? undefined : '******'}
          />
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

export default RepositoryDrawer;
