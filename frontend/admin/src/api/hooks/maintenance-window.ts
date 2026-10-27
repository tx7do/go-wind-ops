import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_ListMaintenanceWindowResponse,
  type opsservicev1_MaintenanceWindow,
} from '@/api/generated/admin/service/v1';
import { PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 告警维护窗口
// ==============================

export function useListMaintenanceWindows(
  query: PaginationQuery,
  options?: Partial<UseQueryOptions<opsservicev1_ListMaintenanceWindowResponse, Error, opsservicev1_ListMaintenanceWindowResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listMaintenanceWindows', query],
    queryFn: () => apiClient.maintenanceWindowService.ListMaintenanceWindows(query.toRawParams()),
    ...options,
  });
}

export async function fetchListMaintenanceWindows(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listMaintenanceWindows', params],
    queryFn: () => apiClient.maintenanceWindowService.ListMaintenanceWindows(params.toRawParams()),
    retry: 0,
  });
}

export function useCreateMaintenanceWindow(
  options?: UseMutationOptions<opsservicev1_MaintenanceWindow, Error, Partial<opsservicev1_MaintenanceWindow>>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.maintenanceWindowService.CreateMaintenanceWindow({ data: data as opsservicev1_MaintenanceWindow }),
    ...options,
  });
}

export function useUpdateMaintenanceWindow(
  options?: UseMutationOptions<Record<never, never>, Error, { id: number; values: Partial<opsservicev1_MaintenanceWindow> }>,
) {
  return useMutation({
    mutationFn: ({ id, values }) =>
      apiClient.maintenanceWindowService.UpdateMaintenanceWindow({
        id,
        data: { ...values } as opsservicev1_MaintenanceWindow,
      }),
    ...options,
  });
}

export function useDeleteMaintenanceWindow(
  options?: UseMutationOptions<Record<never, never>, Error, { id: number }>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.maintenanceWindowService.DeleteMaintenanceWindow(req as never),
    ...options,
  });
}
