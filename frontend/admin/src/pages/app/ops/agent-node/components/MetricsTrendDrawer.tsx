import { useEffect, useState } from 'react';
import { Drawer, Spin, Tag, App } from 'antd';
import ReactECharts from 'echarts-for-react';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_AgentNode as AgentNode } from '@/api/generated/admin/service/v1';
import { apiClient } from '@/api/client';

interface MetricsDrawerProps {
  data?: AgentNode;
  onClose: () => void;
}

/**
 * 节点指标趋势抽屉:CPU/内存/磁盘时间序列图。
 * 数据来源:GET /admin/v1/agent-nodes/{id}/metrics?hours=1
 */
const MetricsTrendDrawer: React.FC<MetricsDrawerProps> = ({ data, onClose }) => {
  const { t } = useTranslation('agent-node');
  const { message } = App.useApp();
  const [points, setPoints] = useState<{time:string;cpu:number;mem:number;disk:number}[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!data?.id) return;
    setLoading(true);
    apiClient.agentNodeService.GetMetrics({ id: data.id, hours: 1 })
      .then((resp: any) => {
        setPoints((resp.points ?? []).map((p: any) => ({
          time: p.time, cpu: p.cpuPercent, mem: p.memoryUsedPercent, disk: p.diskUsedPercent,
        })));
      })
      .catch((e: Error) => message.error(e.message || 'Failed to load metrics'))
      .finally(() => setLoading(false));
  }, [data?.id]);

  const times = points.map((p) => p.time.slice(11, 19));
  const chartOption = {
    tooltip: { trigger: 'axis' as const },
    legend: { data: ['CPU %', 'MEM %', 'DISK %'], bottom: 0 },
    grid: { left: 50, right: 20, top: 20, bottom: 60 },
    xAxis: { type: 'category' as const, data: times },
    yAxis: { type: 'value' as const, max: 100, axisLabel: { formatter: '{value}%' } },
    series: [
      { name: 'CPU %', type: 'line', data: points.map((p) => p.cpu), smooth: true, showSymbol: false, lineStyle: { width: 2 }, areaStyle: { opacity: 0.1 } },
      { name: 'MEM %', type: 'line', data: points.map((p) => p.mem), smooth: true, showSymbol: false, lineStyle: { width: 2 }, areaStyle: { opacity: 0.1 } },
      { name: 'DISK %', type: 'line', data: points.map((p) => p.disk), smooth: true, showSymbol: false, lineStyle: { width: 2, type: 'dashed' as const } },
    ],
    colors: ['#5b8ff9', '#5ad8a6', '#f6bd16'],
  };

  return (
    <Drawer
      title={<span>{t('metricsTrend')} <Tag style={{ marginLeft: 8 }}>{data?.name}</Tag></span>}
      open={!!data}
      onClose={onClose}
      size={720}
      destroyOnClose
    >
      {loading ? (
        <Spin style={{ display: 'block', margin: '60px auto' }} />
      ) : points.length === 0 ? (
        <div style={{ textAlign: 'center', padding: 40, color: '#999' }}>
          {t('metricsNoData')}
        </div>
      ) : (
        <ReactECharts option={chartOption} style={{ height: 400 }} notMerge />
      )}
    </Drawer>
  );
};

export default MetricsTrendDrawer;
