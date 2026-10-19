import {
  useMutation,
  useQuery,
  type UseMutationOptions,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_ApiKey,
  type opsservicev1_CreateApiKeyResponse,
  type opsservicev1_ListApiKeyResponse,
} from '@/api/generated/admin/service/v1';
import type { PaginationQuery } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// OpenAPI 密钥管理
// ==============================

export function useListApiKeys(
  query: PaginationQuery,
  options?: Partial<UseQueryOptions<opsservicev1_ListApiKeyResponse, Error, opsservicev1_ListApiKeyResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listApiKeys', query],
    queryFn: () => apiClient.apiKeyService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListApiKeys(params: PaginationQuery) {
  return apiClient.apiKeyService.List(params.toRawParams());
}

export function useCreateApiKey(
  options?: Partial<UseMutationOptions<opsservicev1_CreateApiKeyResponse, Error, { data: Partial<opsservicev1_ApiKey> }>>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.apiKeyService.Create(req as never),
    ...options,
  });
}

export function useUpdateApiKey(
  options?: Partial<UseMutationOptions<void, Error, { id: number; data: Partial<opsservicev1_ApiKey> }>>,
) {
  return useMutation({
    mutationFn: async ({ id, data }) => { await apiClient.apiKeyService.Update({ id, data } as never); },
    ...options,
  });
}

export function useDeleteApiKey(
  options?: Partial<UseMutationOptions<void, Error, { id: number }>>,
) {
  return useMutation({
    mutationFn: async ({ id }) => { await apiClient.apiKeyService.Delete({ id }); },
    ...options,
  });
}
