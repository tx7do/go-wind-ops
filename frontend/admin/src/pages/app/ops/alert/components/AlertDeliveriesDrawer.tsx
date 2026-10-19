import { useEffect, useRef, useState } from 'react';
import type { ProColumns, ActionType } from '@ant-design/pro-components';
import { Drawer, Tag } from 'antd';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_NotificationDelivery as NotificationDelivery } from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { TABLE } from '@/config/constants';
import ListTable from '@/components/common/ListTable';
import { apiClient } from '@/api/client';
import { useProTableScrollY } from '@/hooks/useProTableScrollY';

interface Props {
  alertId?: number;
  onClose: () => void;
}

/**
 * 告警投递记录抽屉:该告警经各通知渠道的外呼结果(SENT/FAILED 与原因)。
 */
const AlertDeliveriesDrawer = ({ alertId, onClose }: Props) => {
  const { t } = useTranslation('alert');
  const actionRef = useRef<ActionType>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const tableScrollY = useProTableScrollY(containerRef);
  const [open, setOpen] = useState(false);

  useEffect(() => {
    setOpen(!!alertId);
  }, [alertId]);

  const columns: ProColumns<NotificationDelivery>[] = [
    { title: t('deliveryTime'), dataIndex: 'createdAt', width: 165, valueType: 'dateTime' },
    {
      title: t('deliveryChannel'),
      dataIndex: 'channelType',
      width: 100,
      render: (_, r) => <Tag color={r.channelType === 'EMAIL' ? 'cyan' : 'blue'}>{r.channelType}</Tag>,
    },
    {
      title: t('deliveryTarget'),
      dataIndex: 'target',
      width: 200,
      ellipsis: true,
      render: (_, r) => r.target || '-',
    },
    {
      title: t('deliveryStatus'),
      dataIndex: 'status',
      width: 100,
      render: (_, r) =>
        r.status === 'SENT' ? (
          <Tag color="success">{t('deliverySent')}</Tag>
        ) : (
          <Tag color="error" title={r.lastError || undefined}>
            {t('deliveryFailed')}
          </Tag>
        ),
    },
    {
      title: t('deliveryError'),
      dataIndex: 'lastError',
      ellipsis: true,
      render: (_, r) => r.lastError || '-',
    },
  ];

  return (
    <Drawer
      title={t('deliveriesTitle', { id: alertId })}
      open={open}
      onClose={onClose}
      size={820}
      destroyOnHidden
    >
      <div ref={containerRef} className="page-container-content">
        <ListTable<NotificationDelivery>
          actionRef={actionRef}
          columns={columns}
          rowKey="id"
          request={async (params) => {
            if (!alertId) return { data: [], total: 0, success: true };
            const q = new PaginationQuery({
              paging: { page: params.current || 1, pageSize: params.pageSize || 10 },
              formValues: { alertId },
              orderBy: ['-createdAt'],
            });
            const resp = await apiClient.notificationDeliveryService.List(q.toRawParams());
            return { data: resp.items || [], total: resp.total || 0, success: true };
          }}
          search={false}
          pagination={{ defaultPageSize: TABLE.DEFAULT_PAGE_SIZE }}
          options={{ density: true, reload: true }}
          size="middle"
          bordered
          scroll={{ y: tableScrollY }}
        />
      </div>
    </Drawer>
  );
};

export default AlertDeliveriesDrawer;
