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

export async function fetchListAgentFiles(agentId: string, dir: string): Promise<opsservicev1_ListAgentFilesResponse> {
  return apiClient.fileService.ListAgentFiles({ agentId, dir });
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

// ==============================
// 拖拽收集(含目录递归)
// ==============================

export interface DroppedEntry {
  file: File;
  /** 相对拖入根的路径(a/b/c.txt);顶层平铺文件为空串 */
  relativePath: string;
}

type FsEntry = {
  isFile: boolean;
  isDirectory: boolean;
  name: string;
  file: (cb: (f: File) => void, err?: (e: unknown) => void) => void;
  createReader: () => { readEntries: (cb: (r: FsEntry[]) => void, err?: (e: unknown) => void) => void };
};

/** 递归遍历文件系统条目;readEntries 每批最多 100 条,必须循环到空数组。 */
async function walkFsEntry(entry: FsEntry, prefix: string, out: DroppedEntry[]): Promise<void> {
  if (entry.isFile) {
    const file = await new Promise<File | null>((resolve) => {
      entry.file((f) => resolve(f), () => resolve(null));
    });
    if (file) out.push({ file, relativePath: prefix ? `${prefix}/${file.name}` : '' });
    return;
  }
  if (!entry.isDirectory) return;
  const reader = entry.createReader();
  const next = prefix ? `${prefix}/${entry.name}` : entry.name;
  for (;;) {
    const batch = await new Promise<FsEntry[]>((resolve) => {
      reader.readEntries((r) => resolve(r), () => resolve([]));
    });
    if (!batch.length) break;
    for (const child of batch) {
      await walkFsEntry(child, next, out);
    }
  }
}

/**
 * 收集 drop 事件的数据项为待上传清单:顶层文件平铺,目录递归出全部文件并
 * 保留相对路径(agent 落盘会自动 MkdirAll 父目录)。webkitGetAsEntry/getAsFile
 * 必须在 drop 事件同步调用栈内执行,调用方须直接把 event.dataTransfer 传入。
 */
export async function collectDroppedEntries(dt: DataTransfer): Promise<{ entries: DroppedEntry[]; dirs: number }> {
  const entries: DroppedEntry[] = [];
  let dirs = 0;
  const roots: FsEntry[] = [];
  for (const item of Array.from(dt.items || [])) {
    // 新版 lib.dom 原生声明了 webkitGetAsEntry(返回 FileSystemEntry),与这里的
    // 窄化 FsEntry 交叉后会得到联合类型,须经 unknown 强转拿回自己的签名。
    // 新版 lib.dom 已原生声明 webkitGetAsEntry(返回 FileSystemEntry),窄化交叉后
    // 联合类型须经 unknown 强转拿回自己的签名。
    const entry = (item as unknown as { webkitGetAsEntry?: () => FsEntry | null }).webkitGetAsEntry?.() as FsEntry | null;
    if (entry) {
      if (entry.isDirectory) dirs += 1;
      roots.push(entry);
      continue;
    }
    const f = item.getAsFile();
    if (f) entries.push({ file: f, relativePath: '' });
  }
  for (const root of roots) {
    await walkFsEntry(root, '', entries);
  }
  return { entries, dirs };
}

export function useDeleteAgentFile(
  options?: Partial<UseMutationOptions<opsservicev1_DeleteAgentFileResponse, Error, { agentId: string; path: string }>>,
) {
  return useMutation({
    mutationFn: (req) => apiClient.fileService.DeleteAgentFile(req),
    ...options,
  });
}
