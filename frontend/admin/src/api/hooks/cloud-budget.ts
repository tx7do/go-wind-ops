import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_CloudBudget,
  type opsservicev1_ListCloudBudgetResponse,
} from '@/api/generated/admin/service/v1';
import { makeUpdateMask, PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 云成本预算
// ==============================

export function useListCloudBudgets(
  query: PaginationQuery,
  options?: Partial<UseQueryOptions<opsservicev1_ListCloudBudgetResponse, Error, opsservicev1_ListCloudBudgetResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listCloudBudgets', query],
    queryFn: () => apiClient.cloudBudgetService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListCloudBudgets(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listCloudBudgets', params],
    queryFn: () => apiClient.cloudBudgetService.List(params.toRawParams()),
    retry: 0,
  });
}

export function useCreateCloudBudget(
  options?: UseMutationOptions<opsservicev1_CloudBudget, Error, Partial<opsservicev1_CloudBudget>>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.cloudBudgetService.Create({ data: data as opsservicev1_CloudBudget }),
    ...options,
  });
}

export function useUpdateCloudBudget(
  options?: UseMutationOptions<Record<never, never>, Error, { id: number; values: Record<string, any> }>,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) =>
      apiClient.cloudBudgetService.Update({
        id,
        data: { ...values } as opsservicev1_CloudBudget,
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeleteCloudBudget(
  options?: UseMutationOptions<Record<never, never>, Error, { id: number }>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.cloudBudgetService.Delete(req as never),
    ...options,
  });
}

export function useCheckCloudBudget(
  options?: UseMutationOptions<Record<never, never>, Error, void>,
) {
  return useMutation({
    mutationFn: () => apiClient.cloudBudgetService.Check({} as never),
    ...options,
  });
}
