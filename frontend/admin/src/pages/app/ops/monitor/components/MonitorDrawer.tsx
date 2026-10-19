import { useEffect, useRef, useState } from 'react';
import type { ProFormInstance } from '@ant-design/pro-components';
import { DrawerForm, ProFormDigit, ProFormSelect, ProFormSwitch, ProFormText, ProFormTextArea } from '@ant-design/pro-components';
import { App } from 'antd';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_Monitor as Monitor } from '@/api/generated/admin/service/v1';
import { fetchListProjects, useCreateMonitor, useUpdateMonitor } from '@/api/hooks';
import { fetchListAgentNodes } from '@/api/hooks/agent-node';
import { PaginationQuery } from '@/core';

interface Props { open: boolean; mode: 'create' | 'edit'; data?: Monitor; onClose: () => void; onSuccess: () => void; }

const MonitorDrawer: React.FC<Props> = ({ open, mode, data, onClose, onSuccess }) => {
  const { t } = useTranslation('monitor');
  const formRef = useRef<ProFormInstance>(null);
  const { message } = App.useApp();
  const [projectOptions, setProjectOptions] = useState<{ label: string; value: number }[]>([]);
  const [nodeOptions, setNodeOptions] = useState<{ label: string; value: number }[]>([]);
  const [targetType, setTargetType] = useState<string>('PROJECT');

  useEffect(() => {
    if (!open) return;
    setTargetType(data?.targetType || 'PROJECT');
    fetchListProjects(new PaginationQuery({ paging: { page: 1, pageSize: 100 } }))
      .then((r: any) => setProjectOptions((r.items ?? []).map((p: any) => ({ label: p.name || String(p.id), value: p.id ?? 0 }))))
      .catch(() => setProjectOptions([]));
    fetchListAgentNodes(new PaginationQuery({ paging: { page: 1, pageSize: 100 } }))
      .then((r: any) => setNodeOptions((r.items ?? []).map((n: any) => ({ label: n.name || n.hostname || n.agentId || String(n.id), value: n.id ?? 0 }))))
      .catch(() => setNodeOptions([]));
  }, [open, data]);

  const createMutation = useCreateMonitor({
    onSuccess: () => { message.success(t('createSuccess')); onSuccess(); onClose(); },
    onError: (e: Error) => message.error(e.message || t('createFailed')),
  });
  const updateMutation = useUpdateMonitor({
    onSuccess: () => { message.success(t('updateSuccess')); onSuccess(); onClose(); },
    onError: (e: Error) => message.error(e.message || t('updateFailed')),
  });

  const isNode = targetType === 'NODE_METRIC';
  const isProcess = targetType === 'PROCESS';
  const isProject = !isNode && !isProcess;
  const isNodeOrProcess = isNode || isProcess;

  return (
    <DrawerForm title={mode === 'create' ? t('create') : t('edit')} formRef={formRef} open={open}
      drawerProps={{ onClose, destroyOnHidden: true, size: 560 }}
      onFinish={async (values: any) => {
        const payload = {
          ...values,
          targetType: values.targetType || 'PROJECT',
          nodeId: isNodeOrProcess ? values.nodeId : undefined,
          metricType: isNode ? (values.metricType || 'CPU') : undefined,
          thresholdPercent: isNode ? values.thresholdPercent : undefined,
          processName: isProcess ? values.processName : undefined,
          consecutiveCount: isNodeOrProcess ? (values.consecutiveCount ?? 1) : undefined,
          notifyOnRecovery: isNodeOrProcess ? (values.notifyOnRecovery ?? true) : undefined,
        };
        if (mode === 'create') await createMutation.mutateAsync(payload);
        else if (data?.id) await updateMutation.mutateAsync({ id: data.id, values: payload });
      }}
      params={{ data }}
    >
      {open && (<>
        <ProFormText name="name" label={t('name')} rules={[{ required: true }]} initialValue={data?.name} />
        <ProFormSelect
          name="targetType"
          label={t('targetType')}
          options={[
            { label: t('targetProject'), value: 'PROJECT' },
            { label: t('targetNodeMetric'), value: 'NODE_METRIC' },
            { label: t('targetProcess'), value: 'PROCESS' },
          ]}
          initialValue={data?.targetType || 'PROJECT'}
          fieldProps={{ onChange: (v: string) => setTargetType(v) }}
        />
        {isNodeOrProcess && (
          <ProFormSelect name="nodeId" label={t('nodeId')} options={nodeOptions} rules={[{ required: true }]} initialValue={data?.nodeId} />
        )}
        {isProject && (
          <ProFormSelect name="projectId" label={t('projectId')} options={projectOptions} rules={[{ required: true }]} initialValue={data?.projectId} />
        )}
        {isNode && (
          <ProFormSelect
            name="metricType"
            label={t('metricType')}
            options={[
              { label: t('metricCpu'), value: 'CPU' },
              { label: t('metricMemory'), value: 'MEMORY' },
              { label: t('metricDisk'), value: 'DISK' },
            ]}
            initialValue={data?.metricType || 'CPU'}
            rules={[{ required: true }]}
          />
        )}
        {isNode && (
          <ProFormDigit
            name="thresholdPercent"
            label={t('thresholdPercent')}
            min={1}
            max={100}
            fieldProps={{ precision: 1, addonAfter: '%' }}
            rules={[{ required: true }]}
            extra={t('thresholdHint')}
            initialValue={data?.thresholdPercent ?? 80}
          />
        )}
        {isProcess && (
          <ProFormText
            name="processName"
            label={t('processName')}
            rules={[{ required: true }]}
            placeholder="nginx / mysqld / java"
            extra={t('processNameHint')}
            initialValue={data?.processName}
          />
        )}
        {isNodeOrProcess && (
          <ProFormDigit
            name="consecutiveCount"
            label={t('consecutiveCount')}
            min={1}
            max={60}
            fieldProps={{ precision: 0 }}
            extra={t('consecutiveHint')}
            initialValue={data?.consecutiveCount ?? 1}
          />
        )}
        {isNodeOrProcess && (
          <ProFormSwitch name="notifyOnRecovery" label={t('notifyOnRecovery')} initialValue={data?.notifyOnRecovery ?? true} />
        )}
        <ProFormText name="cron" label={t('cron')} rules={[{ required: true }]} placeholder="*/5 * * * *" extra={t('cronHint')} initialValue={data?.cron} />
        <ProFormDigit name="repeatIntervalMinutes" label={t('repeatInterval')} min={0} fieldProps={{ precision: 0 }} extra={t('repeatIntervalHint')} initialValue={data?.repeatIntervalMinutes ?? 0} addonAfter="min" />
        {isProject && <ProFormSwitch name="autoRestart" label={t('autoRestart')} initialValue={data?.autoRestart ?? false} />}
        <ProFormSwitch name="enabled" label={t('enabled')} initialValue={data?.enabled ?? true} />
        <ProFormText name="webhookUrl" label={t('webhookUrl')} placeholder="https://..." initialValue={data?.webhookUrl} />
        <ProFormTextArea name="description" label={t('description')} fieldProps={{ rows: 2 }} initialValue={data?.description} />
      </>)}
    </DrawerForm>
  );
};
export default MonitorDrawer;
