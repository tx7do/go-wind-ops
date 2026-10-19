import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_Cert,
  type opsservicev1_ListCertResponse,
  type opsservicev1_ProbeCertResponse,
} from '@/api/generated/admin/service/v1';
import { makeUpdateMask, PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 证书到期台账
// ==============================

export function useListCerts(
  query: PaginationQuery,
  options?: Partial<UseQueryOptions<opsservicev1_ListCertResponse, Error, opsservicev1_ListCertResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listCerts', query],
    queryFn: () => apiClient.certService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListCerts(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listCerts', params],
    queryFn: () => apiClient.certService.List(params.toRawParams()),
    retry: 0,
  });
}

export function useCreateCert(
  options?: UseMutationOptions<opsservicev1_Cert, Error, Partial<opsservicev1_Cert>>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.certService.Create({ data: data as opsservicev1_Cert }),
    ...options,
  });
}

export function useUpdateCert(
  options?: UseMutationOptions<Record<never, never>, Error, { id: number; values: Record<string, any> }>,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) =>
      apiClient.certService.Update({
        id,
        data: { ...values } as opsservicev1_Cert,
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeleteCert(
  options?: UseMutationOptions<Record<never, never>, Error, { id: number }>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.certService.Delete(req as never),
    ...options,
  });
}

export function useProbeCert(
  options?: UseMutationOptions<opsservicev1_ProbeCertResponse, Error, { id: number }>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.certService.Probe(req as never),
    ...options,
  });
}
