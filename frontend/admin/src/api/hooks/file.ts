import {
  useMutation,
  useQuery,
  type UseMutationOptions,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_CompleteAgentFilePushResponse,
  type opsservicev1_DeleteAgentFileResponse,
  type opsservicev1_PrepareAgentFilePushResponse,
  type opsservicev1_PullAgentFileResponse,
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

export async function fetchPullAgentFile(agentId: string, path: string): Promise<opsservicev1_PullAgentFileResponse> {
  return apiClient.fileService.PullAgentFile({ agentId, path });
}

export async function fetchPrepareAgentFilePush(agentId: string, path: string): Promise<opsservicev1_PrepareAgentFilePushResponse> {
  return apiClient.fileService.PrepareAgentFilePush({ agentId, path });
}

export async function fetchCompleteAgentFilePush(agentId: string, path: string, objectKey: string): Promise<opsservicev1_CompleteAgentFilePushResponse> {
  return apiClient.fileService.CompleteAgentFilePush({ agentId, path, objectKey });
}

/** 浏览器直传对象存储(预签名 PUT)。 */
export async function putFileToPresignedUrl(url: string, file: File): Promise<void> {
  const resp = await fetch(url, { method: 'PUT', body: file });
  if (!resp.ok) {
    throw new Error(`upload failed: HTTP ${resp.status}`);
  }
}

export function useDeleteAgentFile(
  options?: Partial<UseMutationOptions<opsservicev1_DeleteAgentFileResponse, Error, { agentId: string; path: string }>>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.fileService.DeleteAgentFile(req),
    ...options,
  });
}
