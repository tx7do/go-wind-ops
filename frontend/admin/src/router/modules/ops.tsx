import type { AppRouteObject } from '@/core/router/types';
import { createLazyRoute } from '@/core/router';

/**
 * 运维域路由配置
 * 节点管理、项目管理、脚本、构建与分发、指令台账
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
      {
        name: 'projects',
        path: 'projects',
        element: createLazyRoute(() => import('@/pages/app/ops/project')),
        meta: {
          title: 'routes:projects',
          icon: 'lucide:folder-open',
          order: 2,
        },
      },
      {
        name: 'scripts',
        path: 'scripts',
        element: createLazyRoute(() => import('@/pages/app/ops/script')),
        meta: {
          title: 'routes:scripts',
          icon: 'lucide:file-code',
          order: 3,
        },
      },
      {
        name: 'repositories',
        path: 'repositories',
        element: createLazyRoute(() => import('@/pages/app/ops/repository')),
        meta: {
          title: 'routes:repositories',
          icon: 'lucide:git-branch',
          order: 4,
        },
      },
      {
        name: 'builds',
        path: 'builds',
        element: createLazyRoute(() => import('@/pages/app/ops/build')),
        meta: {
          title: 'routes:builds',
          icon: 'lucide:hammer',
          order: 5,
        },
      },
      {
        name: 'distributions',
        path: 'distributions',
        element: createLazyRoute(() => import('@/pages/app/ops/distribution')),
        meta: {
          title: 'routes:distributions',
          icon: 'lucide:send',
          order: 6,
        },
      },
      {
        name: 'command-records',
        path: 'command-records',
        element: createLazyRoute(() => import('@/pages/app/ops/command-record')),
        meta: {
          title: 'routes:command-records',
          icon: 'lucide:scroll-text',
          order: 7,
        },
      },
      {
        name: 'monitors',
        path: 'monitors',
        element: createLazyRoute(() => import('@/pages/app/ops/monitor')),
        meta: {
          title: 'routes:monitors',
          icon: 'lucide:activity',
          order: 8,
        },
      },
      {
        name: 'alerts',
        path: 'alerts',
        element: createLazyRoute(() => import('@/pages/app/ops/alert')),
        meta: {
          title: 'routes:alerts',
          icon: 'lucide:bell',
          order: 9,
        },
      },
    ],
  },
];

export default opsRoutes;
