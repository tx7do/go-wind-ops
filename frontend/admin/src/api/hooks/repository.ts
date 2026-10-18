import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_DeleteRepositoryRequest,
  type opsservicev1_GetRepositoryRequest,
  type opsservicev1_ListRepositoryResponse,
  type opsservicev1_Repository,
} from '@/api/generated/admin/service/v1';
import { makeUpdateMask, type PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 构建源仓库管理
// ==============================

export function useListRepositories(
  query: PaginationQuery,
  options?: Partial<UseQueryOptions<opsservicev1_ListRepositoryResponse, Error, opsservicev1_ListRepositoryResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listRepositories', query],
    queryFn: () => apiClient.repositoryService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListRepositories(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listRepositories', params],
    queryFn: () => apiClient.repositoryService.List(params.toRawParams()),
    retry: 0,
  });
}

export function useGetRepository(
  req: opsservicev1_GetRepositoryRequest,
  options?: Partial<UseQueryOptions<opsservicev1_Repository, Error, opsservicev1_Repository, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['getRepository', req],
    queryFn: () => apiClient.repositoryService.Get(req),
    ...options,
  });
}

export function useCreateRepository(
  options?: UseMutationOptions<opsservicev1_Repository, Error, Partial<opsservicev1_Repository>>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.repositoryService.Create({ data: data as opsservicev1_Repository }),
    ...options,
  });
}

export function useUpdateRepository(
  options?: UseMutationOptions<{}, Error, { id: number; values: Record<string, any> }>,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) =>
      apiClient.repositoryService.Update({
        id,
        data: { ...values } as opsservicev1_Repository,
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeleteRepository(
  options?: UseMutationOptions<{}, Error, opsservicev1_DeleteRepositoryRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.repositoryService.Delete(req),
    ...options,
  });
}
