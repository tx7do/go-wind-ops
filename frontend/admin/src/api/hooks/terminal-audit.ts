import {
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_ListTerminalChunkResponse,
  type opsservicev1_ListTerminalSessionResponse,
  type opsservicev1_TerminalSession,
} from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 终端会话审计
// ==============================

export function useListTerminalSessions(
  query: PaginationQuery,
  options?: Partial<UseQueryOptions<opsservicev1_ListTerminalSessionResponse, Error, opsservicev1_ListTerminalSessionResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listTerminalSessions', query],
    queryFn: () => apiClient.terminalAuditService.List(query.toRawParams()),
    ...options,
  });
}

export function useGetTerminalSession(
  id?: number,
  options?: Partial<UseQueryOptions<opsservicev1_TerminalSession, Error, opsservicev1_TerminalSession, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['getTerminalSession', id],
    queryFn: () => apiClient.terminalAuditService.Get({ id } as never),
    enabled: !!id,
    ...options,
  });
}

export function useListTerminalChunks(
  sessionId?: number,
  options?: Partial<UseQueryOptions<opsservicev1_ListTerminalChunkResponse, Error, opsservicev1_ListTerminalChunkResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listTerminalChunks', sessionId],
    queryFn: () => apiClient.terminalAuditService.ListChunks({ sessionId, limit: 2000 } as never),
    enabled: !!sessionId,
    ...options,
  });
}

export async function fetchListTerminalSessions(params: PaginationQuery) {
  return apiClient.terminalAuditService.List(params.toRawParams());
}
