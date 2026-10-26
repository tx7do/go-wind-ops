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
import type { opsservicev1_ApiKey as ApiKey, opsservicev1_ApiKey_Scope as ApiKeyScope } from '@/api/generated/admin/service/v1';
import { useCreateApiKey, useUpdateApiKey } from '@/api/hooks/api-key';

/** 作用域选项值(与生成客户端 ApiKey_Scope 字面量一致) */
const SCOPE_ALL = 'ALL';
const SCOPE_OPTIONS = ['ALL', 'BUILDS', 'DISTRIBUTIONS', 'PIPELINES', 'WEBHOOKS'] as const satisfies readonly ApiKeyScope[];

interface ApiKeyDrawerProps {
  open: boolean;
  mode: 'create' | 'edit';
  data?: ApiKey;
  onClose: () => void;
  /** create 成功时回传仅展示一次的明文 key */
  onSuccess?: (plainKey?: string) => void;
}

/**
 * API 密钥创建/编辑抽屉:创建名称+描述+作用域,编辑状态+描述+作用域。
 * 作用域多选与 ALL 通配互斥;存量密钥无 scopes(全部权限)在编辑时显示为 ALL。
 */
const ApiKeyDrawer: React.FC<ApiKeyDrawerProps> = ({ open, mode, data, onClose, onSuccess }) => {
  const { t } = useTranslation('api-key');
  const formRef = useRef<ProFormInstance>(null);
  const { message } = App.useApp();

  const scopeLabel = (v: string) =>
    v === SCOPE_ALL ? t('scopeAll') : t(`scope${v.charAt(0)}${v.slice(1).toLowerCase()}`);

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

  const handleSubmit = async (values: {
    name?: string;
    description?: string;
    status?: boolean;
    scopes?: ApiKeyScope[];
  }) => {
    // 选中即按所选限定(ALL 或具体项);未选 = 不限定(全部)
    const scopes: ApiKeyScope[] | undefined = values.scopes?.length ? values.scopes : undefined;
    if (mode === 'create') {
      await createMutation.mutateAsync({
        data: { name: values.name, description: values.description || undefined, scopes },
      });
    } else if (data?.id) {
      await updateMutation.mutateAsync({
        id: data.id,
        data: {
          status: values.status === false ? 'DISABLED' : 'ENABLED',
          description: values.description || undefined,
          scopes,
        },
      });
    }
  };

  // 编辑态回显:存量密钥无 scopes(全部)显示为 ALL
  const initialScopes =
    mode === 'edit' ? (data?.scopes?.length ? data.scopes : [SCOPE_ALL]) : [SCOPE_ALL];

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
          <ProFormSelect
            name="scopes"
            label={t('scopes')}
            extra={t('scopesHint')}
            mode="multiple"
            initialValue={initialScopes}
            options={SCOPE_OPTIONS.map((v) => ({ label: scopeLabel(v), value: v }))}
            fieldProps={{
              onChange: (vals: string[]) => {
                // ALL 与具体作用域互斥:后选谁留谁
                const cur = vals ?? [];
                if (cur.length > 1 && cur.includes(SCOPE_ALL)) {
                  const prev = (formRef.current?.getFieldValue('scopes') as string[]) ?? [];
                  const justAdded = cur.find((v) => !prev.includes(v));
                  formRef.current?.setFieldValue(
                    'scopes',
                    justAdded === SCOPE_ALL ? [SCOPE_ALL] : cur.filter((v) => v !== SCOPE_ALL),
                  );
                }
              },
            }}
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

export default ApiKeyDrawer;
