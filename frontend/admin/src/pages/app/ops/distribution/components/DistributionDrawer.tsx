import { useEffect, useRef, useState } from 'react';
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
import type {
  opsservicev1_Build as Build,
  opsservicev1_Distribution as Distribution,
  opsservicev1_Project as Project,
} from '@/api/generated/admin/service/v1';
import { fetchListBuilds, fetchListProjects } from '@/api/hooks';
import { PaginationQuery } from '@/core';
import { useCreateDistribution, useUpdateDistribution } from '@/api/hooks/distribution';

interface DistributionDrawerProps {
  open: boolean;
  mode: 'create' | 'edit';
  data?: Distribution;
  onClose: () => void;
  onSuccess: () => void;
}

/**
 * 分发配置编辑/创建抽屉
 */
const DistributionDrawer: React.FC<DistributionDrawerProps> = ({
  open,
  mode,
  data,
  onClose,
  onSuccess,
}) => {
  const { t } = useTranslation('distribution');
  const formRef = useRef<ProFormInstance>(null);
  const { message } = App.useApp();

  const [buildOptions, setBuildOptions] = useState<{ label: string; value: number }[]>([]);
  const [projectOptions, setProjectOptions] = useState<{ label: string; value: number }[]>([]);

  useEffect(() => {
    if (!open) return;
    fetchListBuilds(new PaginationQuery({ paging: { page: 1, pageSize: 100 } }))
      .then((resp: Build[] | any) => {
        const items = (resp.items ?? []) as Build[];
        setBuildOptions(items.map((b) => ({ label: b.name || String(b.id), value: b.id ?? 0 })));
      })
      .catch(() => setBuildOptions([]));
    fetchListProjects(new PaginationQuery({ paging: { page: 1, pageSize: 100 } }))
      .then((resp: any) => {
        const items = (resp.items ?? []) as Project[];
        setProjectOptions(items.map((p) => ({ label: p.name || String(p.id), value: p.id ?? 0 })));
      })
      .catch(() => setProjectOptions([]));
  }, [open]);

  const createMutation = useCreateDistribution({
    onSuccess: () => {
      message.success(t('createSuccess'));
      onSuccess();
      onClose();
    },
    onError: (error: Error) => message.error(error.message || t('createFailed')),
  });

  const updateMutation = useUpdateDistribution({
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
        destroyOnClose: true,
        width: 520,
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
          <ProFormSelect
            name="buildId"
            label={t('buildId')}
            options={buildOptions}
            rules={[{ required: true, message: t('buildRequired') }]}
            initialValue={data?.buildId}
          />
          <ProFormSelect
            name="projectIds"
            mode="multiple"
            label={t('projectId')}
            options={projectOptions}
            rules={[{ required: true, message: t('projectRequired') }]}
            initialValue={data?.projectIds}
          />
          <ProFormSwitch
            name="clearOld"
            label={t('clearOld')}
            extra={t('clearOldHint')}
            initialValue={data?.clearOld ?? false}
          />
          <ProFormSelect
            name="afterOpt"
            label={t('afterOpt')}
            options={[
              { label: t('afterNone'), value: 'NONE' },
              { label: t('afterRestart'), value: 'RESTART' },
              { label: t('afterStart'), value: 'START' },
            ]}
            initialValue={data?.afterOpt ?? 'NONE'}
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

export default DistributionDrawer;
