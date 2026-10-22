import { useState } from 'react';
import { Alert, Modal, Table, Tag } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import { useQuery } from '@tanstack/react-query';
import dayjs from 'dayjs';
import type { taskservicev1_TaskExecution as TaskExecution } from '@/api/generated/admin/service/v1';
import { fetchListTaskExecutions } from '@/api/hooks/task';

interface TaskExecutionsModalProps {
  open: boolean;
  /** 任务类型名称过滤(空则查全部) */
  typeName?: string;
  onClose: () => void;
}

/** asynq 实例状态 → Tag 颜色 */
const stateColorMap: Record<string, string> = {
  active: 'processing',
  pending: 'default',
  scheduled: 'cyan',
  retry: 'warning',
  archived: 'default',
  completed: 'success',
};

/**
 * 任务执行记录(Asynq 实例快照):单次任务实例的状态/载荷/重试与最近错误,
 * 深度受服务端 retention 窗口约束,非全量历史。
 *
 * 父组件以 key=打开序号 挂载本组件——每次打开都是全新状态,分页自然从
 * 第 1 页开始,无需在 effect 里同步重置(规避 set-state-in-effect 级联渲染)。
 */
const TaskExecutionsModal = ({ open, typeName, onClose }: TaskExecutionsModalProps) => {
  const { t } = useTranslation('task');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);

  const query = useQuery({
    queryKey: ['taskExecutions', typeName, page, pageSize],
    queryFn: () => fetchListTaskExecutions({ typeName, page, pageSize }),
    enabled: open,
  });

  const columns: ColumnsType<TaskExecution> = [
    {
      title: t('execState'),
      dataIndex: 'state',
      width: 100,
      render: (_, record) => {
        const state = record.state || '';
        return <Tag color={stateColorMap[state] || 'default'}>{state}</Tag>;
      },
    },
    {
      title: t('taskPayload'),
      dataIndex: 'payload',
      ellipsis: true,
    },
    {
      title: t('execRetried'),
      dataIndex: 'retried',
      width: 90,
    },
    {
      title: t('execLastError'),
      dataIndex: 'lastError',
      ellipsis: true,
    },
    {
      title: t('execCompletedAt'),
      dataIndex: 'completedAt',
      width: 170,
      render: (_, record) =>
        record.completedAt ? dayjs(record.completedAt).format('YYYY-MM-DD HH:mm:ss') : '-',
    },
    {
      title: t('execNextProcessAt'),
      dataIndex: 'nextProcessAt',
      width: 170,
      render: (_, record) =>
        record.nextProcessAt ? dayjs(record.nextProcessAt).format('YYYY-MM-DD HH:mm:ss') : '-',
    },
  ];

  return (
    <Modal
      title={t('executionsTitle', { typeName: typeName || t('allTypes') })}
      open={open}
      onCancel={onClose}
      footer={null}
      width={880}
      destroyOnHidden
    >
      {query.isError && (
        <Alert
          type="error"
          showIcon
          message={t('executionsLoadFailed')}
          description={String(query.error)}
          style={{ marginBottom: 12 }}
        />
      )}
      <Table<TaskExecution>
        rowKey={(record) => record.id || `${record.typeName}-${record.nextProcessAt}`}
        columns={columns}
        dataSource={query.data?.items || []}
        loading={query.isFetching}
        size="small"
        scroll={{ y: 420 }}
        pagination={{
          current: page,
          pageSize,
          total: query.data?.total || 0,
          showSizeChanger: true,
          onChange: (p, ps) => {
            setPage(p);
            setPageSize(ps);
          },
        }}
      />
    </Modal>
  );
};

export default TaskExecutionsModal;
