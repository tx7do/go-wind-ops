import {
  useMutation,
  useQuery,
  type UseMutationOptions,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_ExecAgentPluginResponse,
  type opsservicev1_ListAgentPluginsResponse,
} from '@/api/generated/admin/service/v1';
import { apiClient } from '@/api/client';

// ==============================
// 插件化指令(agent plugins/ 目录脚本)
// ==============================

export function useListAgentPlugins(
  req: { agentId?: string },
  options?: Partial<UseQueryOptions<opsservicev1_ListAgentPluginsResponse, Error, opsservicev1_ListAgentPluginsResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listAgentPlugins', req.agentId],
    queryFn: () => apiClient.pluginService.ListAgentPlugins(req),
    enabled: Boolean(req.agentId),
    ...options,
  });
}

export function useExecAgentPlugin(
  options?: Partial<UseMutationOptions<opsservicev1_ExecAgentPluginResponse, Error, { agentId: string; action: string; args?: string[]; timeoutSeconds?: number }>>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.pluginService.ExecAgentPlugin(req as never),
    ...options,
  });
}
