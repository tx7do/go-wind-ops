import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_ListApprovalResponse,
  type opsservicev1_ResolveAlertRequest,
} from '@/api/generated/admin/service/v1';
import type { PaginationQuery } from '@/core';
import { apiClient } from '@/api/client';

export function useListApprovals(
  query: PaginationQuery,
  options?: Partial<UseQueryOptions<opsservicev1_ListApprovalResponse, Error, opsservicev1_ListApprovalResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listApprovals', query],
    queryFn: () => apiClient.approvalService.List(query.toRawParams()),
    ...options,
  });
}

export function useApproveApproval(
  options?: UseMutationOptions<Record<never, never>, Error, opsservicev1_ResolveAlertRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.approvalService.Approve(req),
    ...options,
  });
}

export function useRejectApproval(
  options?: UseMutationOptions<Record<never, never>, Error, opsservicev1_ResolveAlertRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.approvalService.Reject(req),
    ...options,
  });
}

export async function fetchListApprovals(params: PaginationQuery) {
  return apiClient.approvalService.List(params.toRawParams());
}
