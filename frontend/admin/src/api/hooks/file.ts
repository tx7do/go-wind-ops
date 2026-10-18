import {
  useMutation,
  useQuery,
  type UseMutationOptions,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_DeleteAgentFileResponse,
  type opsservicev1_ListAgentFilesResponse,
  type opsservicev1_ReadAgentFileResponse,
  type opsservicev1_WriteAgentFileResponse,
} from '@/api/generated/admin/service/v1';
import { apiClient } from '@/api/client';

// ==============================
// 远端文件管理(经 MQTT 代理 agent)
// ==============================

export function useListAgentFiles(
  req: { agentId?: string; dir?: string },
  options?: Partial<UseQueryOptions<opsservicev1_ListAgentFilesResponse, Error, opsservicev1_ListAgentFilesResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listAgentFiles', req.agentId, req.dir],
    queryFn: () => apiClient.fileService.ListAgentFiles(req),
    enabled: options?.enabled ?? Boolean(req.agentId && req.dir),
    ...options,
  });
}

export function useReadAgentFile(
  options?: Partial<UseMutationOptions<opsservicev1_ReadAgentFileResponse, Error, { agentId: string; path: string }>>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.fileService.ReadAgentFile(req),
    ...options,
  });
}

export function useWriteAgentFile(
  options?: Partial<UseMutationOptions<opsservicev1_WriteAgentFileResponse, Error, { agentId: string; path: string; contentBase64: string }>>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.fileService.WriteAgentFile(req),
    ...options,
  });
}

export function useDeleteAgentFile(
  options?: Partial<UseMutationOptions<opsservicev1_DeleteAgentFileResponse, Error, { agentId: string; path: string }>>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.fileService.DeleteAgentFile(req),
    ...options,
  });
}
