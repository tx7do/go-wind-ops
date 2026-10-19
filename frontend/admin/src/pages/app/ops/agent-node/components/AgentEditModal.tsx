import { useEffect } from 'react';
import { Form, Input, Modal, Select } from 'antd';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_AgentNode as AgentNode } from '@/api/generated/admin/service/v1';
import { useUpdateAgentNode } from '@/api/hooks/agent-node';

const ENV_OPTIONS = ['default', 'dev', 'staging', 'prod'].map((e) => ({ label: e, value: e }));

interface Props {
  node?: AgentNode;
  onClose: () => void;
}

interface EditValues {
  name: string;
  env: string;
  labels?: string;
}

/**
 * 节点编辑弹窗:显示名/环境标签/标签(机器指纹由 agent 维护,不可改)。
 */
const AgentEditModal = ({ node, onClose }: Props) => {
  const { t } = useTranslation('agent-node');
  const [form] = Form.useForm();

  const updateMutation = useUpdateAgentNode({
    onSuccess: () => {
      onClose();
    },
  });

  useEffect(() => {
    if (!node) return;
    form.setFieldsValue({
      name: node.name,
      env: node.env || 'default',
      labels: (node.labels || []).join(','),
    });
  }, [node, form]);

  const submit = async () => {
    if (!node?.id) return;
    const values = (await form.validateFields()) as EditValues;
    await updateMutation.mutateAsync({
      id: node.id,
      values: {
        name: values.name,
        env: values.env,
        labels: (values.labels || '')
          .split(',')
          .map((l) => l.trim())
          .filter(Boolean),
      },
    });
  };

  return (
    <Modal
      title={t('editTitle', { name: node?.name || node?.hostname })}
      open={!!node}
      onCancel={onClose}
      onOk={submit}
      okText={t('common:button.ok')}
      cancelText={t('common:button.cancel')}
      confirmLoading={updateMutation.isPending}
      width={440}
      destroyOnHidden
    >
      <Form form={form} layout="vertical">
        <Form.Item name="name" label={t('name')} rules={[{ required: true }]}>
          <Input />
        </Form.Item>
        <Form.Item name="env" label={t('env')}>
          <Select options={ENV_OPTIONS} />
        </Form.Item>
        <Form.Item name="labels" label={t('labels')} extra={t('labelsHint')}>
          <Input placeholder="edge,window" />
        </Form.Item>
      </Form>
    </Modal>
  );
};

export default AgentEditModal;
