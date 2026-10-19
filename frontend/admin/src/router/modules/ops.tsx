import type { AppRouteObject } from '@/core/router/types';
import { createLazyRoute } from '@/core/router';

/**
 * 运维域路由:六个功能组直接作为一级菜单(资源管理/持续交付/自动化/可观测/云管理/治理)。
 * 分组节点不带 path,叶子使用绝对路径 /ops/<page>,URL 与拆分前保持一致。
 */
export const opsRoutes: AppRouteObject[] = [
  // ── 资源管理 ──
  {
    name: 'ops-resource',
    meta: {
      title: 'routes:opsResource',
      icon: 'lucide:hard-drive',
      order: 1,
      authority: ['ops:node_manage', 'ops:project_manage'],
    },
    children: [
      {
        name: 'agent-nodes',
        path: '/ops/agent-nodes',
        element: createLazyRoute(() => import('@/pages/app/ops/agent-node')),
        meta: {
          title: 'routes:agent-nodes',
          icon: 'lucide:monitor-dot',
          order: 1,
          authority: ['ops:node_manage'],
        },
      },
      {
        name: 'terminal',
        path: '/ops/terminal',
        element: createLazyRoute(() => import('@/pages/app/ops/terminal')),
        meta: {
          title: 'routes:terminal',
          icon: 'lucide:terminal',
          order: 2,
          authority: ['ops:node_manage'],
        },
      },
      {
        name: 'file-manager',
        path: '/ops/file-manager',
        element: createLazyRoute(() => import('@/pages/app/ops/file-manager')),
        meta: {
          title: 'routes:file-manager',
          icon: 'lucide:folder-tree',
          order: 3,
          authority: ['ops:node_manage'],
        },
      },
      {
        name: 'projects',
        path: '/ops/projects',
        element: createLazyRoute(() => import('@/pages/app/ops/project')),
        meta: {
          title: 'routes:projects',
          icon: 'lucide:folder-open',
          order: 4,
          authority: ['ops:project_manage'],
        },
      },
    ],
  },

  // ── 持续交付 ──
  {
    name: 'ops-cicd',
    meta: {
      title: 'routes:opsCicd',
      icon: 'lucide:package',
      order: 3,
      authority: ['ops:build_manage', 'ops:distribute'],
    },
    children: [
      {
        name: 'repositories',
        path: '/ops/repositories',
        element: createLazyRoute(() => import('@/pages/app/ops/repository')),
        meta: {
          title: 'routes:repositories',
          icon: 'lucide:git-branch',
          order: 1,
          authority: ['ops:build_manage'],
        },
      },
      {
        name: 'builds',
        path: '/ops/builds',
        element: createLazyRoute(() => import('@/pages/app/ops/build')),
        meta: {
          title: 'routes:builds',
          icon: 'lucide:hammer',
          order: 2,
          authority: ['ops:build_manage'],
        },
      },
      {
        name: 'pipelines',
        path: '/ops/pipelines',
        element: createLazyRoute(() => import('@/pages/app/ops/pipeline')),
        meta: {
          title: 'routes:pipelines',
          icon: 'lucide:workflow',
          order: 3,
          authority: ['ops:build_manage'],
        },
      },
      {
        name: 'distributions',
        path: '/ops/distributions',
        element: createLazyRoute(() => import('@/pages/app/ops/distribution')),
        meta: {
          title: 'routes:distributions',
          icon: 'lucide:send',
          order: 4,
          authority: ['ops:distribute'],
        },
      },
    ],
  },

  // ── 自动化 ──
  {
    name: 'ops-auto',
    meta: {
      title: 'routes:opsAuto',
      icon: 'lucide:bot',
      order: 4,
      authority: ['ops:script_manage', 'ops:node_manage'],
    },
    children: [
      {
        name: 'scripts',
        path: '/ops/scripts',
        element: createLazyRoute(() => import('@/pages/app/ops/script')),
        meta: {
          title: 'routes:scripts',
          icon: 'lucide:file-code',
          order: 1,
          authority: ['ops:script_manage'],
        },
      },
      {
        name: 'script-batches',
        path: '/ops/script-batches',
        element: createLazyRoute(() => import('@/pages/app/ops/script-batch')),
        meta: {
          title: 'routes:script-batches',
          icon: 'lucide:layers',
          order: 2,
          authority: ['ops:script_manage'],
        },
      },
      {
        name: 'backups',
        path: '/ops/backups',
        element: createLazyRoute(() => import('@/pages/app/ops/backup')),
        meta: {
          title: 'routes:backups',
          icon: 'lucide:archive',
          order: 3,
          authority: ['ops:script_manage'],
        },
      },
      {
        name: 'plugins',
        path: '/ops/plugins',
        element: createLazyRoute(() => import('@/pages/app/ops/plugin')),
        meta: {
          title: 'routes:plugins',
          icon: 'lucide:puzzle',
          order: 4,
          authority: ['ops:node_manage'],
        },
      },
    ],
  },

  // ── 可观测 ──
  {
    name: 'ops-observe',
    meta: {
      title: 'routes:opsObserve',
      icon: 'lucide:gauge',
      order: 5,
      authority: ['sys:platform_admin'],
    },
    children: [
      {
        name: 'monitors',
        path: '/ops/monitors',
        element: createLazyRoute(() => import('@/pages/app/ops/monitor')),
        meta: {
          title: 'routes:monitors',
          icon: 'lucide:activity',
          order: 1,
          authority: ['sys:platform_admin'],
        },
      },
      {
        name: 'alerts',
        path: '/ops/alerts',
        element: createLazyRoute(() => import('@/pages/app/ops/alert')),
        meta: {
          title: 'routes:alerts',
          icon: 'lucide:bell',
          order: 2,
          authority: ['sys:platform_admin'],
        },
      },
      {
        name: 'notification-channels',
        path: '/ops/notification-channels',
        element: createLazyRoute(() => import('@/pages/app/ops/notification-channel')),
        meta: {
          title: 'routes:notification-channels',
          icon: 'lucide:radio',
          order: 3,
          authority: ['sys:platform_admin'],
        },
      },
    ],
  },

  // ── 治理 ──
  {
    name: 'ops-govern',
    meta: {
      title: 'routes:opsGovern',
      icon: 'lucide:shield-check',
      order: 6,
      authority: ['sys:platform_admin', 'ops:node_manage'],
    },
    children: [
      {
        name: 'approvals',
        path: '/ops/approvals',
        element: createLazyRoute(() => import('@/pages/app/ops/approval')),
        meta: {
          title: 'routes:approvals',
          icon: 'lucide:check-circle',
          order: 1,
          authority: ['sys:platform_admin'],
        },
      },
      {
        name: 'command-records',
        path: '/ops/command-records',
        element: createLazyRoute(() => import('@/pages/app/ops/command-record')),
        meta: {
          title: 'routes:command-records',
          icon: 'lucide:scroll-text',
          order: 2,
          authority: ['ops:node_manage'],
        },
      },
      {
        name: 'certs',
        path: '/ops/certs',
        element: createLazyRoute(() => import('@/pages/app/ops/cert')),
        meta: {
          title: 'routes:certs',
          icon: 'lucide:file-badge',
          order: 4,
          authority: ['sys:platform_admin'],
        },
      },
      {
        name: 'api-keys',
        path: '/ops/api-keys',
        element: createLazyRoute(() => import('@/pages/app/ops/api-key')),
        meta: {
          title: 'routes:api-keys',
          icon: 'lucide:key-round',
          order: 3,
          authority: ['sys:platform_admin'],
        },
      },
    ],
  },

  // ── 云管理 ──
  {
    name: 'ops-cloud',
    meta: {
      title: 'routes:opsCloud',
      icon: 'lucide:cloudy',
      order: 2,
      authority: ['sys:platform_admin'],
    },
    children: [
      {
        name: 'cloud-accounts',
        path: '/ops/cloud-accounts',
        element: createLazyRoute(() => import('@/pages/app/ops/cloud-account')),
        meta: {
          title: 'routes:cloud-accounts',
          icon: 'lucide:cloud',
          order: 1,
          authority: ['sys:platform_admin'],
        },
      },
      {
        name: 'cloud-resources',
        path: '/ops/cloud-resources',
        element: createLazyRoute(() => import('@/pages/app/ops/cloud-resource')),
        meta: {
          title: 'routes:cloud-resources',
          icon: 'lucide:server',
          order: 2,
          authority: ['sys:platform_admin'],
        },
      },
      {
        name: 'cloud-budgets',
        path: '/ops/cloud-budgets',
        element: createLazyRoute(() => import('@/pages/app/ops/cloud-budget')),
        meta: {
          title: 'routes:cloud-budgets',
          icon: 'lucide:wallet',
          order: 4,
          authority: ['sys:platform_admin'],
        },
      },
      {
        name: 'cloud-costs',
        path: '/ops/cloud-costs',
        element: createLazyRoute(() => import('@/pages/app/ops/cloud-cost')),
        meta: {
          title: 'routes:cloud-costs',
          icon: 'lucide:coins',
          order: 3,
          authority: ['sys:platform_admin'],
        },
      },
    ],
  },
];

export default opsRoutes;
