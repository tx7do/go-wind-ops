import { useEffect, useRef, useState } from 'react';
import type { ProFormInstance } from '@ant-design/pro-components';
import { DrawerForm, ProFormDigit, ProFormSelect, ProFormSwitch, ProFormText, ProFormTextArea } from '@ant-design/pro-components';
import { App } from 'antd';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_CloudBudget as CloudBudget } from '@/api/generated/admin/service/v1';
import { fetchListCloudAccounts } from '@/api/hooks/cloud-account';
import { useCreateCloudBudget, useUpdateCloudBudget } from '@/api/hooks/cloud-budget';
import { PaginationQuery } from '@/core';

interface Props {
  open: boolean;
  mode: 'create' | 'edit';
  data?: CloudBudget;
  onClose: () => void;
  onSuccess: () => void;
}

const CloudBudgetDrawer: React.FC<Props> = ({ open, mode, data, onClose, onSuccess }) => {
  const { t } = useTranslation('cloud-budget');
  const formRef = useRef<ProFormInstance>(null);
  const { message } = App.useApp();
  const [accountOptions, setAccountOptions] = useState<{ label: string; value: number }[]>([]);

  useEffect(() => {
    if (!open) return;
    fetchListCloudAccounts(new PaginationQuery({ paging: { page: 1, pageSize: 100 } }))
      .then((r: any) =>
        setAccountOptions((r.items ?? []).map((a: any) => ({ label: a.name || String(a.id), value: a.id ?? 0 }))),
      )
      .catch(() => setAccountOptions([]));
  }, [open]);

  const createMutation = useCreateCloudBudget({
    onSuccess: () => {
      message.success(t('createSuccess'));
      onSuccess();
      onClose();
    },
    onError: (e: Error) => message.error(e.message || t('createFailed')),
  });
  const updateMutation = useUpdateCloudBudget({
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
          cloudAccountId: values.cloudAccountId ?? 0,
          currency: values.currency || 'CNY',
          thresholdPercent: values.thresholdPercent ?? 100,
        };
        if (mode === 'create') await createMutation.mutateAsync(payload);
        else if (data?.id) await updateMutation.mutateAsync({ id: data.id, values: payload });
      }}
      params={{ data }}
    >
      {open && (
        <>
          <ProFormSelect
            name="cloudAccountId"
            label={t('account')}
            options={[{ label: t('allAccounts'), value: 0 }, ...accountOptions]}
            extra={t('accountHint')}
            initialValue={data?.cloudAccountId ?? 0}
          />
          <ProFormText
            name="month"
            label={t('month')}
            rules={[{ required: true, pattern: /^\d{4}-\d{2}$/, message: t('monthInvalid') }]}
            placeholder="2026-10"
            extra={t('monthHint')}
            initialValue={data?.month}
          />
          <ProFormDigit
            name="amount"
            label={t('amount')}
            rules={[{ required: true }]}
            min={0.01}
            fieldProps={{ precision: 2 }}
            initialValue={data?.amount}
          />
          <ProFormSelect
            name="currency"
            label={t('currency')}
            options={['CNY', 'USD', 'EUR', 'JPY'].map((c) => ({ label: c, value: c }))}
            initialValue={data?.currency || 'CNY'}
          />
          <ProFormDigit
            name="thresholdPercent"
            label={t('thresholdPercent')}
            min={1}
            max={1000}
            fieldProps={{ precision: 0, addonAfter: '%' }}
            extra={t('thresholdHint')}
            initialValue={data?.thresholdPercent ?? 100}
          />
          <ProFormDigit
            name="anomalyRatio"
            label={t('anomalyRatio')}
            min={0}
            fieldProps={{ precision: 1, addonBefore: '×' }}
            extra={t('anomalyHint')}
            initialValue={data?.anomalyRatio ?? 0}
          />
          <ProFormSwitch name="enabled" label={t('enabled')} initialValue={data?.enabled ?? true} />
          <ProFormTextArea name="description" label={t('description')} fieldProps={{ rows: 2 }} initialValue={data?.description} />
        </>
      )}
    </DrawerForm>
  );
};
export default CloudBudgetDrawer;
