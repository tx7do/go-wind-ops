import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { App, Breadcrumb, Button, Descriptions, Drawer, Input, Popconfirm, Select, Space, Spin, Table, Tag, Tooltip, Typography } from 'antd';
import {
  DeleteOutlined,
  DownloadOutlined,
  EditOutlined,
  EyeOutlined,
  FolderOpenOutlined,
  ReloadOutlined,
  FileAddOutlined,
  UploadOutlined,
} from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import type { opsservicev1_AgentFileEntry as FileEntry } from '@/api/generated/admin/service/v1';
import { PaginationQuery } from '@/core';
import { fetchListAgentNodes } from '@/api/hooks/agent-node';
import {
  fetchCompleteAgentFilePush,
  fetchPrepareAgentFilePush,
  fetchPullAgentFile,
  putFileToPresignedUrl,
  useDeleteAgentFile,
  useListAgentFiles,
  useReadAgentFile,
  useWriteAgentFile,
} from '@/api/hooks/file';

/** Unicode 安全的 base64 编解码(TextEncoder/Decoder 中转)。 */
function utf8ToBase64(s: string): string {
  const bytes = new TextEncoder().encode(s);
  let bin = '';
  bytes.forEach((b) => { bin += String.fromCharCode(b); });
  return btoa(bin);
}
function base64ToUtf8(s: string): string {
  const bin = atob(s);
  const bytes = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
  return new TextDecoder().decode(bytes);
}

/** 按平台风格拼接路径段(纯展示用,实际路径由 agent 校验)。 */
function joinPath(dir: string, name: string): string {
  return dir.endsWith('/') || dir.endsWith('\\') ? dir + name : `${dir}/${name}`;
}

type EditorState = { path: string; content: string; truncated: boolean; loading: boolean };

const FileManager = () => {
  const { t } = useTranslation('file');
  const { message } = App.useApp();

  const [agentId, setAgentId] = useState<string>();
  const [dirInput, setDirInput] = useState('');
  const [currentDir, setCurrentDir] = useState('');
  const [editor, setEditor] = useState<EditorState>();
  const [newFile, setNewFile] = useState<{ open: boolean; path: string }>({ open: false, path: '' });

  // agent 下拉(一次拉满,前端过滤)
  const { data: nodesResp } = useQuery({
    queryKey: ['listAgentNodesForFile', 'all'],
    queryFn: () => fetchListAgentNodes(new PaginationQuery({ paging: { page: 1, pageSize: 200 } })),
  });
  const agentOptions = useMemo(
    () => (nodesResp?.items || []).map((n) => ({ label: n.hostname ? `${n.hostname} (${n.agentId})` : n.agentId, value: n.agentId || '' })),
    [nodesResp],
  );

  // 目录浏览:currentDir 仅在点击“浏览”/回车/点击面包屑时变化,不会边输入边触发 MQTT 往返
  const listing = useListAgentFiles(
    { agentId, dir: currentDir },
    { enabled: Boolean(agentId && currentDir), placeholderData: (prev) => prev },
  );
  const entries: FileEntry[] = useMemo(() => listing.data?.entries || [], [listing.data]);

  const refresh = useCallback(() => listing.refetch(), [listing]);

  const readMutation = useReadAgentFile({
    onSuccess: (resp) => {
      setEditor({
        path: resp.path || '',
        content: resp.contentBase64 ? base64ToUtf8(resp.contentBase64) : '',
        truncated: Boolean(resp.truncated),
        loading: false,
      });
    },
    onError: (e: Error) => message.error(e.message || t('readFailed')),
  });
  const writeMutation = useWriteAgentFile({
    onSuccess: (resp) => {
      message.success(t('writeSuccess', { size: resp.size ?? 0 }));
      setEditor(undefined);
      setNewFile({ open: false, path: '' });
      refresh();
    },
    onError: (e: Error) => message.error(e.message || t('writeFailed')),
  });
  // 大文件下载:agent 经对象存储中转,拿预签名 URL 后开新窗口
  const [, setPulling] = useState(false);
  const downloadFile = async (path: string) => {
    if (!agentId) return;
    setPulling(true);
    try {
      const resp = await fetchPullAgentFile(agentId, path);
      window.open(resp.url, '_blank');
      message.success(t('pullReady', { size: resp.size ?? 0 }));
    } catch (e) {
      message.error((e as Error).message || t('pullFailed'));
    } finally {
      setPulling(false);
    }
  };

  // 大文件上传:直传对象存储 → 指令 agent 落盘
  const uploadInputRef = useRef<HTMLInputElement>(null);
  const rootRef = useRef<HTMLDivElement>(null);
  const [uploading, setUploading] = useState(false);
  const [dropActive, setDropActive] = useState(false);
  const onUploadPicked = async (file: File): Promise<boolean> => {
    if (!agentId || !currentDir) {
      message.warning(t('pickAgentFirst'));
      return false;
    }
    const targetPath = joinPath(currentDir, file.name);
    setUploading(true);
    try {
      const prep = await fetchPrepareAgentFilePush(agentId, targetPath);
      await putFileToPresignedUrl(prep.uploadUrl || '', file);
      const done = await fetchCompleteAgentFilePush(agentId, targetPath, prep.objectKey || '');
      message.success(t('uploadDone', { size: done.size ?? 0 }));
      refresh();
      return true;
    } catch (e) {
      message.error((e as Error).message || t('uploadFailed'));
      return false;
    } finally {
      setUploading(false);
      if (uploadInputRef.current) uploadInputRef.current.value = '';
    }
  };

  // 整页拖拽=上传到当前目录(与选择文件按钮同一链路);目录项第一版不支持
  const onUploadPickedRef = useRef(onUploadPicked);
  onUploadPickedRef.current = onUploadPicked;
  useEffect(() => {
    const host = rootRef.current;
    if (!host) return;
    const hasFiles = (e: DragEvent) => Array.from(e.dataTransfer?.types || []).includes('Files');
    const onDragOver = (e: DragEvent) => {
      if (!hasFiles(e)) return;
      e.preventDefault();
      setDropActive(true);
    };
    const onDragLeave = (e: DragEvent) => {
      if (e.relatedTarget && host.contains(e.relatedTarget as Node)) return;
      setDropActive(false);
    };
    const onDrop = async (e: DragEvent) => {
      if (!hasFiles(e)) return;
      e.preventDefault();
      setDropActive(false);
      const files: File[] = [];
      let dirs = 0;
      for (const item of Array.from(e.dataTransfer?.items || [])) {
        const entry = (
          item as DataTransferItem & { webkitGetAsEntry?: () => { isDirectory: boolean } | null }
        ).webkitGetAsEntry?.();
        if (entry?.isDirectory) {
          dirs += 1;
          continue;
        }
        const f = item.getAsFile();
        if (f) files.push(f);
      }
      if (dirs > 0) message.warning(t('folderSkip'));
      if (!files.length) return;
      if (!agentId || !currentDir) {
        message.warning(t('pickAgentFirst'));
        return;
      }
      let ok = 0;
      for (const f of files) {
        if (await onUploadPickedRef.current(f)) ok += 1;
      }
      if (files.length > 1) message.info(t('dropBatchDone', { ok, total: files.length }));
    };
    host.addEventListener('dragover', onDragOver);
    host.addEventListener('dragleave', onDragLeave);
    host.addEventListener('drop', onDrop);
    return () => {
      host.removeEventListener('dragover', onDragOver);
      host.removeEventListener('dragleave', onDragLeave);
      host.removeEventListener('drop', onDrop);
    };
  }, [agentId, currentDir, message, t]);

  const deleteMutation = useDeleteAgentFile({
    onSuccess: () => {
      message.success(t('deleteSuccess'));
      refresh();
    },
    onError: (e: Error) => message.error(e.message || t('deleteFailed')),
  });

  const openDir = (d: string) => { setCurrentDir(d); setDirInput(d); };

  const breadcrumbs = useMemo(() => {
    if (!currentDir) return [];
    const parts = currentDir.split(/[\\/]+/).filter(Boolean);
    const crumbs: { label: string; path: string }[] = [];
    let acc = '';
    parts.forEach((p, i) => {
      acc = i === 0 ? p : `${acc}/${p}`;
      crumbs.push({ label: p, path: acc });
    });
    return crumbs;
  }, [currentDir]);

  const columns = [
    {
      title: t('name'),
      dataIndex: 'name',
      render: (_: unknown, r: FileEntry) => (
        <a onClick={() => (r.isDir ? openDir(joinPath(currentDir, r.name || '')) : readMutation.mutate({ agentId: agentId!, path: joinPath(currentDir, r.name || '') }))}>
          {r.isDir ? <FolderOpenOutlined style={{ marginRight: 6, color: 'var(--ant-color-warning)' }} /> : <EyeOutlined style={{ marginRight: 6 }} />}
          {r.name}
        </a>
      ),
    },
    {
      title: t('type'),
      dataIndex: 'isDir',
      width: 100,
      render: (_: unknown, r: FileEntry) => (r.isDir ? <Tag color="gold">{t('typeDir')}</Tag> : <Tag>{t('typeFile')}</Tag>),
    },
    {
      title: t('size'),
      dataIndex: 'size',
      width: 130,
      render: (_: unknown, r: FileEntry) => (r.isDir ? '-' : `${r.size ?? 0} B`),
    },
    {
      title: t('action'),
      key: 'action',
      width: 160,
      render: (_: unknown, r: FileEntry) => {
        const p = joinPath(currentDir, r.name || '');
        return (
          <Space>
            {r.isDir ? (
              <a onClick={() => openDir(p)}><FolderOpenOutlined /> {t('open')}</a>
            ) : (
              <>
                <a onClick={() => readMutation.mutate({ agentId: agentId!, path: p })}><EditOutlined /> {t('edit')}</a>
                <a onClick={() => downloadFile(p)}><DownloadOutlined /> {t('download')}</a>
              </>
            )}
            <Popconfirm
              title={t('deleteConfirm')}
              onConfirm={() => deleteMutation.mutate({ agentId: agentId!, path: p })}
              okText={t('common:button.ok')}
              cancelText={t('common:button.cancel')}
            >
              <a style={{ color: 'var(--ant-color-error)' }}><DeleteOutlined /> {t('delete')}</a>
            </Popconfirm>
          </Space>
        );
      },
    },
  ];

  const canBrowse = Boolean(agentId && currentDir);

  return (
    <div ref={rootRef} style={{ padding: 16, position: 'relative' }}>
      {dropActive && (
        <div
          style={{
            position: 'absolute',
            inset: 0,
            borderRadius: 8,
            border: '2px dashed #1668dc',
            background: 'rgba(22, 104, 220, 0.08)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 20,
            pointerEvents: 'none',
            color: '#4096ff',
            fontSize: 18,
          }}
        >
          {t('dropHere')}
        </div>
      )}
      <Space direction="vertical" style={{ width: '100%' }} size="middle">
        <Space wrap>
          <Select
            showSearch
            optionFilterProp="label"
            placeholder={t('selectAgent')}
            style={{ minWidth: 260 }}
            value={agentId}
            options={agentOptions}
            onChange={(v) => { setAgentId(v); setCurrentDir(''); setDirInput(''); setEditor(undefined); }}
          />
          <Input
            style={{ width: 380 }}
            placeholder={t('dirPlaceholder')}
            value={dirInput}
            onChange={(e) => setDirInput(e.target.value)}
            onPressEnter={() => openDir(dirInput.trim())}
            disabled={!agentId}
          />
          <Button type="primary" onClick={() => openDir(dirInput.trim())} disabled={!agentId || !dirInput.trim()}>
            <FolderOpenOutlined /> {t('browse')}
          </Button>
          <Button onClick={refresh} disabled={!canBrowse} loading={listing.isFetching}>
            <ReloadOutlined /> {t('refresh')}
          </Button>
          <Button type="primary" ghost disabled={!canBrowse} onClick={() => setNewFile({ open: true, path: currentDir.endsWith('/') ? currentDir : currentDir + '/' })}>
            <FileAddOutlined /> {t('newFile')}
          </Button>
          <Button disabled={!canBrowse} loading={uploading} onClick={() => uploadInputRef.current?.click()}>
            <UploadOutlined /> {t('upload')}
          </Button>
          <input
            ref={uploadInputRef}
            type="file"
            style={{ display: 'none' }}
            onChange={(e) => {
              const f = e.target.files?.[0];
              if (f) void onUploadPicked(f);
            }}
          />
        </Space>

        {canBrowse && (
          <Breadcrumb
            items={[
              { title: <a onClick={() => openDir('')}>{t('root')}</a> },
              ...breadcrumbs.map((c, i) => ({
                title: i === breadcrumbs.length - 1
                  ? c.label
                  : <a onClick={() => openDir(c.path)}>{c.label}</a>,
              })),
            ]}
          />
        )}

        <Table
          rowKey="name"
          size="small"
          columns={columns as never}
          dataSource={entries}
          loading={listing.isFetching && !canBrowse ? false : listing.isFetching}
          pagination={false}
          locale={{ emptyText: canBrowse ? (listing.isFetching ? <Spin /> : t('emptyDir')) : t('pickAgentFirst') }}
        />
      </Space>

      <Drawer
        title={<span>{t('editTitle')}: <Typography.Text code>{editor?.path}</Typography.Text></span>}
        size={720}
        open={Boolean(editor)}
        onClose={() => setEditor(undefined)}
        extra={
          <Space>
            {editor?.truncated && <Tooltip title={t('truncatedHint')}><Tag color="warning">{t('truncated')}</Tag></Tooltip>}
            <Button onClick={() => setEditor(undefined)}>{t('common:button.cancel')}</Button>
            <Button
              type="primary"
              loading={writeMutation.isPending}
              disabled={editor?.truncated}
              onClick={() => agentId && editor && writeMutation.mutate({ agentId, path: editor.path, contentBase64: utf8ToBase64(editor.content) })}
            >
              {t('save')}
            </Button>
          </Space>
        }
      >
        {editor && (
          <Input.TextArea
            value={editor.content}
            onChange={(e) => setEditor({ ...editor, content: e.target.value })}
            rows={28}
            styles={{ textarea: { fontFamily: 'monospace', fontSize: 13 } }}
          />
        )}
      </Drawer>

      <Drawer
        title={t('newFileTitle')}
        size={720}
        open={newFile.open}
        onClose={() => setNewFile({ open: false, path: '' })}
        extra={
          <Space>
            <Button onClick={() => setNewFile({ open: false, path: '' })}>{t('common:button.cancel')}</Button>
            <Button
              type="primary"
              loading={writeMutation.isPending}
              disabled={!newFile.path.trim()}
              onClick={() => agentId && writeMutation.mutate({ agentId, path: newFile.path.trim(), contentBase64: utf8ToBase64('') })}
            >
              {t('create')}
            </Button>
          </Space>
        }
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <Typography.Text type="secondary">{t('newFileHint')}</Typography.Text>
          <Input value={newFile.path} onChange={(e) => setNewFile({ ...newFile, path: e.target.value })} />
          <Descriptions size="small" column={1}>
            <Descriptions.Item label={t('whitelistHint')}>{t('whitelistDesc')}</Descriptions.Item>
          </Descriptions>
        </Space>
      </Drawer>
    </div>
  );
};

export default FileManager;
