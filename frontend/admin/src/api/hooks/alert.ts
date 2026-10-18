import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_Alert,
  type opsservicev1_GetAlertRequest,
  type opsservicev1_ListAlertResponse,
  type opsservicev1_ResolveAlertRequest,
} from '@/api/generated/admin/service/v1';
import type { PaginationQuery } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 告警记录(只读+确认恢复)
// ==============================

export function useListAlerts(
  query: PaginationQuery,
  options?: Partial<UseQueryOptions<opsservicev1_ListAlertResponse, Error, opsservicev1_ListAlertResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listAlerts', query],
    queryFn: () => apiClient.alertService.List(query.toRawParams()),
    ...options,
  });
}

export function useGetAlert(
  req: opsservicev1_GetAlertRequest,
  options?: Partial<UseQueryOptions<opsservicev1_Alert, Error, opsservicev1_Alert, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['getAlert', req],
    queryFn: () => apiClient.alertService.Get(req),
    ...options,
  });
}

export async function fetchListAlerts(params: PaginationQuery) {
  return apiClient.alertService.List(params.toRawParams());
}

export function useResolveAlert(
  options?: UseMutationOptions<Record<never, never>, Error, opsservicev1_ResolveAlertRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.alertService.Resolve(req),
    ...options,
  });
}
