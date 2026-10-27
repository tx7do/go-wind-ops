import {
  useMutation,
  useQuery,
  type UseMutationOptions,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_ListPromotionPipelineResponse,
  type opsservicev1_PromotePromotionResponse,
  type opsservicev1_GetPromotionStatusResponse,
  type opsservicev1_PromotionPipeline,
} from '@/api/generated/admin/service/v1';
import { PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 环境晋升流水线
// ==============================

export function useListPromotionPipelines(
  query: PaginationQuery,
  options?: Partial<UseQueryOptions<opsservicev1_ListPromotionPipelineResponse, Error, opsservicev1_ListPromotionPipelineResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listPromotionPipelines', query],
    queryFn: () => apiClient.promotionPipelineService.ListPromotionPipelines(query.toRawParams()),
    ...options,
  });
}

export async function fetchListPromotionPipelines(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listPromotionPipelines', params],
    queryFn: () => apiClient.promotionPipelineService.ListPromotionPipelines(params.toRawParams()),
    retry: 0,
  });
}

export function useCreatePromotionPipeline(
  options?: UseMutationOptions<opsservicev1_PromotionPipeline, Error, Partial<opsservicev1_PromotionPipeline>>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.promotionPipelineService.CreatePromotionPipeline({ data: data as opsservicev1_PromotionPipeline }),
    ...options,
  });
}

export function useUpdatePromotionPipeline(
  options?: UseMutationOptions<Record<never, never>, Error, { id: number; values: Partial<opsservicev1_PromotionPipeline> }>,
) {
  return useMutation({
    mutationFn: ({ id, values }) =>
      apiClient.promotionPipelineService.UpdatePromotionPipeline({
        id,
        data: { ...values } as opsservicev1_PromotionPipeline,
      }),
    ...options,
  });
}

export function useDeletePromotionPipeline(
  options?: UseMutationOptions<Record<never, never>, Error, { id: number }>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.promotionPipelineService.DeletePromotionPipeline(req as never),
    ...options,
  });
}

export function useGetPromotionStatus(
  id: number | undefined,
  options?: Partial<UseQueryOptions<opsservicev1_GetPromotionStatusResponse, Error, opsservicev1_GetPromotionStatusResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['promotionStatus', id],
    queryFn: () => apiClient.promotionPipelineService.GetPromotionStatus({ id } as never),
    enabled: !!id,
    ...options,
  });
}

export function usePromotePromotion(
  options?: UseMutationOptions<opsservicev1_PromotePromotionResponse, Error, { id: number; stageIndex: number; historyId?: number }>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.promotionPipelineService.PromotePromotion(req as never),
    ...options,
  });
}
