import { useState } from 'react';
import { App, Button, Drawer, Upload } from 'antd';
import { DownloadOutlined, InboxOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { runImportFile, generateImportTemplate, type ImportField } from '@/utils/import';

/**
 * 通用 Excel 导入抽屉：客户端模板下载 + xlsx 解析 + 逐行落库。
 * 落库经调用方注入的 createRow（页面既有 create 变体），
 * { data } 包裹/校验/租户隔离/审计全走既有链路。
 */
export interface ImportDrawerProps {
  open: boolean;
  /** 可导入字段（label 即模板表头） */
  fields: ImportField[];
  /** 单行落库动作（页面既有 create mutation 的 mutateAsync） */
  createRow: (values: Record<string, any>) => Promise<any>;
  onClose: () => void;
  onSuccess?: (imported: number) => void;
}

export default function ImportDrawer({
  open,
  fields,
  createRow,
  onClose,
  onSuccess,
}: ImportDrawerProps) {
  const { t } = useTranslation('common');
  const { message } = App.useApp();
  const [file, setFile] = useState<File | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const handleDownloadTemplate = async () => {
    const { data } = await generateImportTemplate(fields);
    const blob = new Blob([data], {
      type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
    });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'import-template.xlsx';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };

  const handleOk = async () => {
    if (!file) return;
    setSubmitting(true);
    try {
      const imported = await runImportFile(file, fields, createRow);
      message.success(t('import.importSuccess', { count: imported }));
      reset();
      onSuccess?.(imported);
      onClose();
    } catch (err: any) {
      message.error(`${t('import.importFailed')}：${err?.message ?? ''}`);
    } finally {
      setSubmitting(false);
    }
  };

  const reset = () => setFile(null);

  const handleCancel = () => {
    reset();
    onClose();
  };

  return (
    <Drawer
      title={t('import.title')}
      open={open}
      onClose={handleCancel}
      destroyOnHidden
      size={520}
      footer={
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
          <Button onClick={handleCancel}>{t('button.cancel')}</Button>
          <Button type="primary" disabled={!file} loading={submitting} onClick={handleOk}>
            {t('button.ok')}
          </Button>
        </div>
      }
    >
      <Upload.Dragger
        accept=".xlsx"
        maxCount={1}
        beforeUpload={(f) => {
          setFile(f);
          return false; // 手动控制：选择即停，提交时统一解析
        }}
        onRemove={() => setFile(null)}
        fileList={file ? [file as any] : []}
      >
        <p className="ant-upload-drag-icon">
          <InboxOutlined />
        </p>
        <p className="ant-upload-text">{t('import.uploadText')}</p>
        <p className="ant-upload-hint">{t('import.uploadHint')}</p>
      </Upload.Dragger>
      <div style={{ marginTop: 8 }}>
        <Button type="link" icon={<DownloadOutlined />} onClick={handleDownloadTemplate}>
          {t('import.downloadTemplate')}
        </Button>
      </div>
    </Drawer>
  );
}
