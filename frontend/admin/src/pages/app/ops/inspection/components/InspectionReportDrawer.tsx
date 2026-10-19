import { Descriptions, Drawer, Spin, Table, Tag } from 'antd';
import { useTranslation } from 'react-i18next';
import { useGetInspection } from '@/api/hooks/inspection';

interface Props {
  inspectionId?: number;
  onClose: () => void;
}

interface ReportNode {
  name: string;
  agentId: string;
  status: string;
  env: string;
  cpu: number;
  mem: number;
  disk: number;
  version: string;
}

interface ReportProject {
  name: string;
  nodeId: number;
  status: string;
}

interface Report {
  generatedAt: string;
  nodes?: ReportNode[];
  projects?: ReportProject[];
  alerts?: { open: number };
  certExpiring?: { name: string; host: string; expires: string }[];
}

/**
 * 巡检报告抽屉:概览统计 + 节点明细 + 项目明细 + 到期证书列表。
 */
const InspectionReportDrawer = ({ inspectionId, onClose }: Props) => {
  const { t } = useTranslation('inspection');
  const { data, isFetching } = useGetInspection(inspectionId);

  let report: Report | undefined;
  try {
    if (data?.summary) report = JSON.parse(data.summary) as Report;
  } catch {
    report = undefined;
  }

  return (
    <Drawer
      title={t('reportTitle', { id: inspectionId })}
      open={!!inspectionId}
      onClose={onClose}
      size={900}
      destroyOnHidden
    >
      {isFetching ? (
        <div style={{ textAlign: 'center', padding: 48 }}><Spin /></div>
      ) : (
        <>
          <Descriptions size="small" column={4} bordered style={{ marginBottom: 16 }}>
            <Descriptions.Item label={t('nodes')}>
              <Tag color="blue">{`${data?.nodeOnline ?? 0}/${data?.nodeTotal ?? 0}`}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label={t('projects')}>
              <Tag color="green">{`${data?.projectRunning ?? 0}/${data?.projectTotal ?? 0}`}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label={t('alertOpen')}>
              <Tag color={(data?.alertOpen ?? 0) > 0 ? 'warning' : 'default'}>{data?.alertOpen ?? 0}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label={t('certExpiring')}>
              <Tag color={(data?.certExpiring ?? 0) > 0 ? 'error' : 'default'}>{data?.certExpiring ?? 0}</Tag>
            </Descriptions.Item>
          </Descriptions>

          {report && (
            <>
              <div style={{ fontWeight: 600, margin: '12px 0 8px' }}>{t('nodeDetail')}</div>
              <Table<ReportNode>
                size="small"
                rowKey="agentId"
                dataSource={report.nodes || []}
                pagination={false}
                columns={[
                  { title: t('nodeName'), dataIndex: 'name', width: 140 },
                  { title: t('status'), dataIndex: 'status', width: 90, render: (v: string) => (v === 'ONLINE' ? <Tag color="success">{v}</Tag> : <Tag>{v}</Tag>) },
                  { title: t('env'), dataIndex: 'env', width: 90 },
                  { title: 'CPU', dataIndex: 'cpu', width: 90, render: (v: number) => `${v.toFixed(1)}%` },
                  { title: t('mem'), dataIndex: 'mem', width: 90, render: (v: number) => `${v.toFixed(1)}%` },
                  { title: t('disk'), dataIndex: 'disk', width: 90, render: (v: number) => `${v.toFixed(1)}%` },
                  { title: t('version'), dataIndex: 'version', width: 120, render: (v: string) => v || '-' },
                ]}
              />

              <div style={{ fontWeight: 600, margin: '16px 0 8px' }}>{t('projectDetail')}</div>
              <Table<ReportProject>
                size="small"
                rowKey={(r) => `${r.name}-${r.nodeId}`}
                dataSource={report.projects || []}
                pagination={false}
                columns={[
                  { title: t('projectName'), dataIndex: 'name', width: 180 },
                  { title: t('nodeId'), dataIndex: 'nodeId', width: 90 },
                  { title: t('status'), dataIndex: 'status', width: 110, render: (v: string) => (v === 'RUNNING' ? <Tag color="success">{v}</Tag> : <Tag>{v || 'UNKNOWN'}</Tag>) },
                ]}
              />

              <div style={{ fontWeight: 600, margin: '16px 0 8px' }}>{t('certDetail')}</div>
              <Table
                size="small"
                rowKey="name"
                dataSource={report.certExpiring || []}
                pagination={false}
                locale={{ emptyText: t('noCertExpiring') }}
                columns={[
                  { title: t('certName'), dataIndex: 'name', width: 160 },
                  { title: t('certHost'), dataIndex: 'host', width: 200 },
                  { title: t('certExpires'), dataIndex: 'expires', width: 180, render: (v: string) => v?.replace('T', ' ').slice(0, 19) },
                ]}
              />
            </>
          )}
        </>
      )}
    </Drawer>
  );
};

export default InspectionReportDrawer;
