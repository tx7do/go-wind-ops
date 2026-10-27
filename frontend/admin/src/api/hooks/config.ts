import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_ConfigDistribution,
  type opsservicev1_ConfigTemplate,
  type opsservicev1_ListConfigTemplateResponse,
  type opsservicev1_ConfigDriftReport,
} from '@/api/generated/admin/service/v1';
import { makeUpdateMask, PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 配置文件中心
// ==============================

export function useListConfigTemplates(
  query: PaginationQuery,
  options?: Partial<UseQueryOptions<opsservicev1_ListConfigTemplateResponse, Error, opsservicev1_ListConfigTemplateResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listConfigTemplates', query],
    queryFn: () => apiClient.configService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListConfigTemplates(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listConfigTemplates', params],
    queryFn: () => apiClient.configService.List(params.toRawParams()),
    retry: 0,
  });
}

export function useCreateConfigTemplate(
  options?: UseMutationOptions<opsservicev1_ConfigTemplate, Error, Partial<opsservicev1_ConfigTemplate>>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.configService.Create({ data: data as opsservicev1_ConfigTemplate }),
    ...options,
  });
}

export function useUpdateConfigTemplate(
  options?: UseMutationOptions<Record<never, never>, Error, { id: number; values: Record<string, any> }>,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) =>
      apiClient.configService.Update({
        id,
        data: { ...values } as opsservicev1_ConfigTemplate,
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeleteConfigTemplate(
  options?: UseMutationOptions<Record<never, never>, Error, { id: number }>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.configService.Delete(req as never),
    ...options,
  });
}

export function useDistributeConfig(
  options?: UseMutationOptions<opsservicev1_ConfigDistribution, Error, { templateId: number; nodeId: number; projectId?: number }>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.configService.Distribute(req as never),
    ...options,
  });
}

export function useListConfigDistributions(
  templateId?: number,
  options?: Partial<UseQueryOptions<opsservicev1_ConfigDistribution[], Error, opsservicev1_ConfigDistribution[], readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listConfigDistributions', templateId],
    queryFn: async () => {
      if (!templateId) return [];
      const resp = await apiClient.configService.ListDistributions({
        templateId,
        page: 1,
        pageSize: 50,
      } as never);
      return (resp as never as { items?: opsservicev1_ConfigDistribution[] }).items || [];
    },
    enabled: !!templateId,
    ...options,
  });
}

export function useCheckConfigDrift(
  options?: UseMutationOptions<
    opsservicev1_ConfigDriftReport,
    Error,
    { templateId: number; nodeId: number; projectId?: number }
  >,
) {
  return useMutation({
    mutationFn: (req) => apiClient.configService.CheckConfigDrift(req as never),
    ...options,
  });
}
