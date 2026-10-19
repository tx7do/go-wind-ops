import { useRef } from 'react';
import type { ProFormInstance } from '@ant-design/pro-components';
import { DrawerForm, ProFormText } from '@ant-design/pro-components';
import { App } from 'antd';
import { useTranslation } from 'react-i18next';
import { useEditUserPassword } from '@/api/hooks/user';
import { queryClient } from '@/core';

interface EditPasswordDrawerProps {
  open: boolean;
  userId: number | undefined;
  onClose: () => void;
}

/**
 * 管理员修改用户密码抽屉
 */
const EditPasswordDrawer: React.FC<EditPasswordDrawerProps> = ({ open, userId, onClose }) => {
  const { t } = useTranslation('user-detail');
  const formRef = useRef<ProFormInstance>(null);
  const { message } = App.useApp();

  const editPasswordMutation = useEditUserPassword({
    onSuccess: () => {
      message.success(t('editPasswordSuccess'));
      onClose();
      queryClient.invalidateQueries({ queryKey: ['getUser'] });
    },
    onError: (error: Error) => {
      message.error(error.message || t('editPasswordFailed'));
    },
  });

  const handleSubmit = async (values: any) => {
    if (values.newPassword !== values.confirmPassword) {
      message.error(t('passwordMismatch'));
      return;
    }
    await editPasswordMutation.mutateAsync({
      userId: userId!,
      newPassword: values.newPassword,
    });
  };

  return (
    <DrawerForm
      title={t('editPasswordTitle')}
      formRef={formRef}
      open={open}
      drawerProps={{ destroyOnHidden: true, onClose, size: 480 }}
      onFinish={handleSubmit}
    >
      {open && (
        <>
          <ProFormText.Password
            name="newPassword"
            label={t('newPassword')}
            rules={[{ required: true, message: t('newPasswordPlaceholder') }]}
            fieldProps={{ placeholder: t('newPasswordPlaceholder'), autoComplete: 'new-password' }}
          />
          <ProFormText.Password
            name="confirmPassword"
            label={t('confirmPassword')}
            rules={[{ required: true, message: t('confirmPasswordPlaceholder') }]}
            fieldProps={{ placeholder: t('confirmPasswordPlaceholder'), autoComplete: 'new-password' }}
          />
        </>
      )}
    </DrawerForm>
  );
};

export default EditPasswordDrawer;
