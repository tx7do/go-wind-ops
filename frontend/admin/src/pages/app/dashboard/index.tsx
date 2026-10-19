import { Card, Col, Empty, Popconfirm, Progress, Row, Space, Tag, Typography } from 'antd';
import {
  BellOutlined,
  CheckCircleOutlined,
  CloudServerOutlined,
  ReloadOutlined,
  RocketOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import { useQueryClient } from '@tanstack/react-query';
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
  s === 'SUCCESS' ? '#52c41a' : s === 'FAILED' ? '#ff4d4f' : s === 'RUNNING' || s === 'PENDING' ? '#1677ff' : '#8c8c8c';

const fmtTime = (v?: string) => (v ? String(v).replace('T', ' ').slice(0, 19) : '-');

/** 相对时间:刚刚/N 分钟前/N 小时前,更早退化绝对日期。 */
function relTime(v?: string): string {
  if (!v) return '-';
  const t = new Date(v).getTime();
  if (Number.isNaN(t)) return fmtTime(v);
  const diff = Date.now() - t;
  const m = Math.floor(diff / 60_000);
  if (m < 1) return '-';
  if (m < 60) return `${m} 分钟前`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h} 小时前`;
  return String(v).slice(0, 10);
}

const percentColor = (p: number) => (p >= 90 ? '#ff4d4f' : p >= 70 ? '#faad14' : '#52c41a');

/** 软色块图标芯片。 */
function IconChip({ color, children }: { color: string; children: React.ReactNode }) {
  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
        width: 44,
        height: 44,
        borderRadius: 12,
        background: `${color}1f`,
        color,
        fontSize: 20,
        flexShrink: 0,
      }}
    >
      {children}
    </span>
  );
}

/** 卡片小节标题:竖条强调 + 文案。 */
function PanelTitle({ color, text }: { color: string; text: string }) {
  return (
    <Space size={8}>
      <span style={{ display: 'inline-block', width: 3, height: 14, borderRadius: 2, background: color }} />
      <span style={{ fontWeight: 600 }}>{text}</span>
    </Space>
  );
}

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
  const builds = useListBuildHistories(dashQuery(), { ...common, refetchInterval: 15000 } as never);
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
    { key: 'onlineNodes', value: onlineNodes.length, suffix: `/ ${nodeList.length}`, color: '#52c41a', icon: <CloudServerOutlined /> },
    { key: 'runningProjects', value: runningProjects.length, suffix: `/ ${projectList.length}`, color: '#1677ff', icon: <RocketOutlined /> },
    { key: 'buildSuccess', value: successBuilds.length, suffix: `/ ${buildList.length}`, color: '#722ed1', icon: <CheckCircleOutlined /> },
    { key: 'pendingApprovals', value: pendingApprovals.length, color: pendingApprovals.length > 0 ? '#faad14' : '#8c8c8c', icon: <ThunderboltOutlined /> },
    { key: 'activeAlerts', value: activeAlerts.length, color: activeAlerts.length > 0 ? '#ff4d4f' : '#8c8c8c', icon: <BellOutlined /> },
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
        <Card loading style={{ minHeight: 400 }} />
      </ContentContainer>
    );
  }

  const nodeCard = (n: any) => {
    const cpu = Math.round(n.cpuPercent ?? 0);
    const mem = Math.round(n.memoryUsedPercent ?? 0);
    const disk = Math.round(n.diskUsedPercent ?? 0);
    return (
      <div
        key={n.id}
        style={{
          border: '1px solid var(--ant-color-border-secondary)',
          borderRadius: 12,
          padding: '10px 14px 4px',
          flex: '1 1 300px',
          minWidth: 280,
        }}
      >
        <Space size={6} style={{ marginBottom: 6, width: '100%' }}>
          <span
            style={{
              width: 8,
              height: 8,
              borderRadius: '50%',
              background: n.status === 'ONLINE' ? '#52c41a' : '#8c8c8c',
              boxShadow: n.status === 'ONLINE' ? '0 0 6px #52c41a88' : undefined,
              flexShrink: 0,
            }}
          />
          <span style={{ fontWeight: 600 }}>{n.name || n.hostname}</span>
          {n.env ? <Tag style={{ margin: 0 }}>{n.env}</Tag> : null}
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {n.platform}
          </Typography.Text>
          <Typography.Text type="secondary" style={{ fontSize: 12, marginLeft: 'auto' }}>
            {n.status === 'ONLINE'
              ? relTime(n.lastHeartbeatAt) === '-'
                ? t('online')
                : t('heartbeat', { time: relTime(n.lastHeartbeatAt) })
              : t('offline')}
          </Typography.Text>
        </Space>
        <Row gutter={8}>
          {[
            ['cpu', cpu],
            ['mem', mem],
            ['disk', disk],
          ].map(([label, v]) => (
            <Col span={8} key={label as string} style={{ textAlign: 'center' }}>
              <Progress
                type="dashboard"
                percent={v as number}
                size={80}
                strokeColor={percentColor(v as number)}
                format={(p) => (
                  <span style={{ fontSize: 14, fontWeight: 600, color: percentColor(v as number) }}>{p}%</span>
                )}
              />
              <div style={{ fontSize: 12, color: '#8c8c8c', marginTop: 2 }}>{t(label as string)}</div>
            </Col>
          ))}
        </Row>
      </div>
    );
  };

  const rowShell = { padding: '8px 10px', marginBottom: 4 };

  return (
    <ContentContainer>
      <style>{`
        .dash-row { transition: background .2s; border-radius: 8px; }
        .dash-row:hover { background: var(--ant-color-fill-tertiary); }
      `}</style>
      <Row gutter={[16, 16]}>
        {/* ── 问候横幅 ── */}
        <Col span={24}>
          <div
            style={{
              borderRadius: 14,
              padding: '18px 22px',
              background:
                'linear-gradient(120deg, rgba(22,119,255,.16) 0%, rgba(114,46,209,.14) 55%, rgba(82,196,26,.10) 100%)',
            }}
          >
            <Space size={12} align="center" style={{ width: '100%', justifyContent: 'space-between' }}>
              <Space size={12} align="baseline">
                <Typography.Title level={4} style={{ marginBottom: 0 }}>
                  {t('greeting')}
                  {userInfo?.nickname || userInfo?.username || 'GoWind Ops'}
                </Typography.Title>
                <Typography.Text type="secondary">{today}</Typography.Text>
              </Space>
              <Space size={6}>
                <ReloadOutlined style={{ color: 'var(--ant-color-text-tertiary)', fontSize: 12 }} />
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                  {t('autoRefresh')}
                </Typography.Text>
              </Space>
            </Space>
          </div>
        </Col>

        {/* ── 统计瓷砖(flex 均分撑满一行,窄屏自动换行) ── */}
        <Col span={24}>
          <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap' }}>
            {stats.map((s) => (
              <div key={s.key} style={{ flex: '1 1 190px', minWidth: 190 }}>
                <Card size="small" hoverable styles={{ body: { padding: '16px 18px' } }}>
                  <Space size={14} align="center">
                    <IconChip color={s.color}>{s.icon}</IconChip>
                    <div>
                      <Typography.Text type="secondary" style={{ fontSize: 13 }}>
                        {t(`stat.${s.key}`)}
                      </Typography.Text>
                      <div style={{ fontSize: 26, fontWeight: 700, lineHeight: 1.2, color: s.color }}>
                        {s.value}
                        {s.suffix ? (
                          <span style={{ fontSize: 13, fontWeight: 400, color: '#8c8c8c', marginLeft: 6 }}>{s.suffix}</span>
                        ) : null}
                      </div>
                    </div>
                  </Space>
                </Card>
              </div>
            ))}
          </div>
        </Col>

        {/* ── 待办审批 ── */}
        <Col xs={24} xl={12}>
          <Card
            size="small"
            title={<PanelTitle color="#faad14" text={t('pendingPanel')} />}
            extra={
              pendingApprovals.length > 0 ? (
                <Tag color="warning" style={{ margin: 0 }}>{pendingApprovals.length}</Tag>
              ) : (
                <ThunderboltOutlined style={{ color: '#8c8c8c' }} />
              )
            }
          >
            {pendingApprovals.length === 0 ? (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('noPending')} style={{ margin: '12px 0' }} />
            ) : (
              pendingApprovals.slice(0, 6).map((a) => (
                <div key={a.id} className="dash-row" style={rowShell}>
                  <Space size={8} wrap style={{ width: '100%' }}>
                    <Tag color="warning" style={{ margin: 0 }}>{a.operationType}</Tag>
                    <span>{a.operationName || `#${a.operationRefId}`}</span>
                    <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                      {relTime(a.createdAt)}
                    </Typography.Text>
                    <Space size={10} style={{ marginLeft: 'auto' }}>
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

        {/* ── 活跃告警 ── */}
        <Col xs={24} xl={12}>
          <Card
            size="small"
            title={<PanelTitle color="#ff4d4f" text={t('alertsPanel')} />}
            extra={
              activeAlerts.length > 0 ? (
                <Tag color="error" style={{ margin: 0 }}>{activeAlerts.length}</Tag>
              ) : (
                <BellOutlined style={{ color: '#8c8c8c' }} />
              )
            }
          >
            {activeAlerts.length === 0 ? (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('noAlerts')} style={{ margin: '12px 0' }} />
            ) : (
              activeAlerts.slice(0, 6).map((a) => (
                <div key={a.id} className="dash-row" style={rowShell}>
                  <Space size={8} wrap style={{ width: '100%' }}>
                    <BellOutlined style={{ color: '#ff4d4f' }} />
                    <span>{a.message}</span>
                    <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                      {relTime(a.createdAt)}
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

        {/* ── 最近构建 ── */}
        <Col xs={24} xl={12}>
          <Card
            size="small"
            title={<PanelTitle color="#722ed1" text={t('recentBuilds')} />}
            extra={<CheckCircleOutlined style={{ color: '#8c8c8c' }} />}
          >
            {buildList.length === 0 ? (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('noData')} style={{ margin: '12px 0' }} />
            ) : (
              buildList.slice(0, 6).map((b) => (
                <div key={b.id} className="dash-row" style={rowShell}>
                  <Space size={10} wrap style={{ width: '100%' }}>
                    <span
                      style={{ width: 8, height: 8, borderRadius: '50%', background: statusColor(b.status), flexShrink: 0 }}
                    />
                    <span style={{ fontWeight: 600 }}>#{b.buildNo}</span>
                    <span>{b.branch}</span>
                    {b.status === 'SUCCESS' && b.artifactObjectKey && (
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        {Math.ceil((b.artifactSize ?? 0) / 1024)} KB
                      </Typography.Text>
                    )}
                    <Typography.Text type="secondary" style={{ fontSize: 12, marginLeft: 'auto' }}>
                      {relTime(b.createdAt)}
                    </Typography.Text>
                  </Space>
                </div>
              ))
            )}
          </Card>
        </Col>

        {/* ── 项目状态 ── */}
        <Col xs={24} xl={12}>
          <Card
            size="small"
            title={<PanelTitle color="#1677ff" text={t('projectsPanel')} />}
            extra={<RocketOutlined style={{ color: '#8c8c8c' }} />}
          >
            {projectList.length === 0 ? (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('noProjects')} style={{ margin: '12px 0' }} />
            ) : (
              projectList.slice(0, 6).map((pj) => (
                <div key={pj.id} className="dash-row" style={rowShell}>
                  <Space size={10} wrap style={{ width: '100%' }}>
                    <span
                      style={{
                        width: 8,
                        height: 8,
                        borderRadius: '50%',
                        background: pj.lastStatus === 'RUNNING' ? '#52c41a' : '#8c8c8c',
                        flexShrink: 0,
                      }}
                    />
                    <span style={{ fontWeight: 600 }}>{pj.name}</span>
                    <Tag color={pj.lastStatus === 'RUNNING' ? 'processing' : 'default'} style={{ margin: 0 }}>
                      {(t as (k: string, d?: string) => string)(`projStatus.${pj.lastStatus}`, pj.lastStatus)}
                    </Tag>
                    <Typography.Text type="secondary" style={{ fontSize: 12, marginLeft: 'auto' }}>
                      {relTime(pj.lastStatusAt)}
                    </Typography.Text>
                  </Space>
                </div>
              ))
            )}
          </Card>
        </Col>

        {/* ── 节点资源 ── */}
        <Col span={24}>
          <Card
            size="small"
            title={<PanelTitle color="#52c41a" text={t('nodesPanel')} />}
            extra={<CloudServerOutlined style={{ color: '#8c8c8c' }} />}
          >
            {nodeList.length === 0 ? (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('noData')} style={{ margin: '12px 0' }} />
            ) : (
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 10 }}>
                {nodeList.map(nodeCard)}
              </div>
            )}
          </Card>
        </Col>

      </Row>
    </ContentContainer>
  );
}
