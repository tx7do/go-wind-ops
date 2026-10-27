import {

  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_DeleteScriptRequest,
  type opsservicev1_ExecuteScriptRequest,
  type opsservicev1_ExecuteScriptResponse,
  type opsservicev1_GetScriptRequest,
  type opsservicev1_ListScriptResponse,
  type opsservicev1_Script,
  type opsservicev1_ListScriptVersionsResponse,
} from '@/api/generated/admin/service/v1';
import { makeUpdateMask, type PaginationQuery, queryClient } from '@/core';
import { apiClient } from '@/api/client';

// ==============================
// 危险命令确认契约(与 core script_dangerous.go 对齐)
// ==============================

/** 后端拦截危险命令时的专用 reason(message 会被 i18n 覆盖,不可作契约)。 */
export const DANGEROUS_COMMAND_REASON = 'DANGEROUS_COMMAND_CONFIRM';

/** 判定错误是否为"危险命令需确认"拦截(onError 里据此弹二次确认并重发)。 */
export function isDangerousConfirmError(e: unknown): boolean {
  return Boolean(
    e &&
      typeof e === 'object' &&
      (e as { reason?: unknown }).reason === DANGEROUS_COMMAND_REASON,
  );
}

// ==============================
// 脚本管理
// ==============================

export function useListScripts(
  query: PaginationQuery,
  options?: Partial<UseQueryOptions<opsservicev1_ListScriptResponse, Error, opsservicev1_ListScriptResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listScripts', query],
    queryFn: () => apiClient.scriptService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListScripts(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listScripts', params],
    queryFn: () => apiClient.scriptService.List(params.toRawParams()),
    retry: 0,
  });
}

export function useGetScript(
  req: opsservicev1_GetScriptRequest,
  options?: Partial<UseQueryOptions<opsservicev1_Script, Error, opsservicev1_Script, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['getScript', req],
    queryFn: () => apiClient.scriptService.Get(req),
    ...options,
  });
}

export function useCreateScript(
  options?: UseMutationOptions<opsservicev1_Script, Error, Partial<opsservicev1_Script>>,
) {
  return useMutation({
    mutationFn: (data) => apiClient.scriptService.Create({ data: data as opsservicev1_Script }),
    ...options,
  });
}

export function useUpdateScript(
  options?: UseMutationOptions<{}, Error, { id: number; values: Record<string, any> }>,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) =>
      apiClient.scriptService.Update({
        id,
        data: { ...values } as opsservicev1_Script,
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeleteScript(
  options?: UseMutationOptions<{}, Error, opsservicev1_DeleteScriptRequest>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.scriptService.Delete(req),
    ...options,
  });
}

export function useExecuteScript(
  options?: UseMutationOptions<opsservicev1_ExecuteScriptResponse, Error, Partial<opsservicev1_ExecuteScriptRequest>>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.scriptService.Execute(req as never),
    ...options,
  });
}

export function useListScriptVersions(
  scriptId: number | undefined,
  options?: Partial<UseQueryOptions<opsservicev1_ListScriptVersionsResponse, Error, opsservicev1_ListScriptVersionsResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['scriptVersions', scriptId],
    queryFn: () => apiClient.scriptService.ListScriptVersions({ scriptId } as never),
    enabled: !!scriptId,
    ...options,
  });
}

export function useRestoreScriptVersion(
  options?: UseMutationOptions<void, Error, { scriptId: number; versionId: number }>,
) {
  return useMutation({
    mutationFn: async (req) => {
      await apiClient.scriptService.RestoreScriptVersion(req as never);
    },
    ...options,
  });
}
