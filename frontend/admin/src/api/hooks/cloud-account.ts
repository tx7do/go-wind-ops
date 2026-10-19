import {
  useMutation,
  useQuery,
  type UseMutationOptions,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_CloudAccount,
  type opsservicev1_ListCloudAccountsResponse,
  type opsservicev1_VerifyCloudAccountResponse,
} from '@/api/generated/admin/service/v1';
import type { PaginationQuery } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 云平台账号管理
// ==============================

export function useListCloudAccounts(
  query: PaginationQuery,
  options?: Partial<UseQueryOptions<opsservicev1_ListCloudAccountsResponse, Error, opsservicev1_ListCloudAccountsResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listCloudAccounts', query],
    queryFn: () => apiClient.cloudAccountService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListCloudAccounts(params: PaginationQuery) {
  return apiClient.cloudAccountService.List(params.toRawParams());
}

export function useCreateCloudAccount(
  options?: Partial<UseMutationOptions<opsservicev1_CloudAccount, Error, { data: Partial<opsservicev1_CloudAccount>; secretKey?: string }>>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.cloudAccountService.Create(req as never),
    ...options,
  });
}

export function useUpdateCloudAccount(
  options?: Partial<UseMutationOptions<void, Error, { id: number; data: Partial<opsservicev1_CloudAccount>; secretKey?: string }>>,
) {
  return useMutation({
    mutationFn: ({ id, data, secretKey }) =>
      apiClient.cloudAccountService.Update({ id, data, secretKey } as never),
    ...options,
  });
}

export function useDeleteCloudAccount(
  options?: Partial<UseMutationOptions<void, Error, { id: number }>>,
) {
  return useMutation({
    mutationFn: ({ id }) => apiClient.cloudAccountService.Delete({ id }),
    ...options,
  });
}

export function useVerifyCloudAccount(
  options?: Partial<UseMutationOptions<opsservicev1_VerifyCloudAccountResponse, Error, { id: number }>>,
) {
  return useMutation({
    mutationFn: ({ id }) => apiClient.cloudAccountService.Verify({ id }),
    ...options,
  });
}
