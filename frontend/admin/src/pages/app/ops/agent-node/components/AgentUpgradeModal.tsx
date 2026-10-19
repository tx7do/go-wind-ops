import { useRef, useState } from 'react';
import { Modal, Input, App, Alert, Progress } from 'antd';
import { InboxOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_AgentNode as AgentNode } from '@/api/generated/admin/service/v1';
import { useGetAgentUpgradeUploadUrl, useUpgradeAgent } from '@/api/hooks/agent-node';

interface Props {
  node?: AgentNode;
  onClose: () => void;
}

/**
 * Agent 远程升级弹窗:选升级包 → 预签名 PUT 直传 MinIO → 下发 agent_upgrade
 * 指令;agent 下载完成后自替换重启(以重启后心跳上报的版本为准)。
 */
const AgentUpgradeModal = ({ node, onClose }: Props) => {
  const { t } = useTranslation('agent-node');
  const { message } = App.useApp();
  const [version, setVersion] = useState('');
  const [file, setFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [percent, setPercent] = useState(0);
  const fileRef = useRef<HTMLInputElement>(null);

  const uploadUrlMutation = useGetAgentUpgradeUploadUrl();
  const upgradeMutation = useUpgradeAgent({
    onSuccess: (resp) => {
      message.success(t('upgradeSent', { requestId: (resp.requestId || '').slice(0, 8) }));
      onClose();
    },
    onError: (e: Error) => message.error(e.message || t('upgradeFailed')),
  });

  const handleUpload = async (uploadUrl: string) => {
    if (!file) return;
    await new Promise<void>((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      xhr.open('PUT', uploadUrl);
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) setPercent(Math.round((e.loaded / e.total) * 100));
      };
      xhr.onload = () => (xhr.status >= 200 && xhr.status < 300 ? resolve() : reject(new Error(`PUT ${xhr.status}`)));
      xhr.onerror = () => reject(new Error('upload network error'));
      xhr.send(file);
    });
  };

  const handleOk = async () => {
    if (!node?.id) return;
    if (!version.trim()) {
      message.warning(t('upgradeVersionRequired'));
      return;
    }
    if (!file) {
      message.warning(t('upgradeFileRequired'));
      return;
    }
    setUploading(true);
    setPercent(0);
    try {
      const presign = await uploadUrlMutation.mutateAsync({ filename: file.name });
      if (!presign.uploadUrl || !presign.objectKey) throw new Error('presign failed');
      await handleUpload(presign.uploadUrl);
      await upgradeMutation.mutateAsync({
        nodeId: node.id,
        version: version.trim(),
        objectKey: presign.objectKey,
      });
    } catch (e) {
      message.error((e as Error).message || t('upgradeFailed'));
    } finally {
      setUploading(false);
    }
  };

  return (
    <Modal
      title={t('upgradeTitle', { name: node?.name || node?.agentId })}
      open={!!node}
      onCancel={onClose}
      onOk={handleOk}
      okText={t('upgradeGo')}
      confirmLoading={uploading || upgradeMutation.isPending}
      width={480}
      destroyOnHidden
    >
      <Alert type="warning" showIcon message={t('upgradeWarning')} style={{ marginBottom: 16 }} />
      <div style={{ marginBottom: 8 }}>{t('upgradeVersionLabel')}</div>
      <Input
        placeholder="v1.2.3"
        value={version}
        onChange={(e) => setVersion(e.target.value)}
        style={{ marginBottom: 16 }}
      />
      <div style={{ marginBottom: 8 }}>{t('upgradeFileLabel')}</div>
      <div
        onClick={() => fileRef.current?.click()}
        style={{
          border: '1px dashed var(--ant-color-border)',
          borderRadius: 8,
          padding: '24px 12px',
          textAlign: 'center',
          cursor: 'pointer',
        }}
      >
        <InboxOutlined style={{ fontSize: 28, color: 'var(--ant-color-primary)' }} />
        <div style={{ marginTop: 8 }}>{file ? file.name : t('upgradeFilePlaceholder')}</div>
      </div>
      <input
        ref={fileRef}
        type="file"
        style={{ display: 'none' }}
        onChange={(e) => setFile(e.target.files?.[0] ?? null)}
      />
      {uploading && <Progress percent={percent} style={{ marginTop: 12 }} />}
      <div style={{ marginTop: 12, fontSize: 12, color: 'var(--ant-color-text-secondary)' }}>
        {t('upgradeHint')}
      </div>
    </Modal>
  );
};

export default AgentUpgradeModal;
