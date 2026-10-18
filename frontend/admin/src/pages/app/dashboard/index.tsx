import { useEffect, useState } from 'react';
import { Card, Col, Row, Spin, Statistic, Tag, Typography } from 'antd';
import {
  CheckCircleOutlined,
  CloudServerOutlined,
  RocketOutlined,
  WarningOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { PaginationQuery } from '@/core';
import ContentContainer from '@/layouts/components/PageContainer/ContentContainer';
import { fetchListAgentNodes } from '@/api/hooks/agent-node';
import { fetchListProjects } from '@/api/hooks/project';
import { fetchListApprovals } from '@/api/hooks/approval';
import { fetchListBuildHistories } from '@/api/hooks/build-history';
import { useUserStore } from '@/stores/user';

interface StatCard {
  title: string;
  value: number;
  total: number;
  color: string;
  icon: React.ReactNode;
}

/**
 * 运维大盘:节点/项目/构建/告警 概览统计 + 最近事件。
 */
export default function DashboardPage() {
  const { t } = useTranslation('dashboard');
  const userInfo = useUserStore((state) => state.userInfo);

  const [loading, setLoading] = useState(true);
  const [nodes, setNodes] = useState<any[]>([]);
  const [projects, setProjects] = useState<any[]>([]);
  const [builds, setBuilds] = useState<any[]>([]);
  const [approvals, setApprovals] = useState<any[]>([]);

  useEffect(() => {
    const pq = () => new PaginationQuery({ paging: { page: 1, pageSize: 200 } });
    Promise.allSettled([
      fetchListAgentNodes(pq()),
      fetchListProjects(pq()),
      fetchListBuildHistories(pq()),
      fetchListApprovals(pq()),
    ]).then(([n, p, b, a]) => {
      if (n.status === 'fulfilled') setNodes(n.value?.items ?? []);
      if (p.status === 'fulfilled') setProjects(p.value?.items ?? []);
      if (b.status === 'fulfilled') setBuilds(b.value?.items ?? []);
      if (a.status === 'fulfilled') setApprovals(a.value?.items ?? []);
      setLoading(false);
    });
  }, []);

  const onlineNodes = nodes.filter((n) => n.status === 'ONLINE').length;
  const runningProjects = projects.filter((p) => p.lastStatus === 'RUNNING').length;
  const pendingApprovals = approvals.filter((a) => a.status === 'PENDING').length;
  const recentBuilds = builds.slice(0, 6);
  const pendingApprovalList = approvals.filter((a) => a.status === 'PENDING').slice(0, 5);

  const stats: StatCard[] = [
    {
      title: t('statNodes'),
      value: onlineNodes,
      total: nodes.length,
      color: '#52c41a',
      icon: <CloudServerOutlined />,
    },
    {
      title: t('statProjectsRunning'),
      value: runningProjects,
      total: projects.length,
      color: '#1890ff',
      icon: <RocketOutlined />,
    },
    {
      title: t('statBuildSuccess'),
      value: builds.filter((b) => b.status === 'SUCCESS').length,
      total: builds.length,
      color: '#722ed1',
      icon: <CheckCircleOutlined />,
    },
    {
      title: t('statPendingApprovals'),
      value: pendingApprovals,
      total: approvals.length,
      color: pendingApprovals > 0 ? '#faad14' : '#999',
      icon: <WarningOutlined />,
    },
  ];

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
        {/* ── 欢迎语 ── */}
        <Col span={24}>
          <Typography.Title level={4} style={{ marginBottom: 0 }}>
            {userInfo?.nickname || userInfo?.username || 'GoWind Ops'}
          </Typography.Title>
          <Typography.Text type="secondary">GoWind Ops · 多节点运维平台</Typography.Text>
        </Col>

        {/* ── 统计卡片 ── */}
        {stats.map((s) => (
          <Col xs={24} sm={12} lg={6} key={s.title}>
            <Card size="small">
              <Statistic
                title={<span>{s.icon} {s.title}</span>}
                value={s.value}
                suffix={s.total > 0 ? `/ ${s.total}` : undefined}
                valueStyle={{ color: s.color }}
              />
            </Card>
          </Col>
        ))}

        {/* ── 待审批 ── */}
        {pendingApprovalList.length > 0 && (
          <Col span={24}>
            <Card size="small" title={t('pendingApprovalsTitle')}>
              {pendingApprovalList.map((a) => (
                <div key={a.id} style={{ padding: '6px 0', borderBottom: '1px solid #f0f0f0' }}>
                  <Tag color="warning">{a.operationType}</Tag>
                  <span>{a.operationName || `#${a.operationRefId}`}</span>
                  <span style={{ float: 'right', color: '#999', fontSize: 12 }}>
                    {(a.createdAt ?? '').replace('T', ' ').slice(0, 19)}
                  </span>
                </div>
              ))}
            </Card>
          </Col>
        )}

        {/* ── 最近构建 ── */}
        <Col xs={24} lg={12}>
          <Card size="small" title={t('recentBuildsTitle')}>
            {recentBuilds.length === 0 ? (
              <Typography.Text type="secondary">{t('noData')}</Typography.Text>
            ) : (
              recentBuilds.map((b) => (
                <div key={b.id} style={{ padding: '6px 0', borderBottom: '1px solid #f0f0f0' }}>
                  <Tag color={b.status === 'SUCCESS' ? 'success' : b.status === 'FAILED' ? 'error' : 'processing'}>
                    #{b.buildNo}
                  </Tag>
                  <span>{b.branch}</span>
                  <span style={{ float: 'right', color: '#999', fontSize: 12 }}>
                    {(b.createdAt ?? '').replace('T', ' ').slice(0, 19)}
                  </span>
                </div>
              ))
            )}
          </Card>
        </Col>

        {/* ── 节点状态 ── */}
        <Col xs={24} lg={12}>
          <Card size="small" title={t('nodeStatusTitle')}>
            {nodes.length === 0 ? (
              <Typography.Text type="secondary">{t('noNodes')}</Typography.Text>
            ) : (
              nodes.map((n) => (
                <div key={n.id} style={{ padding: '6px 0', borderBottom: '1px solid #f0f0f0' }}>
                  <Tag color={n.status === 'ONLINE' ? 'success' : 'default'}>{n.status}</Tag>
                  <span style={{ fontWeight: 500 }}>{n.name || n.hostname}</span>
                  <span style={{ float: 'right', fontSize: 12, color: '#999' }}>
                    CPU {Math.round(n.cpuPercent ?? 0)}% · MEM {Math.round(n.memoryUsedPercent ?? 0)}%
                  </span>
                </div>
              ))
            )}
          </Card>
        </Col>
      </Row>
    </ContentContainer>
  );
}
