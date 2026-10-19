import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_BackupRecord,
  type opsservicev1_BackupTask,
  type opsservicev1_GetBackupRecordDownloadUrlResponse,
  type opsservicev1_ListBackupRecordResponse,
  type opsservicev1_ListBackupTaskResponse,
  type opsservicev1_TriggerBackupResponse,
} from '@/api/generated/admin/service/v1';
import { makeUpdateMask, PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 备份任务
// ==============================

export function useListBackupTasks(
  query: PaginationQuery,
  options?: Partial<UseQueryOptions<opsservicev1_ListBackupTaskResponse, Error, opsservicev1_ListBackupTaskResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listBackupTasks', query],
    queryFn: () => apiClient.backupService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListBackupTasks(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listBackupTasks', params],
    queryFn: () => apiClient.backupService.List(params.toRawParams()),
    retry: 0,
  });
}

export function useCreateBackupTask(
  options?: UseMutationOptions<opsservicev1_BackupTask, Error, Partial<opsservicev1_BackupTask>>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.backupService.Create({ data: data as opsservicev1_BackupTask }),
    ...options,
  });
}

export function useUpdateBackupTask(
  options?: UseMutationOptions<Record<never, never>, Error, { id: number; values: Record<string, any> }>,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) =>
      apiClient.backupService.Update({
        id,
        data: { ...values } as opsservicev1_BackupTask,
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeleteBackupTask(
  options?: UseMutationOptions<Record<never, never>, Error, { id: number }>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.backupService.Delete(req as never),
    ...options,
  });
}

export function useTriggerBackup(
  options?: UseMutationOptions<opsservicev1_TriggerBackupResponse, Error, { id: number }>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.backupService.Trigger(req as never),
    ...options,
  });
}

// 备份记录列表(taskId 过滤,query 走 EQ)
export function useListBackupRecords(
  query: PaginationQuery,
  options?: Partial<UseQueryOptions<opsservicev1_ListBackupRecordResponse, Error, opsservicev1_ListBackupRecordResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listBackupRecords', query],
    queryFn: () => apiClient.backupService.ListRecords(query.toRawParams()),
    ...options,
  });
}

export async function fetchListBackupRecords(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listBackupRecords', params],
    queryFn: () => apiClient.backupService.ListRecords(params.toRawParams()),
    retry: 0,
  });
}

export function useGetBackupRecordDownloadUrl(
  options?: UseMutationOptions<opsservicev1_GetBackupRecordDownloadUrlResponse, Error, { id: number }>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.backupService.GetRecordDownloadUrl(req as never),
    ...options,
  });
}

export type { opsservicev1_BackupRecord };
