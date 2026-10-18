import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_ClearProjectLogRequest,
  type opsservicev1_DeleteProjectRequest,
  type opsservicev1_GetProjectLogRequest,
  type opsservicev1_GetProjectLogResponse,
  type opsservicev1_GetProjectRequest,
  type opsservicev1_ListProjectResponse,
  type opsservicev1_OperateProjectRequest,
  type opsservicev1_OperateProjectResponse,
  type opsservicev1_Project,
} from '@/api/generated/admin/service/v1';
import { makeUpdateMask, type PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 项目管理
// ==============================

export function useListProjects(
  query: PaginationQuery,
  options?: Partial<UseQueryOptions<opsservicev1_ListProjectResponse, Error, opsservicev1_ListProjectResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listProjects', query],
    queryFn: () => apiClient.projectService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListProjects(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listProjects', params],
    queryFn: () => apiClient.projectService.List(params.toRawParams()),
    retry: 0,
  });
}

export function useGetProject(
  req: opsservicev1_GetProjectRequest,
  options?: Partial<UseQueryOptions<opsservicev1_Project, Error, opsservicev1_Project, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['getProject', req],
    queryFn: () => apiClient.projectService.Get(req),
    ...options,
  });
}

export function useCreateProject(
  options?: UseMutationOptions<opsservicev1_Project, Error, Partial<opsservicev1_Project>>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.projectService.Create({ data: data as opsservicev1_Project }),
    ...options,
  });
}

export function useUpdateProject(
  options?: UseMutationOptions<{}, Error, { id: number; values: Record<string, any> }>,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) =>
      apiClient.projectService.Update({
        id,
        data: { ...values } as opsservicev1_Project,
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeleteProject(
  options?: UseMutationOptions<{}, Error, opsservicev1_DeleteProjectRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.projectService.Delete(req),
    ...options,
  });
}

export function useOperateProject(
  options?: UseMutationOptions<opsservicev1_OperateProjectResponse, Error, opsservicev1_OperateProjectRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.projectService.Operate(req),
    ...options,
  });
}

export function useGetProjectLog(
  req: opsservicev1_GetProjectLogRequest,
  options?: Partial<UseQueryOptions<opsservicev1_GetProjectLogResponse, Error, opsservicev1_GetProjectLogResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['getProjectLog', req],
    queryFn: () => apiClient.projectService.GetLog(req),
    ...options,
  });
}

export function useClearProjectLog(
  options?: UseMutationOptions<{}, Error, opsservicev1_ClearProjectLogRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.projectService.ClearLog(req),
    ...options,
  });
}
