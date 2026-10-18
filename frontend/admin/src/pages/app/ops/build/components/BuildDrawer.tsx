import type { ProFormInstance } from '@ant-design/pro-components';
import {
  DrawerForm,
  ProFormDigit,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { App } from 'antd';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_Build as Build } from '@/api/generated/admin/service/v1';
import { fetchListRepositories } from '@/api/hooks/repository';
import { useCreateBuild, useUpdateBuild } from '@/api/hooks/build';
import { PaginationQuery } from '@/core';

interface BuildDrawerProps {
  open: boolean;
  mode: 'create' | 'edit';
  data?: Build;
  onClose: () => void;
  onSuccess: () => void;
}

/**
 * 构建配置编辑/创建抽屉
 */
const BuildDrawer: React.FC<BuildDrawerProps> = ({
  open,
  mode,
  data,
  onClose,
  onSuccess,
}) => {
  const { t } = useTranslation('build');
  const formRef = useRef<ProFormInstance>(null);
  const { message } = App.useApp();

  const [repoOptions, setRepoOptions] = useState<
    { label: string; value: number }[]
  >([]);

  useEffect(() => {
    if (!open) return;
    fetchListRepositories(new PaginationQuery({ paging: { page: 1, pageSize: 100 } }))
      .then((resp) => {
        setRepoOptions(
          (resp.items ?? []).map((r) => ({ label: r.name || String(r.id), value: r.id ?? 0 })),
        );
      })
      .catch(() => setRepoOptions([]));
  }, [open]);

  const createMutation = useCreateBuild({
    onSuccess: () => {
      message.success(t('createSuccess'));
      onSuccess();
      onClose();
    },
    onError: (error: Error) => message.error(error.message || t('createFailed')),
  });

  const updateMutation = useUpdateBuild({
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
          <ProFormSelect
            name="repositoryId"
            label={t('repositoryId')}
            options={repoOptions}
            rules={[{ required: true, message: t('repositoryRequired') }]}
            initialValue={data?.repositoryId}
          />
          <ProFormText
            name="branch"
            label={t('branch')}
            placeholder={t('branchPlaceholder')}
            initialValue={data?.branch}
          />
          <ProFormTextArea
            name="buildCommand"
            label={t('buildCommand')}
            rules={[{ required: true, message: t('buildCommandRequired') }]}
            fieldProps={{
              rows: 3,
              allowClear: true,
              style: { fontFamily: 'Consolas, Monaco, monospace', fontSize: 13 },
            }}
            placeholder="e.g. go build -o bin/app ./cmd/app"
            initialValue={data?.buildCommand}
          />
          <ProFormText
            name="artifactPath"
            label={t('artifactPath')}
            rules={[{ required: true, message: t('artifactPathRequired') }]}
            placeholder="dist/*.zip 或 bin/app.exe"
            extra={t('artifactPathHint')}
            initialValue={data?.artifactPath}
          />
          <ProFormDigit
            name="timeoutSeconds"
            label={t('timeoutSeconds')}
            min={1}
            max={3600}
            fieldProps={{ precision: 0 }}
            initialValue={600}
          />
          <ProFormText
            name="cron"
            label={t('cron')}
            placeholder="*/30 * * * *"
            extra={t('cronHint')}
            initialValue={data?.cron}
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

export default BuildDrawer;
