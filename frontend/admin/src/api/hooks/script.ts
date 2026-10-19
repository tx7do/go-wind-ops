import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_DeleteScriptRequest,
  type opsservicev1_ExecuteScriptRequest,
  type opsservicev1_ExecuteScriptResponse,
  type opsservicev1_GetScriptRequest,
  type opsservicev1_ListScriptResponse,
  type opsservicev1_Script,
} from '@/api/generated/admin/service/v1';
import { makeUpdateMask, type PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 脚本管理
// ==============================

export function useListScripts(
  query: PaginationQuery,
  options?: Partial<UseQueryOptions<opsservicev1_ListScriptResponse, Error, opsservicev1_ListScriptResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listScripts', query],
    queryFn: () => apiClient.scriptService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListScripts(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listScripts', params],
    queryFn: () => apiClient.scriptService.List(params.toRawParams()),
    retry: 0,
  });
}

export function useGetScript(
  req: opsservicev1_GetScriptRequest,
  options?: Partial<UseQueryOptions<opsservicev1_Script, Error, opsservicev1_Script, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['getScript', req],
    queryFn: () => apiClient.scriptService.Get(req),
    ...options,
  });
}

export function useCreateScript(
  options?: UseMutationOptions<opsservicev1_Script, Error, Partial<opsservicev1_Script>>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.scriptService.Create({ data: data as opsservicev1_Script }),
    ...options,
  });
}

export function useUpdateScript(
  options?: UseMutationOptions<{}, Error, { id: number; values: Record<string, any> }>,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) =>
      apiClient.scriptService.Update({
        id,
        data: { ...values } as opsservicev1_Script,
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeleteScript(
  options?: UseMutationOptions<{}, Error, opsservicev1_DeleteScriptRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.scriptService.Delete(req),
    ...options,
  });
}

export function useExecuteScript(
  options?: UseMutationOptions<opsservicev1_ExecuteScriptResponse, Error, Partial<opsservicev1_ExecuteScriptRequest>>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.scriptService.Execute(req as never),
    ...options,
  });
}
