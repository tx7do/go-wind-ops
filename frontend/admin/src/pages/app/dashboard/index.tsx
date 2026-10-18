import { Card, Col, Row, Typography } from 'antd';
import { useI18n } from '@/core/i18n';
import ContentContainer from '@/layouts/components/PageContainer/ContentContainer';
import { useUserStore } from '@/stores/user';

/**
 * 仪表盘(占位版):原 CMS 统计域已随业务裁剪移除,
 * 后续以 ops 概览(节点在线/项目运行/构建分发)重建数据面板。
 */
export default function DashboardPage() {
  const { t } = useI18n('common');
  const userInfo = useUserStore((state) => state.userInfo);

  return (
    <ContentContainer>
      <Row gutter={[16, 16]}>
        <Col span={24}>
          <Card>
            <Typography.Title level={4}>
              {userInfo?.nickname || userInfo?.username || t('welcome')}
            </Typography.Title>
            <Typography.Text type="secondary">
              GoWind Ops · 多节点运维平台
            </Typography.Text>
          </Card>
        </Col>
      </Row>
    </ContentContainer>
  );
}
