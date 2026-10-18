import { useEffect, useRef, useState } from 'react';
import type { ProFormInstance } from '@ant-design/pro-components';
import { DrawerForm, ProFormSelect, ProFormSwitch, ProFormText, ProFormTextArea } from '@ant-design/pro-components';
import { App } from 'antd';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_Monitor as Monitor } from '@/api/generated/admin/service/v1';
import { fetchListProjects, useCreateMonitor, useUpdateMonitor } from '@/api/hooks';
import { PaginationQuery } from '@/core';

interface Props { open: boolean; mode: 'create' | 'edit'; data?: Monitor; onClose: () => void; onSuccess: () => void; }

const MonitorDrawer: React.FC<Props> = ({ open, mode, data, onClose, onSuccess }) => {
  const { t } = useTranslation('monitor');
  const formRef = useRef<ProFormInstance>(null);
  const { message } = App.useApp();
  const [projectOptions, setProjectOptions] = useState<{ label: string; value: number }[]>([]);

  useEffect(() => {
    if (!open) return;
    fetchListProjects(new PaginationQuery({ paging: { page: 1, pageSize: 100 } }))
      .then((r: any) => setProjectOptions((r.items ?? []).map((p: any) => ({ label: p.name || String(p.id), value: p.id ?? 0 }))))
      .catch(() => setProjectOptions([]));
  }, [open]);

  const createMutation = useCreateMonitor({
    onSuccess: () => { message.success(t('createSuccess')); onSuccess(); onClose(); },
    onError: (e: Error) => message.error(e.message || t('createFailed')),
  });
  const updateMutation = useUpdateMonitor({
    onSuccess: () => { message.success(t('updateSuccess')); onSuccess(); onClose(); },
    onError: (e: Error) => message.error(e.message || t('updateFailed')),
  });

  return (
    <DrawerForm title={mode === 'create' ? t('create') : t('edit')} formRef={formRef} open={open}
      drawerProps={{ onClose, destroyOnClose: true, width: 520 }}
      onFinish={async (values: any) => {
        if (mode === 'create') await createMutation.mutateAsync(values);
        else if (data?.id) await updateMutation.mutateAsync({ id: data.id, values });
      }}
      params={{ data }}
    >
      {open && (<>
        <ProFormText name="name" label={t('name')} rules={[{ required: true }]} initialValue={data?.name} />
        <ProFormSelect name="projectId" label={t('projectId')} options={projectOptions} rules={[{ required: true }]} initialValue={data?.projectId} />
        <ProFormText name="cron" label={t('cron')} rules={[{ required: true }]} placeholder="*/5 * * * *" extra={t('cronHint')} initialValue={data?.cron} />
        <ProFormSwitch name="autoRestart" label={t('autoRestart')} initialValue={data?.autoRestart ?? false} />
        <ProFormSwitch name="enabled" label={t('enabled')} initialValue={data?.enabled ?? true} />
        <ProFormText name="webhookUrl" label={t('webhookUrl')} placeholder="https://..." initialValue={data?.webhookUrl} />
        <ProFormTextArea name="description" label={t('description')} fieldProps={{ rows: 2 }} initialValue={data?.description} />
      </>)}
    </DrawerForm>
  );
};
export default MonitorDrawer;
