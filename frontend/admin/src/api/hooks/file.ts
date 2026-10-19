import {
  useMutation,
  useQuery,
  type UseMutationOptions,
  type UseQueryOptions,
} from '@tanstack/react-query';
import {
  type opsservicev1_CompleteAgentFilePushResponse,
  type opsservicev1_DeleteAgentFileResponse,
  type opsservicev1_PrepareAgentFilePushResponse,
  type opsservicev1_PullAgentFileResponse,
  type opsservicev1_ListAgentFilesResponse,
  type opsservicev1_ReadAgentFileResponse,
  type opsservicev1_WriteAgentFileResponse,
} from '@/api/generated/admin/service/v1';
import { apiClient } from '@/api/client';

// ==============================
// 远端文件管理(经 MQTT 代理 agent)
// ==============================

export function useListAgentFiles(
  req: { agentId?: string; dir?: string },
  options?: Partial<UseQueryOptions<opsservicev1_ListAgentFilesResponse, Error, opsservicev1_ListAgentFilesResponse, readonly unknown[]>>,
) {
  return useQuery({
    queryKey: ['listAgentFiles', req.agentId, req.dir],
    queryFn: () => apiClient.fileService.ListAgentFiles(req),
    enabled: options?.enabled ?? Boolean(req.agentId && req.dir),
    ...options,
  });
}

export function useReadAgentFile(
  options?: Partial<UseMutationOptions<opsservicev1_ReadAgentFileResponse, Error, { agentId: string; path: string }>>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.fileService.ReadAgentFile(req),
    ...options,
  });
}

export function useWriteAgentFile(
  options?: Partial<UseMutationOptions<opsservicev1_WriteAgentFileResponse, Error, { agentId: string; path: string; contentBase64: string }>>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.fileService.WriteAgentFile(req),
    ...options,
  });
}

export async function fetchPullAgentFile(agentId: string, path: string): Promise<opsservicev1_PullAgentFileResponse> {
  return apiClient.fileService.PullAgentFile({ agentId, path });
}

export async function fetchPrepareAgentFilePush(agentId: string, path: string): Promise<opsservicev1_PrepareAgentFilePushResponse> {
  return apiClient.fileService.PrepareAgentFilePush({ agentId, path });
}

export async function fetchCompleteAgentFilePush(agentId: string, path: string, objectKey: string): Promise<opsservicev1_CompleteAgentFilePushResponse> {
  return apiClient.fileService.CompleteAgentFilePush({ agentId, path, objectKey });
}

/** 浏览器直传对象存储(预签名 PUT);传 onProgress 时走 XHR 上报百分比。 */
export async function putFileToPresignedUrl(
  url: string,
  file: File,
  onProgress?: (percent: number) => void,
): Promise<void> {
  if (!onProgress) {
    const resp = await fetch(url, { method: 'PUT', body: file });
    if (!resp.ok) {
      throw new Error(`upload failed: HTTP ${resp.status}`);
    }
    return;
  }
  await new Promise<void>((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open('PUT', url);
    xhr.upload.onprogress = (ev) => {
      if (ev.lengthComputable) onProgress(Math.round((ev.loaded / ev.total) * 100));
    };
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) resolve();
      else reject(new Error(`upload failed: HTTP ${xhr.status}`));
    };
    xhr.onerror = () => reject(new Error('upload network error'));
    xhr.send(file);
  });
}

/** 字节数人性化显示(终端上传回显/提示用)。 */
export function formatBytes(n?: number): string {
  if (!n || n <= 0) return '0B';
  if (n < 1024) return `${n}B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)}KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)}MB`;
  return `${(n / 1024 / 1024 / 1024).toFixed(2)}GB`;
}

/** 远端路径拼接(分隔符跟随目录写法,win/posix 通用)。 */
export function joinRemotePath(dir: string, name: string): string {
  if (!dir) return name;
  const sep = dir.includes('\\') ? '\\' : '/';
  return dir.endsWith(sep) ? `${dir}${name}` : `${dir}${sep}${name}`;
}

export interface UploadAgentFileOptions {
  agentId: string;
  /** 远端绝对目标路径(含文件名) */
  path: string;
  file: File;
  onProgress?: (percent: number) => void;
}

/** 上传文件到 agent(预签名三步链:prepare → 浏览器直传 MinIO → complete 落盘)。 */
export async function uploadAgentFile({
  agentId,
  path,
  file,
  onProgress,
}: UploadAgentFileOptions): Promise<opsservicev1_CompleteAgentFilePushResponse> {
  const prep = await fetchPrepareAgentFilePush(agentId, path);
  await putFileToPresignedUrl(prep.uploadUrl || '', file, onProgress);
  return fetchCompleteAgentFilePush(agentId, path, prep.objectKey || '');
}

export function useDeleteAgentFile(
  options?: Partial<UseMutationOptions<opsservicev1_DeleteAgentFileResponse, Error, { agentId: string; path: string }>>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.fileService.DeleteAgentFile(req),
    ...options,
  });
}
