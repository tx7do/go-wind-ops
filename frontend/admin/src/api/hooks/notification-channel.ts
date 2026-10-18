import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_DeleteNotificationChannelRequest,
  type opsservicev1_ListNotificationChannelResponse,
  type opsservicev1_NotificationChannel,
} from '@/api/generated/admin/service/v1';
import { makeUpdateMask, type PaginationQuery } from '@/core';
import { apiClient } from '@/api/client';

export function useListNotificationChannels(
  query: PaginationQuery,
  options?: Partial<UseQueryOptions<opsservicev1_ListNotificationChannelResponse, Error, opsservicev1_ListNotificationChannelResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listNotificationChannels', query],
    queryFn: () => apiClient.notificationChannelService.List(query.toRawParams()),
    ...options,
  });
}

export function useCreateNotificationChannel(
  options?: UseMutationOptions<opsservicev1_NotificationChannel, Error, Partial<opsservicev1_NotificationChannel>>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.notificationChannelService.Create({ data: data as opsservicev1_NotificationChannel }),
    ...options,
  });
}

export function useUpdateNotificationChannel(
  options?: UseMutationOptions<Record<never, never>, Error, { id: number; values: Record<string, any> }>,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) =>
      apiClient.notificationChannelService.Update({
        id,
        data: { ...values } as opsservicev1_NotificationChannel,
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeleteNotificationChannel(
  options?: UseMutationOptions<Record<never, never>, Error, opsservicev1_DeleteNotificationChannelRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.notificationChannelService.Delete(req),
    ...options,
  });
}
