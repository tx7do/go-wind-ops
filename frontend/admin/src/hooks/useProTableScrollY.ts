import { useEffect, type RefObject } from 'react';

/**
 * 为 ProTable 的 .ant-table-body 动态设置高度，使分页器固定在容器底部。
 *
 * 核心思路（参考 CSDN 验证可行的方案）：
 *   1. scroll.y 给一个初始像素值，让 antd 创建 .ant-table-body 滚动结构
 *   2. JS 测量容器高度、分页器位置，直接操作 .ant-table-body.style.maxHeight
 *   3. 不依赖 CSS flex 穿透 antd 内部 DOM（因为 antd 内部层级不可控）
 *
 * @param containerRef - .page-container-content 容器 div 的 ref
 * @param watchValue - 容器何时真正挂载的信号。容器若不在 effect 首跑时上屏
 *   （如 Drawer destroyOnHidden：面板内容异步挂载，晚于 open 翻转后的 effect），
 *   只判空返回会永久错过；传一个随容器挂载翻转的值（如 Drawer 的 open），
 *   effect 重跑后此处轮询等容器出现再挂观察者。
 */
export function useProTableScrollY(
  containerRef: RefObject<HTMLElement | null>,
  watchValue?: unknown,
): string {
  useEffect(() => {
    // 用 setTimeout 而非 requestAnimationFrame 批量合并重算:IAB/遮挡窗口里
    // 合成器可能停帧,rAF 回调会被无限期挂起;timeout 至多被节流,不会不跑。
    let resizeTimer = 0;
    const timers: number[] = [];
    let resizeObserver: ResizeObserver | null = null;
    let mutationObserver: MutationObserver | null = null;

    const resize = () => {
      window.clearTimeout(resizeTimer);
      resizeTimer = window.setTimeout(() => {
        const container = containerRef.current;
        if (!container) return;
        const tableBody = container.querySelector('.ant-table-body') as HTMLElement | null;
        if (!tableBody) return;

        const containerHeight = container.clientHeight;
        if (containerHeight <= 0) return;

        const containerRect = container.getBoundingClientRect();
        const bodyRect = tableBody.getBoundingClientRect();

        // 表格体上方的空间（搜索栏 + 工具栏 + 表头 + 所有间距）
        const aboveSpace = bodyRect.top - containerRect.top;

        // 分页器占用的空间（高度 + margin）
        const pagination = container.querySelector('.ant-pagination') as HTMLElement | null;
        let belowSpace = 0;
        if (pagination) {
          const pagStyle = getComputedStyle(pagination);
          belowSpace =
            pagination.offsetHeight +
            (parseFloat(pagStyle.marginTop) || 0) +
            (parseFloat(pagStyle.marginBottom) || 0);
        }

        // 安全边距（给分页器留出视觉呼吸空间，确保表格底线完整可见）
        const buffer = 24;

        // 计算表格体可用高度
        const availableHeight = Math.max(containerHeight - aboveSpace - belowSpace - buffer, 100);
        tableBody.style.maxHeight = `${availableHeight}px`;
        tableBody.style.minHeight = `${availableHeight}px`;

        // 空列表时占位行只占自然高度，被 minHeight 撑高的 body 会露出大段白底。
        // body 高度是 min/max 撑出来的（height 仍为 auto），内层表格写 height:100% 解析不到，
        // 必须给显式像素值；有数据时还原，行高由内容决定。
        const bodyTable = tableBody.querySelector(':scope > table') as HTMLElement | null;
        if (bodyTable) {
          const isEmpty = !!tableBody.querySelector('.ant-table-placeholder');
          bodyTable.style.height = isEmpty ? `${availableHeight}px` : '';
        }

        // 根据内容是否溢出动态切换 overflow-y
        // antd 默认设置 overflow-y: scroll，在 Windows 上会始终显示滚动条轨道
        // 内容不超出时用 hidden 隐藏滚动条，超出时用 auto 按需显示
        if (tableBody.scrollHeight <= tableBody.clientHeight) {
          tableBody.style.overflowY = 'hidden';
        } else {
          tableBody.style.overflowY = 'auto';
        }
      }, 0);
    };

    // 容器已上屏：多次延迟重试（ProTable 异步渲染），再常驻观察后续尺寸/DOM 变化
    const attach = (container: HTMLElement) => {
      [50, 200, 500, 1500].forEach((d) => timers.push(window.setTimeout(resize, d)));

      resizeObserver = new ResizeObserver(resize);
      resizeObserver.observe(container);

      mutationObserver = new MutationObserver(resize);
      mutationObserver.observe(container, { childList: true, subtree: true });
    };

    const container = containerRef.current;
    if (container) {
      attach(container);
    } else {
      // 容器尚未上屏（Drawer 面板异步挂载）：轮询等待，最多 5s 放弃
      let waited = 0;
      const iv = window.setInterval(() => {
        const c = containerRef.current;
        if (c) {
          window.clearInterval(iv);
          attach(c);
        } else if ((waited += 100) >= 5000) {
          window.clearInterval(iv);
        }
      }, 100);
      timers.push(iv);
    }

    return () => {
      window.clearTimeout(resizeTimer);
      timers.forEach((t) => {
        window.clearTimeout(t);
        window.clearInterval(t);
      });
      resizeObserver?.disconnect();
      mutationObserver?.disconnect();
    };
  }, [containerRef, watchValue]);

  // 返回初始像素值，仅用于触发 antd 创建 .ant-table-body
  return '100px';
}
