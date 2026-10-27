import { useRef, useState } from 'react';
import type { ProColumns, ActionType, ProFormInstance } from '@ant-design/pro-components';
import { DrawerForm, ProFormDateTimePicker, ProFormSwitch, ProFormText, ProFormTextArea } from '@ant-design/pro-components';
import ListTable from '@/components/common/ListTable';
import { App, Button, Popconfirm, Space, Tag } from 'antd';
import { DeleteOutlined, EditOutlined, PlusOutlined } from '@ant-design/icons';
import { useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_MaintenanceWindow as MaintenanceWindow } from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { TABLE } from '@/config/constants';
import {
  fetchListMaintenanceWindows,
  useCreateMaintenanceWindow,
  useDeleteMaintenanceWindow,
  useUpdateMaintenanceWindow,
} from '@/api/hooks/maintenance-window';
import { useProTableScrollY } from '@/hooks/useProTableScrollY';
import ContentContainer from '@/layouts/components/PageContainer/ContentContainer';

/**
 * 告警维护窗口:计划停机/发布窗口期静默告警通知(告警仍落库,恢复通知不受影响)。
 */
const MaintenanceWindowManagement = () => {
  const { t } = useTranslation('maintenance-window');
  const actionRef = useRef<ActionType>(null);
  const formRef = useRef<ProFormInstance>(null);
  const queryClient = useQueryClient();
  const { message } = App.useApp();
  const containerRef = useRef<HTMLDivElement>(null);
  const tableScrollY = useProTableScrollY(containerRef);

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerMode, setDrawerMode] = useState<'create' | 'edit'>('create');
  const [editing, setEditing] = useState<MaintenanceWindow | undefined>();

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['listMaintenanceWindows'] });

  const saveMutation = useCreateMaintenanceWindow({
    onSuccess: () => {
      message.success(t('createSuccess'));
      invalidate();
      actionRef.current?.reload();
    },
    onError: (e: Error) => message.error(e.message || t('createFailed')),
  });
  const updateMutation = useUpdateMaintenanceWindow({
    onSuccess: () => {
      message.success(t('updateSuccess'));
      invalidate();
      actionRef.current?.reload();
    },
    onError: (e: Error) => message.error(e.message || t('updateFailed')),
  });
  const deleteMutation = useDeleteMaintenanceWindow({
    onSuccess: () => {
      message.success(t('deleteSuccess'));
      invalidate();
      actionRef.current?.reload();
    },
    onError: (e: Error) => message.error(e.message || t('deleteFailed')),
  });

  const columns: ProColumns<MaintenanceWindow>[] = [
    { title: t('name'), dataIndex: 'name', width: 200, render: (_, r) => <span style={{ fontWeight: 600 }}>{r.name}</span> },
    {
      title: t('timeRange'),
      width: 340,
      render: (_, r) => `${fmtTime(r.startsAt)} ~ ${fmtTime(r.endsAt)}`,
    },
    {
      title: t('enabled'),
      dataIndex: 'enabled',
      width: 100,
      render: (_, r) => (r.enabled ? <Tag color="success">{t('on')}</Tag> : <Tag color="default">{t('off')}</Tag>),
    },
    { title: t('description'), dataIndex: 'description', ellipsis: true },
    { title: t('createdAt'), dataIndex: 'createdAt', width: 165, render: fmtTime },
    {
      title: t('action'),
      valueType: 'option',
      width: 120,
      fixed: 'right',
      render: (_, record) => [
        <a
          key="edit"
          onClick={() => {
            setDrawerMode('edit');
            setEditing(record);
            setDrawerOpen(true);
          }}
        >
          <EditOutlined />
        </a>,
        <Popconfirm
          key="delete"
          title={t('deleteConfirmTitle')}
          onConfirm={() => record.id && deleteMutation.mutate({ id: record.id })}
          okText={t('common:button.ok')}
          cancelText={t('common:button.cancel')}
        >
          <a style={{ color: 'var(--ant-color-error)' }}>
            <DeleteOutlined />
          </a>
        </Popconfirm>,
      ],
    },
  ];

  return (
    <>
      <ContentContainer heightMode="fixed" padding="16px" bottomMargin={0}>
        <div ref={containerRef} className="page-container-content">
          <ListTable<MaintenanceWindow>
            actionRef={actionRef}
            columns={columns}
            headerTitle={t('moduleName')}
            request={async (params) => {
              const query = new PaginationQuery({
                paging: { page: params.current || 1, pageSize: params.pageSize || 10 },
              });
              const resp = await fetchListMaintenanceWindows(query);
              return {
                data: resp.items || [],
                total: resp.total || 0,
                success: true,
              };
            }}
            rowKey="id"
            search={false}
            pagination={{ defaultPageSize: TABLE.DEFAULT_PAGE_SIZE, showSizeChanger: true }}
            toolBarRender={() => [
              <Button
                key="create"
                type="primary"
                icon={<PlusOutlined />}
                onClick={() => {
          setEditing(undefined);
          setDrawerMode('create');
          setDrawerOpen(true);
        }}
              >
                {t('create')}
              </Button>,
            ]}
            size="middle"
            bordered
            scroll={{ y: tableScrollY, x: 940 }}
          />
        </div>
      </ContentContainer>

      <MaintenanceWindowDrawer
        open={drawerOpen}
        mode={drawerMode}
        data={editing}
        formRef={formRef}
        onClose={() => {
          setDrawerOpen(false);
          setEditing(undefined);
        }}
        onSave={async (values) => {
          if (drawerMode === 'create') await saveMutation.mutateAsync(values);
          else if (editing?.id) await updateMutation.mutateAsync({ id: editing.id, values });
        }}
      />
    </>
  );
};

interface DrawerProps {
  open: boolean;
  mode: 'create' | 'edit';
  data?: MaintenanceWindow;
  formRef: React.RefObject<ProFormInstance | null>;
  onClose: () => void;
  onSave: (values: Record<string, unknown>) => Promise<void>;
}

const MaintenanceWindowDrawer: React.FC<DrawerProps> = ({ open, mode, data, formRef, onClose, onSave }) => {
  const { t } = useTranslation('maintenance-window');

  return (
    <DrawerForm
      title={mode === 'create' ? t('create') : `${t('edit')}: ${data?.name ?? ''}`}
      formRef={formRef}
      open={open}
      drawerProps={{ onClose, destroyOnHidden: true, width: 480 }}
      onFinish={async (values: Record<string, unknown>) => {
        await onSave(values);
        onClose();
        return true;
      }}
      params={{ data, mode }}
    >
      {open && (
        <>
          <ProFormText
            name="name"
            label={t('name')}
            rules={[{ required: true, message: t('nameRequired') }]}
            initialValue={data?.name}
          />
          <Space.Compact block>
            <ProFormDateTimePicker
              name="startsAt"
              label={t('startsAt')}
              width="sm"
              rules={[{ required: true, message: t('timeRequired') }]}
              initialValue={data?.startsAt?.slice(0, 19)}
            />
            <ProFormDateTimePicker
              name="endsAt"
              label={t('endsAt')}
              width="sm"
              rules={[{ required: true, message: t('timeRequired') }]}
              initialValue={data?.endsAt?.slice(0, 19)}
            />
          </Space.Compact>
          <ProFormSwitch
            name="enabled"
            label={t('enabled')}
            checkedChildren={t('on')}
            unCheckedChildren={t('off')}
            initialValue={data?.enabled ?? true}
          />
          <ProFormTextArea name="description" label={t('description')} fieldProps={{ rows: 2, allowClear: true }} initialValue={data?.description} />
        </>
      )}
    </DrawerForm>
  );
};

function fmtTime(v: unknown): string {
  if (!v) return '-';
  return String(v).replace('T', ' ').slice(0, 19);
}

export default MaintenanceWindowManagement;
