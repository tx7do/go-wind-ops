import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_Build,
  type opsservicev1_DeleteBuildRequest,
  type opsservicev1_GetBuildRequest,
  type opsservicev1_ListBuildResponse,
  type opsservicev1_TriggerBuildRequest,
  type opsservicev1_TriggerBuildResponse,
} from '@/api/generated/admin/service/v1';
import { makeUpdateMask, type PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 构建配置管理
// ==============================

export function useListBuilds(
  query: PaginationQuery,
  options?: Partial<UseQueryOptions<opsservicev1_ListBuildResponse, Error, opsservicev1_ListBuildResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listBuilds', query],
    queryFn: () => apiClient.buildService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListBuilds(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listBuilds', params],
    queryFn: () => apiClient.buildService.List(params.toRawParams()),
    retry: 0,
  });
}

export function useGetBuild(
  req: opsservicev1_GetBuildRequest,
  options?: Partial<UseQueryOptions<opsservicev1_Build, Error, opsservicev1_Build, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['getBuild', req],
    queryFn: () => apiClient.buildService.Get(req),
    ...options,
  });
}

export function useCreateBuild(
  options?: UseMutationOptions<opsservicev1_Build, Error, Partial<opsservicev1_Build>>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.buildService.Create({ data: data as opsservicev1_Build }),
    ...options,
  });
}

export function useUpdateBuild(
  options?: UseMutationOptions<{}, Error, { id: number; values: Record<string, any> }>,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) =>
      apiClient.buildService.Update({
        id,
        data: { ...values } as opsservicev1_Build,
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeleteBuild(
  options?: UseMutationOptions<{}, Error, opsservicev1_DeleteBuildRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.buildService.Delete(req),
    ...options,
  });
}

export function useTriggerBuild(
  options?: UseMutationOptions<opsservicev1_TriggerBuildResponse, Error, Partial<opsservicev1_TriggerBuildRequest>>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.buildService.Trigger(req as never),
    ...options,
  });
}
