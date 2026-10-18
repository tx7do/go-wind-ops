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
          authority: ['ops:node_manage'],
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
          authority: ['ops:project_manage'],
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
          authority: ['ops:script_manage'],
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
          authority: ['ops:build_manage'],
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
          authority: ['ops:build_manage'],
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
          authority: ['ops:distribute'],
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
          authority: ['ops:node_manage'],
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
          authority: ['sys:platform_admin'],
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
          authority: ['sys:platform_admin'],
        },
      },
      {
        name: 'notification-channels',
        path: 'notification-channels',
        element: createLazyRoute(() => import('@/pages/app/ops/notification-channel')),
        meta: {
          title: 'routes:notification-channels',
          icon: 'lucide:radio',
          order: 10,
          authority: ['sys:platform_admin'],
        },
      },
      {
        name: 'approvals',
        path: 'approvals',
        element: createLazyRoute(() => import('@/pages/app/ops/approval')),
        meta: {
          title: 'routes:approvals',
          icon: 'lucide:check-circle',
          order: 11,
          authority: ['sys:platform_admin'],
        },
      },
    ],
  },
];

export default opsRoutes;
