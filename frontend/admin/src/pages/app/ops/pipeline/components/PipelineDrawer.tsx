import { useEffect, useMemo, useRef, useState } from 'react';
import type { ProFormInstance } from '@ant-design/pro-components';
import {
  DrawerForm,
  ProFormDependency,
  ProFormList,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { App } from 'antd';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_Pipeline as Pipeline } from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { fetchListBuilds } from '@/api/hooks/build';
import { fetchListScripts } from '@/api/hooks/script';
import { fetchListDistributions } from '@/api/hooks/distribution';
import { useCreatePipeline, useUpdatePipeline } from '@/api/hooks/pipeline';

interface PipelineDrawerProps {
  open: boolean;
  mode: 'create' | 'edit';
  data?: Pipeline;
  onClose: () => void;
  onSuccess: () => void;
}

const STEP_TYPE_OPTIONS = [
  { label: 'BUILD', value: 'BUILD' },
  { label: 'SCRIPT', value: 'SCRIPT' },
  { label: 'DISTRIBUTION', value: 'DISTRIBUTION' },
];

/**
 * 流水线编辑/创建抽屉:步骤列表按序执行、失败即中止。
 * 步骤行内按 type 切换引用对象下拉(构建/脚本/分发配置)。
 */
const PipelineDrawer: React.FC<PipelineDrawerProps> = ({ open, mode, data, onClose, onSuccess }) => {
  const { t } = useTranslation('pipeline');
  const formRef = useRef<ProFormInstance>(null);
  const { message } = App.useApp();

  const [buildOptions, setBuildOptions] = useState<{ label: string; value: number }[]>([]);
  const [scriptOptions, setScriptOptions] = useState<{ label: string; value: number }[]>([]);
  const [distOptions, setDistOptions] = useState<{ label: string; value: number }[]>([]);

  useEffect(() => {
    if (!open) return;
    fetchListBuilds(new PaginationQuery({ paging: { page: 1, pageSize: 200 } }))
      .then((r) => setBuildOptions((r.items ?? []).map((b) => ({ label: b.name || String(b.id), value: b.id ?? 0 }))))
      .catch(() => setBuildOptions([]));
    fetchListScripts(new PaginationQuery({ paging: { page: 1, pageSize: 200 } }))
      .then((r) => setScriptOptions((r.items ?? []).map((x) => ({ label: x.name || String(x.id), value: x.id ?? 0 }))))
      .catch(() => setScriptOptions([]));
    fetchListDistributions(new PaginationQuery({ paging: { page: 1, pageSize: 200 } }))
      .then((r) => setDistOptions((r.items ?? []).map((d) => ({ label: d.name || String(d.id), value: d.id ?? 0 }))))
      .catch(() => setDistOptions([]));
  }, [open]);

  const createMutation = useCreatePipeline({
    onSuccess: () => {
      message.success(t('createSuccess'));
      onSuccess();
      onClose();
    },
    onError: (e: Error) => message.error(e.message || t('createFailed')),
  });
  const updateMutation = useUpdatePipeline({
    onSuccess: () => {
      message.success(t('updateSuccess'));
      onSuccess();
      onClose();
    },
    onError: (e: Error) => message.error(e.message || t('updateFailed')),
  });

  const refOptions = useMemo(
    () => ({ BUILD: buildOptions, SCRIPT: scriptOptions, DISTRIBUTION: distOptions }),
    [buildOptions, scriptOptions, distOptions],
  );

  const handleSubmit = async (values: any) => {
    if (mode === 'create') {
      await createMutation.mutateAsync({ data: values });
    } else if (data?.id) {
      await updateMutation.mutateAsync({ id: data.id, data: values });
    }
  };

  return (
    <DrawerForm
      title={mode === 'create' ? t('create') : t('edit')}
      formRef={formRef}
      open={open}
      drawerProps={{ onClose, destroyOnClose: true, width: 640 }}
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
          <ProFormList
            name="steps"
            label={t('steps')}
            min={1}
            copyIconProps={false}
            creatorButtonProps={{ creatorButtonText: t('addStep') }}
            initialValue={data?.steps ?? [{}]}
          >
            <ProFormDependency name={['type']}>
              {({ type }: { type?: any }, _form?: any, action?: any) => (
                <div style={{ display: 'flex', gap: 8, alignItems: 'flex-start', width: '100%' }}>
                  <ProFormSelect
                    name="type"
                    label={t('stepType')}
                    options={STEP_TYPE_OPTIONS}
                    style={{ width: 160 }}
                    fieldProps={{
                      onChange: () => {
                        // 类型切换后清空引用选择
                        const rowKey = (action as any)?.namePath ?? [];
                        if (rowKey?.length === 2) {
                          formRef.current?.setFieldValue([...rowKey, 'refId'], undefined);
                        }
                      },
                    }}
                  />
                  <ProFormSelect
                    name="refId"
                    label={t('stepRef')}
                    options={(refOptions as any)[type] ?? []}
                    rules={[{ required: true, message: t('stepRefRequired') }]}
                    style={{ flex: 1, minWidth: 180 }}
                  />
                  {type === 'SCRIPT' && (
                    <ProFormText
                      name="agentId"
                      label={t('stepAgent')}
                      rules={[{ required: true, message: t('stepAgentRequired') }]}
                      style={{ width: 200 }}
                      placeholder="agent-xxxx"
                    />
                  )}
                </div>
              )}
            </ProFormDependency>
          </ProFormList>
          <ProFormTextArea
            name="description"
            label={t('description')}
            fieldProps={{ rows: 2, allowClear: true }}
            initialValue={data?.description}
          />
          <div style={{ color: 'var(--ant-color-text-tertiary)', fontSize: 12 }}>{t('failFastHint')}</div>
        </>
      )}
    </DrawerForm>
  );
};

export default PipelineDrawer;
