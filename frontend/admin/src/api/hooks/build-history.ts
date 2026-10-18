import {
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_BuildHistory,
  type opsservicev1_GetArtifactDownloadUrlResponse,
  type opsservicev1_GetBuildHistoryRequest,
  type opsservicev1_ListBuildHistoryResponse,
} from '@/api/generated/admin/service/v1';
import type { PaginationQuery } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 构建历史(只读)
// ==============================

export function useListBuildHistories(
  query: PaginationQuery,
  options?: UseQueryOptions<opsservicev1_ListBuildHistoryResponse, Error>,
) {
  return useQuery({
    queryKey: ['listBuildHistories', query],
    queryFn: () => apiClient.buildHistoryService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListBuildHistories(params: PaginationQuery) {
  return apiClient.buildHistoryService.List(params.toRawParams());
}

export function useGetBuildHistory(
  req: opsservicev1_GetBuildHistoryRequest,
  options?: UseQueryOptions<opsservicev1_BuildHistory, Error>,
) {
  return useQuery({
    queryKey: ['getBuildHistory', req],
    queryFn: () => apiClient.buildHistoryService.Get(req),
    ...options,
  });
}

export async function fetchArtifactDownloadUrl(id: number): Promise<opsservicev1_GetArtifactDownloadUrlResponse> {
  return apiClient.buildHistoryService.GetArtifactUrl({ id });
}
