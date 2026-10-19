import { useRef, useState } from 'react';
import type { ProColumns, ActionType } from '@ant-design/pro-components';
import ListTable from '@/components/common/ListTable';
import { Button, Popconfirm, Tag, Tooltip, App } from 'antd';
import {
  DeleteOutlined,
  EditOutlined,
  PlayCircleOutlined,
  PauseCircleOutlined,
  ReloadOutlined,
  FileTextOutlined,
  HistoryOutlined,
  PlusOutlined,
} from '@ant-design/icons';
import { useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_Project as Project } from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { ENV_PRESETS, ENV_TAG_COLOR, TABLE } from '@/config/constants';
import {
  fetchListProjects,
  useDeleteProject,
  useOperateProject,
} from '@/api/hooks/project';
import { useProTableScrollY } from '@/hooks/useProTableScrollY';

import ContentContainer from '@/layouts/components/PageContainer/ContentContainer';
import ProjectDrawer from './components/ProjectDrawer';
import ProjectLogDrawer from './components/ProjectLogDrawer';
import ProjectTimelineDrawer from './components/ProjectTimelineDrawer';

/**
 * 项目管理页面
 */
const ProjectManagement = () => {
  const { t } = useTranslation('project');
  const actionRef = useRef<ActionType>(null);
  const queryClient = useQueryClient();
  const { message } = App.useApp();

  const containerRef = useRef<HTMLDivElement>(null);
  const tableScrollY = useProTableScrollY(containerRef);

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [drawerMode, setDrawerMode] = useState<'create' | 'edit'>('create');
  const [selectedProject, setSelectedProject] = useState<Project | undefined>();
  const [logProject, setLogProject] = useState<Project | undefined>();
  const [timelineProject, setTimelineProject] = useState<Project | undefined>();

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['listProjects'] });

  const deleteMutation = useDeleteProject({
    onSuccess: () => {
      message.success(t('deleteSuccess'));
      invalidate();
      actionRef.current?.reload();
    },
    onError: (error: Error) => message.error(error.message || t('deleteFailed')),
  });

  const operateMutation = useOperateProject({
    onSuccess: (resp) => {
      message.success(t('operateSent', { requestId: resp.requestId }));
      // 稍后刷新以拿到回填的运行状态
      setTimeout(() => {
        invalidate();
        actionRef.current?.reload();
      }, 2500);
    },
    onError: (error: Error) => message.error(error.message || t('operateFailed')),
  });

  const statusTag = (status: string | undefined) => {
    switch (status) {
      case 'RUNNING':
        return <Tag color="processing">{t('statusRunning')}</Tag>;
      case 'STOPPED':
        return <Tag color="default">{t('statusStopped')}</Tag>;
      default:
        return <Tag>{t('statusUnknown')}</Tag>;
    }
  };

  const operate = (record: Project, operation: 'START' | 'STOP' | 'RESTART' | 'STATUS') => {
    if (record.id) operateMutation.mutate({ id: record.id, operation });
  };

  const columns: ProColumns<Project>[] = [
    {
      title: t('env'),
      dataIndex: 'env',
      width: 100,
      valueType: 'select',
      fieldProps: { options: ENV_PRESETS.map((v) => ({ label: v, value: v })) },
      render: (_, record) => {
        const env = record.env || 'default';
        return <Tag color={ENV_TAG_COLOR[env] ?? 'default'}>{env}</Tag>;
      },
    },
    {
      title: t('name'),
      dataIndex: 'name',
      width: 140,
      render: (_, record) => (
        <span style={{ fontWeight: 600 }}>{record.name || '-'}</span>
      ),
    },
    {
      title: t('nodeId'),
      dataIndex: 'nodeId',
      width: 80,
      hideInSearch: true,
    },
    {
      title: t('projectDir'),
      dataIndex: 'projectDir',
      width: 220,
      hideInSearch: true,
      ellipsis: true,
    },
    {
      title: t('lastStatus'),
      width: 90,
      hideInSearch: true,
      render: (_, record) => statusTag(record.lastStatus),
    },
    {
      title: t('lastPid'),
      dataIndex: 'lastPid',
      width: 90,
      hideInSearch: true,
      render: (_, record) => (record.lastPid ? String(record.lastPid) : '-'),
    },
    {
      title: t('description'),
      dataIndex: 'description',
      width: 160,
      hideInSearch: true,
      ellipsis: true,
    },
    {
      title: t('action'),
      valueType: 'option',
      width: 260,
      fixed: 'right',
      render: (_, record) => [
        <Tooltip key="start" title={t('opStart')}>
          <a onClick={() => operate(record, 'START')}>
            <PlayCircleOutlined />
          </a>
        </Tooltip>,
        <Tooltip key="stop" title={t('opStop')}>
          <a onClick={() => operate(record, 'STOP')}>
            <PauseCircleOutlined />
          </a>
        </Tooltip>,
        <Tooltip key="restart" title={t('opRestart')}>
          <a onClick={() => operate(record, 'RESTART')}>
            <ReloadOutlined />
          </a>
        </Tooltip>,
        <Tooltip key="log" title={t('opLog')}>
          <a onClick={() => setLogProject(record)}>
            <FileTextOutlined />
          </a>
        </Tooltip>,
        <Tooltip key="timeline" title={t('opTimeline')}>
          <a onClick={() => setTimelineProject(record)}>
            <HistoryOutlined />
          </a>
        </Tooltip>,
        <Tooltip key="edit" title={t('edit')}>
          <a
            onClick={() => {
              setDrawerMode('edit');
              setSelectedProject(record);
              setDrawerOpen(true);
            }}
          >
            <EditOutlined />
          </a>
        </Tooltip>,
        <Popconfirm
          key="delete"
          title={t('deleteConfirmTitle')}
          description={t('deleteConfirmDesc', { moduleName: t('moduleName') })}
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
          <ListTable<Project>
            actionRef={actionRef}
            columns={columns}
            request={async (params, sorter, _filter) => {
              const query = new PaginationQuery({
                paging: {
                  page: params.current || 1,
                  pageSize: params.pageSize || 10,
                },
                formValues: Object.fromEntries(
                  Object.entries(params).filter(
                    ([key]) => !['current', 'pageSize'].includes(key),
                  ),
                ),
                orderBy:
                  sorter && Object.keys(sorter).length > 0
                    ? Object.entries(sorter).map(([key, value]) =>
                        value === 'ascend' ? key : `-${key}`,
                      )
                    : undefined,
              });

              const response = await fetchListProjects(query);

              return {
                data: response.items || [],
                total: response.total || 0,
                success: true,
              };
            }}
            rowKey="id"
            search={{
              labelWidth: 'auto',
              defaultCollapsed: false,
            }}
            pagination={{
              defaultPageSize: TABLE.DEFAULT_PAGE_SIZE,
              showSizeChanger: true,
              showQuickJumper: true,
            }}
            toolBarRender={() => [
              <Button
                key="create"
                type="primary"
                icon={<PlusOutlined />}
                onClick={() => {
                  setDrawerMode('create');
                  setSelectedProject(undefined);
                  setDrawerOpen(true);
                }}
              >
                {t('create')}
              </Button>,
            ]}
            options={{
              density: true,
              fullScreen: true,
              setting: true,
              reload: true,
            }}
            size="middle"
            bordered
            cardBordered={false}
            scroll={{
              y: tableScrollY,
              x: 1300,
            }}
          />
        </div>
      </ContentContainer>

      <ProjectDrawer
        open={drawerOpen}
        mode={drawerMode}
        data={selectedProject}
        onClose={() => {
          setDrawerOpen(false);
          setSelectedProject(undefined);
        }}
        onSuccess={() => {
          invalidate();
          actionRef.current?.reload();
        }}
      />

      <ProjectLogDrawer
        data={logProject}
        onClose={() => setLogProject(undefined)}
      />

      <ProjectTimelineDrawer
        projectId={timelineProject?.id}
        projectName={timelineProject?.name}
        onClose={() => setTimelineProject(undefined)}
      />
    </>
  );
};

export default ProjectManagement;
