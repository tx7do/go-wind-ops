import type { ProFormInstance } from '@ant-design/pro-components';
import {
  DrawerForm,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { App } from 'antd';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_Project as Project } from '@/api/generated/admin/service/v1';
import { fetchListAgentNodes, useCreateProject, useUpdateProject } from '@/api/hooks';
import { PaginationQuery } from '@/core';
import { ENV_PRESETS } from '@/config/constants';

interface ProjectDrawerProps {
  open: boolean;
  mode: 'create' | 'edit';
  data?: Project;
  onClose: () => void;
  onSuccess: () => void;
}

/**
 * 项目编辑/创建抽屉
 */
const ProjectDrawer: React.FC<ProjectDrawerProps> = ({
  open,
  mode,
  data,
  onClose,
  onSuccess,
}) => {
  const { t } = useTranslation('project');
  const formRef = useRef<ProFormInstance>(null);
  const { message } = App.useApp();

  const [confirmLoading, setConfirmLoading] = useState(false);

  // 节点下拉(在线优先展示由后端排序;这里取全量)
  const [nodeOptions, setNodeOptions] = useState<{ label: string; value: number }[]>([]);
  useEffect(() => {
    if (!open) return;
    fetchListAgentNodes(new PaginationQuery({ paging: { page: 1, pageSize: 100 } }))
      .then((resp) => {
        setNodeOptions(
          (resp.items ?? []).map((n) => ({
            label: `${n.name || n.hostname || n.agentId}${n.status === 'ONLINE' ? '（在线）' : '（离线）'}`,
            value: n.id ?? 0,
          })),
        );
      })
      .catch(() => setNodeOptions([]));
  }, [open]);

  const createMutation = useCreateProject({
    onSuccess: () => {
      message.success(t('createSuccess'));
      onSuccess();
      onClose();
    },
    onError: (error: Error) => message.error(error.message || t('createFailed')),
  });

  const updateMutation = useUpdateProject({
    onSuccess: () => {
      message.success(t('updateSuccess'));
      onSuccess();
      onClose();
    },
    onError: (error: Error) => message.error(error.message || t('updateFailed')),
  });

  // 机器指纹/运行状态字段由 agent 链路维护,仅生命周期配置可编辑
  const EDITABLE_KEYS = ['name', 'nodeId', 'projectDir', 'startCommand', 'stopCommand', 'statusCommand', 'description'];

  const handleSubmit = async (values: any) => {
    setConfirmLoading(true);
    try {
      const payload: any = { ...values };
      if (payload.nodeId === undefined || payload.nodeId === null) {
        delete payload.nodeId;
      }
      if (mode === 'create') {
        await createMutation.mutateAsync(payload);
      } else if (data?.id) {
        const filtered: Record<string, any> = {};
        for (const key of EDITABLE_KEYS) {
          if (payload[key] !== undefined) filtered[key] = payload[key];
        }
        await updateMutation.mutateAsync({ id: data.id, values: filtered });
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
        destroyOnClose: true,
        width: 560,
      }}
      submitter={{ submitButtonProps: { loading: confirmLoading } }}
      onFinish={handleSubmit}
      params={{ data }}
    >
      {open && (
        <>
          <ProFormSelect
            name="nodeId"
            label={t('nodeId')}
            options={nodeOptions}
            placeholder={t('nodeIdPlaceholder')}
            rules={[{ required: true, message: t('nodeIdRequired') }]}
            fieldProps={{
              onChange: () => formRef.current?.validateFields(['name']),
            }}
            initialValue={data?.nodeId}
          />
          <ProFormSelect
            name="env"
            label={t('env')}
            options={ENV_PRESETS.map((v) => ({ label: v, value: v }))}
            initialValue={data?.env ?? 'default'}
            extra={t('envHint')}
          />
          <ProFormText
            name="name"
            label={t('name')}
            rules={[{ required: true, message: t('nameRequired') }]}
            initialValue={data?.name}
          />
          <ProFormText
            name="projectDir"
            label={t('projectDir')}
            placeholder="/opt/apps/my-app 或 D:/apps/my-app"
            rules={[{ required: true, message: t('projectDirRequired') }]}
            extra={t('projectDirHint')}
            initialValue={data?.projectDir}
          />
          <ProFormTextArea
            name="startCommand"
            label={t('startCommand')}
            placeholder="e.g. ./bin/app --port 8080"
            fieldProps={{ rows: 2, allowClear: true }}
            initialValue={data?.startCommand}
          />
          <ProFormTextArea
            name="stopCommand"
            label={t('stopCommand')}
            placeholder={t('stopCommandPlaceholder')}
            fieldProps={{ rows: 1, allowClear: true }}
            initialValue={data?.stopCommand}
          />
          <ProFormTextArea
            name="statusCommand"
            label={t('statusCommand')}
            placeholder={t('statusCommandPlaceholder')}
            fieldProps={{ rows: 1, allowClear: true }}
            initialValue={data?.statusCommand}
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

export default ProjectDrawer;
