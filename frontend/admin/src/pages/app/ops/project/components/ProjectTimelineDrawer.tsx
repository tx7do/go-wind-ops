import { useEffect, useState } from 'react';
import { Drawer, Timeline, Tag, Spin, Empty } from 'antd';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_Alert as Alert } from '@/api/generated/admin/service/v1';
import type { opsservicev1_DistributionRecord as DistributionRecord } from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { fetchListAlerts } from '@/api/hooks/alert';
import { fetchListDistributionRecords } from '@/api/hooks/distribution';

interface Props {
  projectId?: number;
  projectName?: string;
  onClose: () => void;
}

interface TimelineEntry {
  time: number;
  createdAt: string;
  kind: 'distribution' | 'alert';
  record: DistributionRecord | Alert;
}

const TIMELINE_LIMIT = 50;

/**
 * 项目变更时间线:聚合分发记录与告警,按时间倒序合并展示
 * (谁在什么时候发了什么版、出过什么告警)。
 */
const ProjectTimelineDrawer = ({ projectId, projectName, onClose }: Props) => {
  const { t } = useTranslation('project');
  const [loading, setLoading] = useState(false);
  const [entries, setEntries] = useState<TimelineEntry[]>([]);

  useEffect(() => {
    if (!projectId) return;
    setLoading(true);
    setEntries([]);
    const q = new PaginationQuery({
      paging: { page: 1, pageSize: TIMELINE_LIMIT },
      formValues: { projectId },
      orderBy: ['-createdAt'],
    });
    Promise.all([fetchListDistributionRecords(q), fetchListAlerts(q)])
      .then(([distResp, alertResp]) => {
        const merged: TimelineEntry[] = [
          ...(distResp.items ?? []).map((r: DistributionRecord) => ({
            time: new Date(r.createdAt ?? 0).getTime(),
            createdAt: r.createdAt ?? '',
            kind: 'distribution' as const,
            record: r,
          })),
          ...(alertResp.items ?? []).map((a: Alert) => ({
            time: new Date(a.createdAt ?? 0).getTime(),
            createdAt: a.createdAt ?? '',
            kind: 'alert' as const,
            record: a,
          })),
        ];
        merged.sort((x, y) => y.time - x.time);
        setEntries(merged.slice(0, TIMELINE_LIMIT));
      })
      .catch(() => setEntries([]))
      .finally(() => setLoading(false));
  }, [projectId]);

  const renderEntry = (e: TimelineEntry) => {
    if (e.kind === 'distribution') {
      const r = e.record as DistributionRecord;
      const color = r.status === 'SUCCESS' ? 'green' : r.status === 'FAILED' ? 'red' : 'blue';
      return {
        color,
        children: (
          <>
            <div>
              <Tag>{t('timelineDistribution')}</Tag>
              <span style={{ fontSize: 12, color: 'var(--ant-color-text-secondary)' }}>{e.createdAt}</span>
            </div>
            <div style={{ fontSize: 13 }}>
              {t('timelineDistTarget')} #{r.projectId} · {r.status}
              {r.error ? <span style={{ color: 'var(--ant-color-error)' }}> · {r.error}</span> : null}
            </div>
          </>
        ),
      };
    }
    const a = e.record as Alert;
    const color = a.level === 'ERROR' ? 'red' : a.level === 'RECOVERY' ? 'green' : 'orange';
    return {
      color,
      children: (
        <>
          <div>
            <Tag>{t('timelineAlert')}</Tag>
            <span style={{ fontSize: 12, color: 'var(--ant-color-text-secondary)' }}>{e.createdAt}</span>
          </div>
          <div style={{ fontSize: 13 }}>
            [{a.level}] {a.message}
            {a.resolved ? ` · ${t('timelineResolved')}` : ''}
          </div>
        </>
      ),
    };
  };

  return (
    <Drawer
      title={t('timelineTitle', { name: projectName })}
      open={!!projectId}
      onClose={onClose}
      size={560}
      destroyOnHidden
    >
      {loading ? (
        <div style={{ textAlign: 'center', padding: 48 }}><Spin /></div>
      ) : entries.length === 0 ? (
        <Empty description={t('timelineEmpty')} />
      ) : (
        <Timeline items={entries.map(renderEntry)} />
      )}
    </Drawer>
  );
};

export default ProjectTimelineDrawer;
