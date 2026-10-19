import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_ExecuteScriptBatchResponse,
  type opsservicev1_GetScriptBatchRequest,
  type opsservicev1_ListScriptBatchResponse,
  type opsservicev1_ScriptBatchDetail,
} from '@/api/generated/admin/service/v1';
import { PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 脚本批量执行
// ==============================

export function useListScriptBatches(
  query: PaginationQuery,
  options?: Partial<UseQueryOptions<opsservicev1_ListScriptBatchResponse, Error, opsservicev1_ListScriptBatchResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listScriptBatches', query],
    queryFn: () => apiClient.scriptBatchService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListScriptBatches(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listScriptBatches', params],
    queryFn: () => apiClient.scriptBatchService.List(params.toRawParams()),
    retry: 0,
  });
}

export function useGetScriptBatch(
  req: opsservicev1_GetScriptBatchRequest,
  options?: Partial<UseQueryOptions<opsservicev1_ScriptBatchDetail, Error, opsservicev1_ScriptBatchDetail, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['getScriptBatch', req],
    queryFn: () => apiClient.scriptBatchService.Get(req),
    ...options,
  });
}

export function useBatchExecScript(
  options?: UseMutationOptions<
    opsservicev1_ExecuteScriptBatchResponse,
    Error,
    { scriptId: number; nodeIds: number[] }
  >,
) {
  return useMutation({
    mutationFn: (req) => apiClient.scriptBatchService.BatchExec(req as never),
    ...options,
  });
}
