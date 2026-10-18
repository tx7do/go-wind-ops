import {
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_CommandRecord,
  type opsservicev1_GetCommandRecordRequest,
  type opsservicev1_ListCommandRecordResponse,
} from '@/api/generated/admin/service/v1';
import type { PaginationQuery } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 指令台账(只读)
// ==============================

export function useListCommandRecords(
  query: PaginationQuery,
  options?: Partial<UseQueryOptions<opsservicev1_ListCommandRecordResponse, Error, opsservicev1_ListCommandRecordResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listCommandRecords', query],
    queryFn: () => apiClient.commandRecordService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListCommandRecords(params: PaginationQuery) {
  return apiClient.commandRecordService.List(params.toRawParams());
}

export function useGetCommandRecord(
  req: opsservicev1_GetCommandRecordRequest,
  options?: Partial<UseQueryOptions<opsservicev1_CommandRecord, Error, opsservicev1_CommandRecord, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['getCommandRecord', req],
    queryFn: () => apiClient.commandRecordService.Get(req),
    ...options,
  });
}
