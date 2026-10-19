import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_Inspection,
  type opsservicev1_ListInspectionResponse,
} from '@/api/generated/admin/service/v1';
import { PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 平台巡检报告
// ==============================

export function useListInspections(
  query: PaginationQuery,
  options?: Partial<UseQueryOptions<opsservicev1_ListInspectionResponse, Error, opsservicev1_ListInspectionResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listInspections', query],
    queryFn: () => apiClient.inspectionService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListInspections(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listInspections', params],
    queryFn: () => apiClient.inspectionService.List(params.toRawParams()),
    retry: 0,
  });
}

export function useGetInspection(
  id?: number,
  options?: Partial<UseQueryOptions<opsservicev1_Inspection, Error, opsservicev1_Inspection, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['getInspection', id],
    queryFn: () => apiClient.inspectionService.Get({ id } as never),
    enabled: !!id,
    ...options,
  });
}

export function useRunInspection(
  options?: UseMutationOptions<opsservicev1_Inspection, Error, void>,
) {
  return useMutation({
    mutationFn: () => apiClient.inspectionService.Run({} as never),
    ...options,
  });
}
