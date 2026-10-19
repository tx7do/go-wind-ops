import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_ListLogSnapshotResponse,
  type opsservicev1_LogSnapshot,
  type opsservicev1_SearchLogSnapshotResponse,
} from '@/api/generated/admin/service/v1';
import { PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 项目日志快照
// ==============================

export function useListLogSnapshots(
  query: PaginationQuery,
  options?: Partial<UseQueryOptions<opsservicev1_ListLogSnapshotResponse, Error, opsservicev1_ListLogSnapshotResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listLogSnapshots', query],
    queryFn: () => apiClient.logSnapshotService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListLogSnapshots(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listLogSnapshots', params],
    queryFn: () => apiClient.logSnapshotService.List(params.toRawParams()),
    retry: 0,
  });
}

export function useCaptureLogSnapshot(
  options?: UseMutationOptions<opsservicev1_LogSnapshot, Error, { projectId: number }>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.logSnapshotService.Capture(req as never),
    ...options,
  });
}

export function useSearchLogSnapshots(
  options?: UseMutationOptions<opsservicev1_SearchLogSnapshotResponse, Error, { keyword: string; projectId?: number }>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.logSnapshotService.Search(req as never),
    ...options,
  });
}

export function useDeleteLogSnapshot(
  options?: UseMutationOptions<Record<never, never>, Error, { id: number }>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.logSnapshotService.Delete(req as never),
    ...options,
  });
}

export function useGetLogSnapshot(
  id?: number,
  options?: Partial<UseQueryOptions<opsservicev1_LogSnapshot, Error, opsservicev1_LogSnapshot, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['getLogSnapshot', id],
    queryFn: () => apiClient.logSnapshotService.Get({ id } as never),
    enabled: !!id,
    ...options,
  });
}
