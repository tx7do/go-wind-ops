import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_DeleteDistributionRequest,
  type opsservicev1_DistributionRecord,
  type opsservicev1_GetDistributionRecordRequest,
  type opsservicev1_GetDistributionRequest,
  type opsservicev1_ListDistributionRecordResponse,
  type opsservicev1_ListDistributionResponse,
  type opsservicev1_Distribution,
  type opsservicev1_TriggerDistributionRequest,
  type opsservicev1_TriggerDistributionResponse,
} from '@/api/generated/admin/service/v1';
import { makeUpdateMask, type PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 构建产物分发
// ==============================

export function useListDistributions(
  query: PaginationQuery,
  options?: Partial<UseQueryOptions<opsservicev1_ListDistributionResponse, Error, opsservicev1_ListDistributionResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listDistributions', query],
    queryFn: () => apiClient.distributionService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListDistributions(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listDistributions', params],
    queryFn: () => apiClient.distributionService.List(params.toRawParams()),
    retry: 0,
  });
}

export function useGetDistribution(
  req: opsservicev1_GetDistributionRequest,
  options?: Partial<UseQueryOptions<opsservicev1_Distribution, Error, opsservicev1_Distribution, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['getDistribution', req],
    queryFn: () => apiClient.distributionService.Get(req),
    ...options,
  });
}

export function useCreateDistribution(
  options?: UseMutationOptions<opsservicev1_Distribution, Error, Partial<opsservicev1_Distribution>>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.distributionService.Create({ data: data as opsservicev1_Distribution }),
    ...options,
  });
}

export function useUpdateDistribution(
  options?: UseMutationOptions<{}, Error, { id: number; values: Record<string, any> }>,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) =>
      apiClient.distributionService.Update({
        id,
        data: { ...values } as opsservicev1_Distribution,
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeleteDistribution(
  options?: UseMutationOptions<{}, Error, opsservicev1_DeleteDistributionRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.distributionService.Delete(req),
    ...options,
  });
}

export function useTriggerDistribution(
  options?: UseMutationOptions<opsservicev1_TriggerDistributionResponse, Error, Partial<opsservicev1_TriggerDistributionRequest>>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.distributionService.Trigger(req),
    ...options,
  });
}

export function useListDistributionRecords(
  query: PaginationQuery,
  options?: Partial<UseQueryOptions<opsservicev1_ListDistributionRecordResponse, Error, opsservicev1_ListDistributionRecordResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listDistributionRecords', query],
    queryFn: () => apiClient.distributionService.ListRecords(query.toRawParams()),
    ...options,
  });
}

export async function fetchListDistributionRecords(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listDistributionRecords', params],
    queryFn: () => apiClient.distributionService.ListRecords(params.toRawParams()),
    retry: 0,
  });
}

export function useGetDistributionRecord(
  req: opsservicev1_GetDistributionRecordRequest,
  options?: Partial<UseQueryOptions<opsservicev1_DistributionRecord, Error, opsservicev1_DistributionRecord, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['getDistributionRecord', req],
    queryFn: () => apiClient.distributionService.GetRecord(req),
    ...options,
  });
}
