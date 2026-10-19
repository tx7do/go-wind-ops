import {
  useMutation,
  useQuery,
  type UseMutationOptions,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_ListCloudInstancesResponse,
  type opsservicev1_SyncCloudAccountInstancesResponse,
} from '@/api/generated/admin/service/v1';
import { apiClient } from '@/api/client';

// ==============================
// 云资源实例清单
// ==============================

export interface CloudInstanceListParams {
  page?: number;
  pageSize?: number;
  cloudAccountId?: number;
  provider?: string;
  region?: string;
  matched?: boolean;
  keyword?: string;
}

export function useListCloudInstances(
  params: CloudInstanceListParams,
  options?: Partial<UseQueryOptions<opsservicev1_ListCloudInstancesResponse, Error, opsservicev1_ListCloudInstancesResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listCloudInstances', params],
    queryFn: () => apiClient.cloudInstanceService.List(params as never),
    ...options,
  });
}

export async function fetchListCloudInstances(params: CloudInstanceListParams) {
  return apiClient.cloudInstanceService.List(params as never);
}

export function useSyncCloudAccountInstances(
  options?: Partial<UseMutationOptions<opsservicev1_SyncCloudAccountInstancesResponse, Error, { cloudAccountId: number }>>,
) {
  return useMutation({
    mutationFn: ({ cloudAccountId }) =>
      apiClient.cloudInstanceService.Sync({ cloudAccountId } as never),
    ...options,
  });
}
