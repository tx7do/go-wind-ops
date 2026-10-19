import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_AgentNode,
  type opsservicev1_DeleteAgentNodeRequest,
  type opsservicev1_GetAgentNodeRequest,
  type opsservicev1_ListAgentNodeResponse,
  type opsservicev1_SendCommandRequest,
  type opsservicev1_SendCommandResponse,
  type opsservicev1_UpdateAgentNodeRequest,
  type opsservicev1_GetAgentUpgradeUploadUrlResponse,
  type opsservicev1_UpgradeAgentResponse,
} from '@/api/generated/admin/service/v1';
import { makeUpdateMask, type PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// Agent 节点管理
// ==============================

export function useListAgentNodes(
  query: PaginationQuery,
  options?: Partial<UseQueryOptions<opsservicev1_ListAgentNodeResponse, Error, opsservicev1_ListAgentNodeResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listAgentNodes', query],
    queryFn: () => apiClient.agentNodeService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListAgentNodes(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listAgentNodes', params],
    queryFn: () => apiClient.agentNodeService.List(params.toRawParams()),
    retry: 0,
  });
}

export function useGetAgentNode(
  req: opsservicev1_GetAgentNodeRequest,
  options?: Partial<UseQueryOptions<opsservicev1_AgentNode, Error, opsservicev1_AgentNode, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['getAgentNode', req],
    queryFn: () => apiClient.agentNodeService.Get(req),
    ...options,
  });
}

export function useUpdateAgentNode(
  options?: UseMutationOptions<Record<never, never>, Error, { id: number; values: Record<string, any> }>,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) => {
      const req: opsservicev1_UpdateAgentNodeRequest = {
        id,
        data: { ...values } as opsservicev1_AgentNode,
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      };
      return apiClient.agentNodeService.Update(req);
    },
    ...options,
  });
}

export function useDeleteAgentNode(
  options?: UseMutationOptions<Record<never, never>, Error, opsservicev1_DeleteAgentNodeRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.agentNodeService.Delete(req),
    ...options,
  });
}

export function useSendAgentCommand(
  options?: UseMutationOptions<opsservicev1_SendCommandResponse, Error, opsservicev1_SendCommandRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.agentNodeService.SendCommand(req),
    ...options,
  });
}

export function useGetAgentUpgradeUploadUrl(
  options?: UseMutationOptions<
    opsservicev1_GetAgentUpgradeUploadUrlResponse,
    Error,
    { filename: string }
  >,
) {
  return useMutation({
    mutationFn: (req) => apiClient.agentNodeService.GetUpgradeUploadUrl(req as never),
    ...options,
  });
}

export function useUpgradeAgent(
  options?: UseMutationOptions<
    opsservicev1_UpgradeAgentResponse,
    Error,
    { nodeId: number; version: string; objectKey: string }
  >,
) {
  return useMutation({
    mutationFn: (req) => apiClient.agentNodeService.Upgrade(req as never),
    ...options,
  });
}
