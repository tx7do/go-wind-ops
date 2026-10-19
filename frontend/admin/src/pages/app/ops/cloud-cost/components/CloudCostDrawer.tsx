import { useRef } from 'react';
import type { ProFormInstance } from '@ant-design/pro-components';
import {
  DrawerForm,
  ProFormDigit,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { App } from 'antd';
import dayjs from 'dayjs';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_CloudCost as CloudCost } from '@/api/generated/admin/service/v1';
import { useUpsertCloudCost } from '@/api/hooks/cloud-cost';

interface CloudCostDrawerProps {
  open: boolean;
  data?: CloudCost; // 编辑时预填(按账号+月份定位,无 id 概念)
  accountOptions: { label: string; value: number }[];
  onClose: () => void;
  onSuccess: () => void;
}

/**
 * 成本录入抽屉:账号+月份唯一,重复提交即覆盖(UPSERT)。
 */
const CloudCostDrawer: React.FC<CloudCostDrawerProps> = ({
  open,
  data,
  accountOptions,
  onClose,
  onSuccess,
}) => {
  const { t } = useTranslation('cloud-cost');
  const formRef = useRef<ProFormInstance>(null);
  const { message } = App.useApp();

  const upsertMutation = useUpsertCloudCost({
    onSuccess: () => {
      message.success(t('saveSuccess'));
      onSuccess();
      onClose();
    },
    onError: (e: Error) => message.error(e.message || t('saveFailed')),
  });

  return (
    <DrawerForm
      title={t('entry')}
      formRef={formRef}
      open={open}
      drawerProps={{ destroyOnHidden: true, onClose, size: 480 }}
      onFinish={async (values: any) => {
        await upsertMutation.mutateAsync({
          data: {
            cloudAccountId: values.cloudAccountId,
            month: values.month,
            amount: values.amount,
            currency: values.currency || 'CNY',
            remark: values.remark || undefined,
          },
        });
        return true;
      }}
      params={{ data }}
    >
      {open && (
        <>
          <ProFormSelect
            name="cloudAccountId"
            label={t('cloudAccount')}
            rules={[{ required: true, message: t('accountRequired') }]}
            options={accountOptions}
            initialValue={data?.cloudAccountId}
            fieldProps={{ showSearch: true }}
          />
          <ProFormText
            name="month"
            label={t('month')}
            rules={[
              { required: true, message: t('monthRequired') },
              { pattern: /^\d{4}-(0[1-9]|1[0-2])$/, message: t('monthPattern') },
            ]}
            initialValue={data?.month ?? dayjs().format('YYYY-MM')}
            placeholder="2026-10"
            extra={t('monthHint')}
          />
          <ProFormDigit
            name="amount"
            label={t('amount')}
            rules={[{ required: true, message: t('amountRequired') }]}
            min={0}
            fieldProps={{ precision: 2, step: 0.01 }}
          />
          <ProFormSelect
            name="currency"
            label={t('currency')}
            initialValue={data?.currency ?? 'CNY'}
            options={[
              { label: 'CNY', value: 'CNY' },
              { label: 'USD', value: 'USD' },
              { label: 'EUR', value: 'EUR' },
              { label: 'HKD', value: 'HKD' },
            ]}
            extra={t('currencyHint')}
          />
          <ProFormTextArea name="remark" label={t('remark')} fieldProps={{ rows: 2, allowClear: true }} />
        </>
      )}
    </DrawerForm>
  );
};

export default CloudCostDrawer;
