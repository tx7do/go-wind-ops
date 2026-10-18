import type { AppRouteObject } from '@/core/router/types';
import { createLazyRoute } from '@/core/router';

/**
 * 运维域路由配置
 * 节点管理、项目管理、脚本、构建分发等运维业务页面
 */
export const opsRoutes: AppRouteObject[] = [
  {
    name: 'ops',
    path: 'ops',
    meta: {
      title: 'routes:ops',
      icon: 'lucide:server',
      order: 2004,
      keepAlive: true,
    },
    children: [
      {
        name: 'agent-nodes',
        path: 'agent-nodes',
        element: createLazyRoute(() => import('@/pages/app/ops/agent-node')),
        meta: {
          title: 'routes:agent-nodes',
          icon: 'lucide:monitor-dot',
          order: 1,
        },
      },
    ],
  },
];

export default opsRoutes;
