import {
  useMutation,
  useQuery,
  type UseMutationOptions,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_CloudCost,
  type opsservicev1_FetchCloudCostFromProviderResponse,
  type opsservicev1_GetCloudCostSummaryResponse,
  type opsservicev1_ImportCloudCostsResponse,
  type opsservicev1_ListCloudCostsResponse,
} from '@/api/generated/admin/service/v1';
import { apiClient } from '@/api/client';

// ==============================
// 云成本月账单
// ==============================

export interface CloudCostListParams {
  page?: number;
  pageSize?: number;
  cloudAccountId?: number;
  year?: string;
  provider?: string;
}

export function useListCloudCosts(
  params: CloudCostListParams,
  options?: Partial<UseQueryOptions<opsservicev1_ListCloudCostsResponse, Error, opsservicev1_ListCloudCostsResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listCloudCosts', params],
    queryFn: () => apiClient.cloudCostService.List(params as never),
    ...options,
  });
}

export async function fetchListCloudCosts(params: CloudCostListParams) {
  return apiClient.cloudCostService.List(params as never);
}

export function useUpsertCloudCost(
  options?: Partial<UseMutationOptions<opsservicev1_CloudCost, Error, { data: Partial<opsservicev1_CloudCost> }>>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.cloudCostService.Upsert(req as never),
    ...options,
  });
}

export function useDeleteCloudCost(
  options?: Partial<UseMutationOptions<void, Error, { id: number }>>,
) {
  return useMutation({
    mutationFn: ({ id }) => apiClient.cloudCostService.Delete({ id }),
    ...options,
  });
}

export function useImportCloudCosts(
  options?: Partial<UseMutationOptions<opsservicev1_ImportCloudCostsResponse, Error, { rows: Partial<opsservicev1_CloudCost>[] }>>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.cloudCostService.Import(req as never),
    ...options,
  });
}

export function useCloudCostSummary(
  params: { year?: string; provider?: string },
  options?: Partial<UseQueryOptions<opsservicev1_GetCloudCostSummaryResponse, Error, opsservicev1_GetCloudCostSummaryResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['cloudCostSummary', params],
    queryFn: () => apiClient.cloudCostService.Summary(params as never),
    ...options,
  });
}

export function useFetchCloudCostFromProvider(
  options?: Partial<UseMutationOptions<opsservicev1_FetchCloudCostFromProviderResponse, Error, { cloudAccountId: number; month: string }>>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.cloudCostService.Fetch(req as never),
    ...options,
  });
}
