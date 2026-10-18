import {
  useMutation,
  useQuery,
  type UseMutationOptions,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_GetPipelineRunLogResponse,
  type opsservicev1_GetPipelineRunResponse,
  type opsservicev1_ListPipelineResponse,
  type opsservicev1_ListPipelineRunResponse,
  type opsservicev1_Pipeline,
  type opsservicev1_PipelineRun,
  type opsservicev1_TriggerPipelineResponse,
} from '@/api/generated/admin/service/v1';
import type { PaginationQuery } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 构建流水线
// ==============================

export function useListPipelines(
  query: PaginationQuery,
  options?: Partial<UseQueryOptions<opsservicev1_ListPipelineResponse, Error, opsservicev1_ListPipelineResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listPipelines', query],
    queryFn: () => apiClient.pipelineService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListPipelines(params: PaginationQuery) {
  return apiClient.pipelineService.List(params.toRawParams());
}

export function useCreatePipeline(
  options?: Partial<UseMutationOptions<opsservicev1_Pipeline, Error, { data: Partial<opsservicev1_Pipeline> }>>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.pipelineService.Create(req as never),
    ...options,
  });
}

export function useUpdatePipeline(
  options?: Partial<UseMutationOptions<void, Error, { id: number; data: Partial<opsservicev1_Pipeline> }>>,
) {
  return useMutation({
    mutationFn: ({ id, data }) => apiClient.pipelineService.Update({ id, data } as never),
    ...options,
  });
}

export function useDeletePipeline(
  options?: Partial<UseMutationOptions<void, Error, { id: number }>>,
) {
  return useMutation({
    mutationFn: ({ id }) => apiClient.pipelineService.Delete({ id }),
    ...options,
  });
}

export function useTriggerPipeline(
  options?: Partial<UseMutationOptions<opsservicev1_TriggerPipelineResponse, Error, { id: number }>>,
) {
  return useMutation({
    mutationFn: ({ id }) => apiClient.pipelineService.Trigger({ id }),
    ...options,
  });
}

export function useListPipelineRuns(
  query: PaginationQuery,
  options?: Partial<UseQueryOptions<opsservicev1_ListPipelineRunResponse, Error, opsservicev1_ListPipelineRunResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listPipelineRuns', query],
    queryFn: () => apiClient.pipelineService.ListRuns(query.toRawParams()),
    ...options,
  });
}

export async function fetchListPipelineRuns(params: PaginationQuery) {
  return apiClient.pipelineService.ListRuns(params.toRawParams());
}

export function useGetPipelineRun(
  req: { id?: number },
  options?: Partial<UseQueryOptions<opsservicev1_PipelineRun, Error, opsservicev1_PipelineRun, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['getPipelineRun', req],
    queryFn: () => apiClient.pipelineService.GetRun(req),
    enabled: Boolean(req.id),
    ...options,
  });
}

/** 增量拉取运行日志:offset 传上次返回的 newOffset(0=全量)。 */
export async function fetchPipelineRunLog(id: number, offset: number): Promise<opsservicev1_GetPipelineRunLogResponse> {
  return apiClient.pipelineService.GetRunLog({ id, offset });
}
