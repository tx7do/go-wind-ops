import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_DeleteMonitorRequest,
  type opsservicev1_GetMonitorRequest,
  type opsservicev1_ListMonitorResponse,
  type opsservicev1_Monitor,
} from '@/api/generated/admin/service/v1';
import { makeUpdateMask, type PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 监控规则管理
// ==============================

export function useListMonitors(
  query: PaginationQuery,
  options?: Partial<UseQueryOptions<opsservicev1_ListMonitorResponse, Error, opsservicev1_ListMonitorResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listMonitors', query],
    queryFn: () => apiClient.monitorService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListMonitors(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listMonitors', params],
    queryFn: () => apiClient.monitorService.List(params.toRawParams()),
    retry: 0,
  });
}

export function useGetMonitor(
  req: opsservicev1_GetMonitorRequest,
  options?: Partial<UseQueryOptions<opsservicev1_Monitor, Error, opsservicev1_Monitor, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['getMonitor', req],
    queryFn: () => apiClient.monitorService.Get(req),
    ...options,
  });
}

export function useCreateMonitor(
  options?: UseMutationOptions<opsservicev1_Monitor, Error, Partial<opsservicev1_Monitor>>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.monitorService.Create({ data: data as opsservicev1_Monitor }),
    ...options,
  });
}

export function useUpdateMonitor(
  options?: UseMutationOptions<Record<never, never>, Error, { id: number; values: Record<string, any> }>,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) =>
      apiClient.monitorService.Update({
        id,
        data: { ...values } as opsservicev1_Monitor,
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeleteMonitor(
  options?: UseMutationOptions<Record<never, never>, Error, opsservicev1_DeleteMonitorRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.monitorService.Delete(req),
    ...options,
  });
}
