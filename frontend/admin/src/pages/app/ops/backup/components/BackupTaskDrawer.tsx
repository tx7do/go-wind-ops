import { useEffect, useRef, useState } from 'react';
import type { ProFormInstance } from '@ant-design/pro-components';
import { DrawerForm, ProFormDigit, ProFormSelect, ProFormSwitch, ProFormText, ProFormTextArea } from '@ant-design/pro-components';
import { App } from 'antd';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_BackupTask as BackupTask } from '@/api/generated/admin/service/v1';
import { fetchListAgentNodes } from '@/api/hooks/agent-node';
import { useCreateBackupTask, useUpdateBackupTask } from '@/api/hooks/backup';
import { PaginationQuery } from '@/core';

interface Props {
  open: boolean;
  mode: 'create' | 'edit';
  data?: BackupTask;
  onClose: () => void;
  onSuccess: () => void;
}

const BackupTaskDrawer: React.FC<Props> = ({ open, mode, data, onClose, onSuccess }) => {
  const { t } = useTranslation('backup');
  const formRef = useRef<ProFormInstance>(null);
  const { message } = App.useApp();

  const createMutation = useCreateBackupTask({
    onSuccess: () => {
      message.success(t('createSuccess'));
      onSuccess();
      onClose();
    },
    onError: (e: Error) => message.error(e.message || t('createFailed')),
  });
  const updateMutation = useUpdateBackupTask({
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
          nodeId: values.nodeId ? Number(values.nodeId) : 0,
          retention: values.retention ?? 0,
        };
        if (mode === 'create') await createMutation.mutateAsync(payload);
        else if (data?.id) await updateMutation.mutateAsync({ id: data.id, values: payload });
      }}
      params={{ data }}
    >
      {open && (
        <>
          <ProFormText name="name" label={t('name')} rules={[{ required: true }]} initialValue={data?.name} />
          <NodeSelect initialValue={data?.nodeId} />
          <ProFormText
            name="sourceDir"
            label={t('sourceDir')}
            rules={[{ required: true }]}
            placeholder="D:/data/app"
            extra={t('sourceDirHint')}
            initialValue={data?.sourceDir}
          />
          <ProFormText
            name="cron"
            label={t('cron')}
            placeholder="0 2 * * *"
            extra={t('cronHint')}
            initialValue={data?.cron}
          />
          <ProFormDigit
            name="retention"
            label={t('retention')}
            min={0}
            fieldProps={{ precision: 0 }}
            extra={t('retentionHint')}
            initialValue={data?.retention ?? 0}
          />
          <ProFormSwitch name="enabled" label={t('enabled')} initialValue={data?.enabled ?? true} />
          <ProFormTextArea name="description" label={t('description')} fieldProps={{ rows: 2 }} initialValue={data?.description} />
        </>
      )}
    </DrawerForm>
  );
};

// NodeSelect 节点下拉(展示全部节点供选择目标机器)
const NodeSelect: React.FC<{ initialValue?: number }> = ({ initialValue }) => {
  const { t } = useTranslation('backup');
  const [options, setOptions] = useState<{ label: string; value: number }[]>([]);

  useEffect(() => {
    fetchListAgentNodes(new PaginationQuery({ paging: { page: 1, pageSize: 100 } }))
      .then((r: any) =>
        setOptions(
          (r.items ?? []).map((n: any) => ({
            label: n.name || n.hostname || n.agentId || String(n.id),
            value: n.id ?? 0,
          })),
        ),
      )
      .catch(() => setOptions([]));
  }, []);

  return (
    <ProFormSelect
      name="nodeId"
      label={t('nodeId')}
      options={options}
      rules={[{ required: true }]}
      initialValue={initialValue}
    />
  );
};

export default BackupTaskDrawer;
