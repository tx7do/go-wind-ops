import { Card, Col, Popconfirm, Progress, Row, Space, Spin, Tag, Typography } from 'antd';
import {
  BellOutlined,
  CheckCircleOutlined,
  CloudServerOutlined,
  RocketOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { PaginationQuery } from '@/core';
import ContentContainer from '@/layouts/components/PageContainer/ContentContainer';
import { useListAgentNodes } from '@/api/hooks/agent-node';
import { useListAlerts, useResolveAlert } from '@/api/hooks/alert';
import { useApproveApproval, useListApprovals, useRejectApproval } from '@/api/hooks/approval';
import { useListBuildHistories } from '@/api/hooks/build-history';
import { useListProjects } from '@/api/hooks/project';
import { useUserStore } from '@/stores/user';

/** 看板自动刷新周期。 */
const REFRESH_MS = 15_000;

const dashQuery = () =>
  new PaginationQuery({
    paging: { page: 1, pageSize: 200 },
    orderBy: ['-createdAt'],
  });

const statusColor = (s?: string) =>
  s === 'SUCCESS' ? 'success' : s === 'FAILED' ? 'error' : s === 'RUNNING' || s === 'PENDING' ? 'processing' : 'default';

const fmtTime = (v?: string) => (v ? String(v).replace('T', ' ').slice(0, 19) : '-');

const percentColor = (p: number) => (p >= 90 ? '#ff4d4f' : p >= 70 ? '#faad14' : '#52c41a');

/**
 * 运维大盘:节点资源 / 待办审批 / 活跃告警 / 最近构建,15s 自动刷新。
 */
export default function DashboardPage() {
  const { t } = useTranslation('dashboard');
  const queryClient = useQueryClient();
  const userInfo = useUserStore((state) => state.userInfo);

  const common = { refetchInterval: REFRESH_MS } as const;
  const nodes = useListAgentNodes(dashQuery(), common);
  const projects = useListProjects(dashQuery(), common);
  const builds = useListBuildHistories(dashQuery(), common);
  const approvals = useListApprovals(dashQuery(), common);
  const alerts = useListAlerts(dashQuery(), common);

  const loading =
    nodes.isLoading || projects.isLoading || builds.isLoading || approvals.isLoading || alerts.isLoading;

  const nodeList = nodes.data?.items ?? [];
  const projectList = projects.data?.items ?? [];
  const buildList = builds.data?.items ?? [];
  const pendingApprovals = (approvals.data?.items ?? []).filter((a) => a.status === 'PENDING');
  const activeAlerts = (alerts.data?.items ?? []).filter((a) => !a.resolved);
  const onlineNodes = nodeList.filter((n) => n.status === 'ONLINE');
  const runningProjects = projectList.filter((p) => p.lastStatus === 'RUNNING');
  const successBuilds = buildList.filter((b) => b.status === 'SUCCESS');

  const invalidate = (key: string) => queryClient.invalidateQueries({ queryKey: [key] });
  const approveMutation = useApproveApproval({
    onSuccess: () => invalidate('listApprovals'),
  });
  const rejectMutation = useRejectApproval({
    onSuccess: () => invalidate('listApprovals'),
  });
  const resolveMutation = useResolveAlert({
    onSuccess: () => invalidate('listAlerts'),
  });

  const stats = [
    {
      key: 'onlineNodes',
      value: onlineNodes.length,
      suffix: `/ ${nodeList.length}`,
      color: '#52c41a',
      icon: <CloudServerOutlined />,
    },
    {
      key: 'runningProjects',
      value: runningProjects.length,
      suffix: `/ ${projectList.length}`,
      color: '#1890ff',
      icon: <RocketOutlined />,
    },
    {
      key: 'buildSuccess',
      value: successBuilds.length,
      suffix: `/ ${buildList.length}`,
      color: '#722ed1',
      icon: <CheckCircleOutlined />,
    },
    {
      key: 'pendingApprovals',
      value: pendingApprovals.length,
      color: pendingApprovals.length > 0 ? '#faad14' : '#8c8c8c',
      icon: <ThunderboltOutlined />,
    },
    {
      key: 'activeAlerts',
      value: activeAlerts.length,
      color: activeAlerts.length > 0 ? '#ff4d4f' : '#8c8c8c',
      icon: <BellOutlined />,
    },
  ];

  const today = new Date().toLocaleDateString('zh-CN', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    weekday: 'long',
  });

  if (loading) {
    return (
      <ContentContainer>
        <Spin style={{ display: 'block', margin: '80px auto' }} />
      </ContentContainer>
    );
  }

  return (
    <ContentContainer>
      <Row gutter={[16, 16]}>
        {/* ── 问候 + 自动刷新说明 ── */}
        <Col span={24}>
          <Space align="baseline" size={12}>
            <Typography.Title level={4} style={{ marginBottom: 0 }}>
              {t('greeting')}
              {userInfo?.nickname || userInfo?.username || ''}
            </Typography.Title>
            <Typography.Text type="secondary">{today}</Typography.Text>
            <Tag style={{ marginLeft: 'auto' }}>{t('autoRefresh')}</Tag>
          </Space>
        </Col>

        {/* ── 统计卡片 ── */}
        {stats.map((s) => (
          <Col xs={12} lg={Math.floor(24 / stats.length)} key={s.key}>
            <Card size="small">
              <Card.Meta
                avatar={
                  <span style={{ fontSize: 28, color: s.color }}>
                    {s.icon}
                  </span>
                }
                title={<Typography.Text type="secondary">{t(`stat.${s.key}`)}</Typography.Text>}
                description={
                  <span style={{ fontSize: 24, fontWeight: 600, color: s.color }}>
                    {s.value}
                    {s.suffix ? (
                      <span style={{ fontSize: 14, color: '#8c8c8c', marginLeft: 6 }}>{s.suffix}</span>
                    ) : null}
                  </span>
                }
              />
            </Card>
          </Col>
        ))}

        {/* ── 节点资源 ── */}
        <Col xs={24} xl={14}>
          <Card size="small" title={t('nodesPanel')} extra={<CloudServerOutlined />}>
            {nodeList.length === 0 ? (
              <Typography.Text type="secondary">{t('noData')}</Typography.Text>
            ) : (
              nodeList.map((n) => {
                const cpu = Math.round(n.cpuPercent ?? 0);
                const mem = Math.round(n.memoryUsedPercent ?? 0);
                const disk = Math.round(n.diskUsedPercent ?? 0);
                return (
                  <div key={n.id} style={{ padding: '8px 0', borderBottom: '1px solid #f0f0f0' }}>
                    <Space size={8} style={{ marginBottom: 4 }}>
                      <Tag color={n.status === 'ONLINE' ? 'success' : 'default'}>
                        {n.status === 'ONLINE' ? t('online') : t('offline')}
                      </Tag>
                      <span style={{ fontWeight: 600 }}>{n.name || n.hostname}</span>
                      {n.env ? <Tag>{n.env}</Tag> : null}
                      <span style={{ color: '#8c8c8c', fontSize: 12 }}>{n.platform}</span>
                    </Space>
                    <Row gutter={16}>
                      {[
                        ['cpu', cpu],
                        ['mem', mem],
                        ['disk', disk],
                      ].map(([label, v]) => (
                        <Col span={8} key={label as string}>
                          <Space size={6}>
                            <span style={{ fontSize: 12, color: '#8c8c8c' }}>{t(label as string)}</span>
                            <Progress
                              percent={v as number}
                              size="small"
                              strokeColor={percentColor(v as number)}
                              style={{ width: 120, marginBottom: 0 }}
                            />
                          </Space>
                        </Col>
                      ))}
                    </Row>
                  </div>
                );
              })
            )}
          </Card>
        </Col>

        {/* ── 待办审批 ── */}
        <Col xs={24} xl={10}>
          <Card
            size="small"
            title={
              <Space>
                {t('pendingPanel')}
                {pendingApprovals.length > 0 && <Tag color="warning">{pendingApprovals.length}</Tag>}
              </Space>
            }
            extra={<ThunderboltOutlined />}
          >
            {pendingApprovals.length === 0 ? (
              <Typography.Text type="secondary">{t('noPending')}</Typography.Text>
            ) : (
              pendingApprovals.slice(0, 6).map((a) => (
                <div key={a.id} style={{ padding: '6px 0', borderBottom: '1px solid #f0f0f0' }}>
                  <Space size={6} wrap>
                    <Tag color="warning">{a.operationType}</Tag>
                    <span>{a.operationName || `#${a.operationRefId}`}</span>
                    <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                      {fmtTime(a.createdAt)}
                    </Typography.Text>
                    <Space size={4} style={{ marginLeft: 'auto' }}>
                      <Popconfirm title={t('approveConfirm')} onConfirm={() => a.id && approveMutation.mutate({ id: a.id })}>
                        <a style={{ color: '#52c41a', fontSize: 12 }}>{t('approve')}</a>
                      </Popconfirm>
                      <Popconfirm title={t('rejectConfirm')} onConfirm={() => a.id && rejectMutation.mutate({ id: a.id })}>
                        <a style={{ color: '#ff4d4f', fontSize: 12 }}>{t('reject')}</a>
                      </Popconfirm>
                    </Space>
                  </Space>
                </div>
              ))
            )}
          </Card>
        </Col>

        {/* ── 最近构建 ── */}
        <Col xs={24} xl={14}>
          <Card size="small" title={t('recentBuilds')} extra={<CheckCircleOutlined />}>
            {buildList.length === 0 ? (
              <Typography.Text type="secondary">{t('noData')}</Typography.Text>
            ) : (
              buildList.slice(0, 6).map((b) => (
                <div key={b.id} style={{ padding: '6px 0', borderBottom: '1px solid #f0f0f0' }}>
                  <Space size={8}>
                    <Tag color={statusColor(b.status)}>#{b.buildNo}</Tag>
                    <Tag color={statusColor(b.status)}>{t(`status.${b.status}`, b.status)}</Tag>
                    <span>{b.branch}</span>
                    {b.artifactObjectKey && b.status === 'SUCCESS' && (
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        {Math.ceil((b.artifactSize ?? 0) / 1024)} KB
                      </Typography.Text>
                    )}
                    <Typography.Text type="secondary" style={{ fontSize: 12, marginLeft: 'auto' }}>
                      {fmtTime(b.createdAt)}
                    </Typography.Text>
                  </Space>
                </div>
              ))
            )}
          </Card>
        </Col>

        {/* ── 活跃告警 ── */}
        <Col xs={24} xl={10}>
          <Card
            size="small"
            title={
              <Space>
                {t('alertsPanel')}
                {activeAlerts.length > 0 && <Tag color="error">{activeAlerts.length}</Tag>}
              </Space>
            }
            extra={<BellOutlined />}
          >
            {activeAlerts.length === 0 ? (
              <Typography.Text type="secondary">{t('noAlerts')}</Typography.Text>
            ) : (
              activeAlerts.slice(0, 6).map((a) => (
                <div key={a.id} style={{ padding: '6px 0', borderBottom: '1px solid #f0f0f0' }}>
                  <Space size={6} wrap>
                    <BellOutlined style={{ color: '#ff4d4f' }} />
                    <span>{a.message}</span>
                    <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                      {fmtTime(a.createdAt)}
                    </Typography.Text>
                    <Popconfirm title={t('resolveConfirm')} onConfirm={() => a.id && resolveMutation.mutate({ id: a.id })}>
                      <a style={{ fontSize: 12, marginLeft: 'auto' }}>{t('resolve')}</a>
                    </Popconfirm>
                  </Space>
                </div>
              ))
            )}
          </Card>
        </Col>
      </Row>
    </ContentContainer>
  );
}
