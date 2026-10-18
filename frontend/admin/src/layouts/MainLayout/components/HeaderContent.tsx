import { useEffect, useMemo, useState } from 'react';
import React from 'react';
import { Avatar, Dropdown, Tooltip, Button, Breadcrumb, Input, Modal } from 'antd';
import type { MenuProps } from 'antd';
import {
  UserOutlined,
  SettingOutlined,
  LogoutOutlined,
  MenuUnfoldOutlined,
  MenuFoldOutlined,
  ReloadOutlined,
  SearchOutlined,
  SunOutlined,
  MoonOutlined,
  GlobalOutlined,
  FullscreenOutlined,
  FullscreenExitOutlined,
  CloseOutlined,
} from '@ant-design/icons';
import { useMatches, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';

import { getIconFromName } from '@/layouts/MainLayout/utils/iconResolver';

import { useI18n } from '@/core/i18n';
import { usePreferencesStore } from '@/core/preferences/store';
import type { SupportedLanguagesType } from '@/core/preferences/types/layout';


/** 键位提示条的小 kbd 徽标（与触发条上的 Ctrl K 同族样式） */
const searchHintKbdStyle: React.CSSProperties = {
  display: 'inline-flex',
  alignItems: 'center',
  justifyContent: 'center',
  minWidth: 18,
  padding: '1px 5px',
  fontSize: 11,
  fontFamily: 'monospace',
  lineHeight: 1.4,
  color: 'var(--ant-color-text-secondary)',
  backgroundColor: 'var(--ant-color-bg-container)',
  border: '1px solid var(--ant-color-border)',
  borderRadius: 4,
};

/** 全局搜索结果行（形态对齐 vben SearchPanel：图标 + 标题，选中主色实底白字） */
const SearchRow = ({
  iconNode,
  title,
  active,
  onMouseEnter,
  onClick,
  onRemove,
}: {
  iconNode?: React.ReactNode;
  title: string;
  active: boolean;
  onMouseEnter: () => void;
  onClick: () => void;
  onRemove?: () => void;
}) => (
  <div
    className="search-item"
    onMouseEnter={onMouseEnter}
    onClick={onClick}
    style={{
      display: 'flex',
      alignItems: 'center',
      gap: 10,
      padding: '8px 10px',
      borderRadius: 6,
      cursor: 'pointer',
      ...(active ? { backgroundColor: 'var(--ant-color-primary)' } : {}),
    }}
  >
    {iconNode && (
      <span
        style={{
          display: 'inline-flex',
          fontSize: 14,
          flexShrink: 0,
          color: active ? 'var(--ant-color-white)' : 'var(--ant-color-text-secondary)',
        }}
      >
        {iconNode}
      </span>
    )}
    <span
      style={{
        flex: 1,
        minWidth: 0,
        overflow: 'hidden',
        textOverflow: 'ellipsis',
        whiteSpace: 'nowrap',
        fontSize: 13,
        ...(active ? { color: 'var(--ant-color-white)' } : {}),
      }}
    >
      {title}
    </span>
    {onRemove && (
      <span
        role="button"
        tabIndex={-1}
        onClick={(e) => {
          e.stopPropagation();
          onRemove();
        }}
        style={{
          display: 'inline-flex',
          padding: 3,
          fontSize: 11,
          lineHeight: 1,
          borderRadius: 4,
          color: active ? 'var(--ant-color-white)' : 'var(--ant-color-text-secondary)',
        }}
        onMouseEnter={(e) => {
          e.currentTarget.style.backgroundColor = active
            ? 'color-mix(in srgb, var(--ant-color-white) 20%, transparent)'
            : 'var(--ant-color-fill-tertiary)';
        }}
        onMouseLeave={(e) => {
          e.currentTarget.style.backgroundColor = 'transparent';
        }}
      >
        <CloseOutlined />
      </span>
    )}
  </div>
);

interface HeaderContentProps {
  userInfo: BasicUserInfo | null;
  collapsed: boolean;
  isFullscreen: boolean;
  onToggleCollapse: () => void;
  onRefresh: () => void;
  onToggleFullscreen: () => void;
  onLogout: () => void;
  isDark: boolean;
  onToggleTheme: (event?: React.MouseEvent<HTMLElement>) => void;
  onOpenSettings: () => void;
  menuData: any[];
  widgetConfig: {
    fullscreen: boolean;
    globalSearch: boolean;
    languageToggle: boolean;
    notification: boolean;
    themeToggle: boolean;
    refresh: boolean;
    sidebarToggle: boolean;
  };
}

export const HeaderContent = ({
  userInfo,
  collapsed,
  isFullscreen,
  onToggleCollapse,
  onRefresh,
  onToggleFullscreen,
  onLogout,
  isDark,
  onToggleTheme,
  onOpenSettings,
  menuData,
  widgetConfig,
}: HeaderContentProps) => {
  const { t } = useI18n('common');
  const { t: tRoutes, i18n } = useTranslation(); // 用于路由翻译
  const navigate = useNavigate();
  const matches = useMatches();

  // 面包屑偏好设置
  const breadcrumbPreferences = usePreferencesStore((state) => state.preferences.breadcrumb);
  const breadcrumbStyleType = breadcrumbPreferences?.styleType ?? 'normal';

  // 默认头像（与 ele/vben 三端统一，取 preferences.app.defaultAvatar）
  const defaultAvatar = usePreferencesStore((state) => state.preferences.app.defaultAvatar);

  // 计算面包屑
  const breadcrumbItems = useMemo(() => {
    type MatchWithHandle = {
      pathname: string;
      handle?: { title?: string; icon?: string };
      id?: string; // React Router 内部 ID，用于识别 index 路由
    };
    const typedMatches = matches as MatchWithHandle[];
    const showIcon = breadcrumbPreferences?.showIcon ?? true;
    const showHome = breadcrumbPreferences?.showHome ?? true;

    const items = typedMatches
      .filter((match) => {
        // 过滤掉没有 title 的路由
        if (!match.handle?.title) return false;

        // 过滤掉 index 路由（它们通常是重定向，不应该出现在面包屑中）
        // index 路由的 id 通常包含 "-index"
        if (match.id?.includes('-index')) return false;

        return true;
      })
      .map((match, index, arr) => {
        // 将图标字符串转换为 React 组件（支持 Iconify 和 Ant Design）
        let icon: React.ReactNode = undefined;
        if (showIcon && match.handle?.icon) {
          icon = getIconFromName(match.handle.icon);
        }

        // 尝试通过路由 name 获取翻译标题
        let title = match.handle?.title || '';
        title = tRoutes(title, { defaultValue: title });

        return {
          key: match.pathname,
          title,
          icon,
          onClick:
            index < arr.length - 1
              ? () => {
                  navigate(match.pathname);
                }
              : undefined,
        };
      });

    // 始终在开头添加首页（如果 showHome 为 true）
    if (showHome) {
      // 检查是否已经存在根路径 '/' 的面包屑项，避免重复 key
      const hasHome = items.some((item) => item.key === '/');
      if (!hasHome) {
        items.unshift({
          key: '/',
          title: t('home'),
          icon: showIcon ? getIconFromName('lucide:home') : undefined,
          onClick: () => navigate('/'),
        });
      }
    }

    return items;
  }, [
    matches,
    navigate,
    t,
    tRoutes,
    i18n.language,
    breadcrumbPreferences?.showIcon,
    breadcrumbPreferences?.showHome,
  ]);

  // 语言切换
  const toggleLocale = (newLocale: SupportedLanguagesType) => {
    const { setPreferences } = usePreferencesStore.getState();
    setPreferences({ app: { locale: newLocale } });
  };

  // 语言菜单
  const languageMenuItems: MenuProps['items'] = [
    {
      key: 'zh-CN',
      label: '简体中文',
      icon: <span style={{ fontSize: 16 }}>🇨🇳</span>,
      onClick: () => toggleLocale('zh-CN'),
    },
    {
      key: 'en-US',
      label: 'English',
      icon: <span style={{ fontSize: 16 }}>🇺🇸</span>,
      onClick: () => toggleLocale('en-US'),
    },
  ];

  // 用户菜单
  const userMenuItems: MenuProps['items'] = [
    {
      key: 'profile',
      icon: <UserOutlined />,
      label: t('header.profile'),
      onClick: () => {
        navigate('/opm/profile');
      },
    },
    {
      key: 'settings',
      icon: <SettingOutlined />,
      label: t('header.settings'),
      onClick: onOpenSettings,
    },
    {
      type: 'divider',
    },
    {
      key: 'logout',
      icon: <LogoutOutlined style={{ color: '#ff4d4f' }} />,
      label: <span style={{ color: '#ff4d4f' }}>{t('header.logout')}</span>,
      onClick: onLogout,
    },
  ];

  // 通用按钮样式
  const btnStyle: React.CSSProperties = {
    color: 'var(--ant-color-text-secondary)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
  };


  // ── 全局搜索（本地菜单匹配；视觉形态对齐 vben SearchPanel）──
  const [searchOpen, setSearchOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [activeIndex, setActiveIndex] = useState(-1);
  const [searchHistory, setSearchHistory] = useState<{title: string; path: string; icon?: string}[]>([]);

  const SEARCH_HISTORY_KEY = 'menu_search_history';
  const SEARCH_HISTORY_MAX = 5;

  // 菜单标题翻译：与面包屑（tRoutes(title)）同语义——i18next 对 'routes:xxx' /
  // 'menu:xxx' 形态的 key 自动按「命名空间:键」解析，无需剥前缀
  const resolveMenuTitle = (label: string | undefined): string =>
    label ? tRoutes(label, { defaultValue: label }) : '';

  // 扁平化菜单树为可搜索的叶子页列表（i18n 相关：语言切换时重建）
  const searchableMenus = useMemo(() => {
    const excluded = ['/login', '/401', '/403', '/404', '/500'];
    const out: { title: string; path: string; icon?: string }[] = [];
    const walk = (items: any[]) => {
      for (const item of items) {
        if (item.children?.length) {
          walk(item.children);
          continue;
        }
        const raw = item.name || item.label;
        if (!item.path || !raw || excluded.includes(item.path)) continue;
        if (out.some((o) => o.path === item.path)) continue;
        out.push({
          title: resolveMenuTitle(raw),
          path: item.path,
          icon: typeof item.icon === 'string' ? item.icon : undefined,
        });
      }
    };
    walk(menuData);
    return out;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [menuData, i18n.language]);

  // 本地关键字匹配（contains，标题或路径）
  const localResults = useMemo(() => {
    const kw = searchQuery.trim().toLowerCase();
    if (!kw) return [];
    return searchableMenus.filter(
      (item) => item.title.toLowerCase().includes(kw) || item.path.toLowerCase().includes(kw),
    );
  }, [searchQuery, searchableMenus]);


  // 键盘导航作用于「与展示一致」的合并列表：有关键词=本地+语义，无关键词=历史
  const combinedResults = useMemo(
    () => (searchQuery.trim() ? localResults : searchHistory),
    [searchQuery, localResults, searchHistory],
  );

  const loadSearchHistory = () => {
    try {
      const raw = localStorage.getItem(SEARCH_HISTORY_KEY);
      setSearchHistory(raw ? JSON.parse(raw) : []);
    } catch (error) {
      console.error('load search history failed:', error);
      setSearchHistory([]);
    }
  };

  useEffect(() => {
    loadSearchHistory();
  }, []);

  const saveSearchHistory = (list: {title: string; path: string; icon?: string}[]) => {
    localStorage.setItem(SEARCH_HISTORY_KEY, JSON.stringify(list));
    setSearchHistory(list);
  };

  const removeSearchHistory = (index: number) => {
    const next = [...searchHistory];
    next.splice(index, 1);
    saveSearchHistory(next);
  };

  const closeSearch = () => {
    setSearchOpen(false);
    setSearchQuery('');
    setActiveIndex(-1);
  };

  const goSearchItem = (item: { path?: string; route?: string; title?: string; icon?: string }) => {
    const path = item.path || item.route;
    if (!path) return;
    // 写入最近搜索（去重置顶，上限 5）
    const matched = searchableMenus.find((m) => m.path === path);
    const entry = {
      title: item.title || matched?.title || path,
      path,
      icon: matched?.icon,
    };
    const next = [entry, ...searchHistory.filter((h) => h.path !== path)].slice(0, SEARCH_HISTORY_MAX);
    saveSearchHistory(next);
    closeSearch();
    navigate(path);
  };

  // Ctrl+K / Cmd+K 呼出全局搜索（偏好里可关：shortcutKeys.enable + shortcutKeys.globalSearch）
  const shortcutEnabled = usePreferencesStore((s) => s.preferences.shortcutKeys?.enable ?? true);
  const searchShortcutEnabled = usePreferencesStore((s) => s.preferences.shortcutKeys?.globalSearch ?? true);
  useEffect(() => {
    if (!widgetConfig.globalSearch || !shortcutEnabled || !searchShortcutEnabled) return undefined;
    const handler = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        setSearchOpen((open) => {
          if (!open) {
            loadSearchHistory();
            setSearchQuery('');
            setActiveIndex(-1);
          }
          return !open;
        });
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [widgetConfig.globalSearch, shortcutEnabled, searchShortcutEnabled]);

  return (
    <div
      style={{
        width: '100%',
        height: '100%',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
      }}
    >
      {/* ========== 左侧区域：折叠按钮 + 刷新 + 面包屑 ========== */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 4, flex: 1, minWidth: 0 }}>
        {/* 隐藏/显示侧边栏 */}
        {widgetConfig.sidebarToggle && (
          <Tooltip title={collapsed ? t('header.expandSidebar') : t('header.collapseSidebar')}>
            <Button
              type="text"
              icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
              onClick={onToggleCollapse}
              size="small"
              style={btnStyle}
            />
          </Tooltip>
        )}

        {/* 刷新按钮 */}
        {widgetConfig.refresh && (
          <Tooltip title={t('header.refresh')}>
            <Button
              type="text"
              icon={<ReloadOutlined />}
              onClick={onRefresh}
              size="small"
              style={btnStyle}
            />
          </Tooltip>
        )}

        {/* 分割线 */}
        <div
          style={{
            width: 1,
            height: 20,
            backgroundColor: 'var(--ant-color-bg-container)',
            margin: '0 8px',
          }}
        />

        {/* 面包屑 */}
        <Breadcrumb
          separator={breadcrumbStyleType === 'background' ? '/' : '>'}
          style={
            breadcrumbStyleType === 'background'
              ? {
                  padding: '4px 8px',
                  borderRadius: 6,
                  backgroundColor: 'var(--ant-color-bg-container)',
                }
              : undefined
          }
          items={breadcrumbItems.map((item, _index) => ({
            key: item.key,
            title: item.onClick ? (
              <a
                onClick={(e) => {
                  e.preventDefault();
                  item.onClick?.();
                }}
                style={{
                  color: 'var(--ant-color-text-secondary)',
                  fontSize: 13,
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: 4,
                  ...(breadcrumbStyleType === 'background'
                    ? {
                        padding: '4px 8px',
                        borderRadius: 4,
                        backgroundColor: 'var(--ant-color-bg-container)',
                        border: '1px solid var(--ant-color-border)',
                      }
                    : {}),
                }}
              >
                {item.icon}
                {item.title}
              </a>
            ) : (
              <span
                style={{
                  color: 'var(--ant-color-text)',
                  fontSize: 13,
                  fontWeight: 500,
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: 4,
                  ...(breadcrumbStyleType === 'background'
                    ? {
                        padding: '4px 8px',
                        borderRadius: 4,
                        backgroundColor: 'var(--ant-color-bg-container)',
                        border: '1px solid var(--ant-color-border)',
                      }
                    : {}),
                }}
              >
                {item.icon}
                {item.title}
              </span>
            ),
          }))}
        />
      </div>

      {/* ========== 右侧区域：搜索 + 设置 + 主题 + 语言 + 全屏 + 通知 + 头像 ========== */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 4, flexShrink: 0 }}>
        {/* 搜索按钮（带快捷键提示） */}
        {widgetConfig.globalSearch && (
          <>
            <Modal
              open={searchOpen}
              onCancel={closeSearch}
              footer={null}
              width={600}
              closable={false}
              centered
              destroyOnHidden
              styles={{ body: { padding: 0 } }}
            >
              {/* 头部：搜索图标 + 无边框大输入 */}
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 10,
                  padding: '10px 16px',
                  borderBottom: '1px solid var(--ant-color-border-secondary)',
                }}
              >
                <SearchOutlined style={{ color: 'var(--ant-color-text-secondary)', fontSize: 16 }} />
                <Input
                  variant="borderless"
                  className="global-search-input"
                  placeholder={t('header.searchPlaceholder')}
                  value={searchQuery}
                  onChange={(e) => {
                    setSearchQuery(e.target.value);
                    setActiveIndex(-1);
                  }}
                  onKeyDown={(e) => {
                    if (e.nativeEvent.isComposing) return;
                    const list = combinedResults;
                    if (e.key === 'ArrowDown') {
                      e.preventDefault();
                      if (list.length) setActiveIndex((i) => (i >= list.length - 1 ? 0 : i + 1));
                    } else if (e.key === 'ArrowUp') {
                      e.preventDefault();
                      if (list.length) setActiveIndex((i) => (i <= 0 ? list.length - 1 : i - 1));
                    } else if (e.key === 'Enter') {
                      e.preventDefault();
                      const target = list[activeIndex >= 0 ? activeIndex : 0];
                      if (target) goSearchItem(target);
                    }
                  }}
                  autoFocus
                  allowClear
                  style={{ fontSize: 15 }}
                />
              </div>

              {/* 结果区（限高内滚） */}
              <div style={{ maxHeight: 450, overflowY: 'auto', padding: '8px' }}>
                {/* 无关键词：最近搜索 */}
                {!searchQuery.trim() && searchHistory.length === 0 && (
                  <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--ant-color-text-secondary)', fontSize: 13 }}>
                    {t('header.noHistory')}
                  </div>
                )}
                {!searchQuery.trim() && searchHistory.length > 0 && (
                  <>
                    <div style={{ fontSize: 12, color: 'var(--ant-color-text-secondary)', padding: '4px 10px 6px' }}>
                      {t('header.searchRecent')}
                    </div>
                    {searchHistory.map((item, i) => (
                      <SearchRow
                        key={item.path}
                        iconNode={item.icon ? getIconFromName(item.icon) : undefined}
                        title={item.title}
                        active={activeIndex === i}
                        onMouseEnter={() => setActiveIndex(i)}
                        onClick={() => goSearchItem(item)}
                        onRemove={() => removeSearchHistory(i)}
                      />
                    ))}
                  </>
                )}

                {/* 有关键词：本地菜单命中 */}
                {searchQuery.trim() && localResults.length > 0 && (
                  <>
                    {localResults.map((item, i) => (
                      <SearchRow
                        key={item.path}
                        iconNode={item.icon ? getIconFromName(item.icon) : undefined}
                        title={item.title}
                        active={activeIndex === i}
                        onMouseEnter={() => setActiveIndex(i)}
                        onClick={() => goSearchItem(item)}
                      />
                    ))}
                  </>
                )}


                {/* 有关键词且无任何结果 */}
                {searchQuery.trim() && combinedResults.length === 0 && (
                  <div style={{ textAlign: 'center', padding: '24px 0', color: 'var(--ant-color-text-secondary)', fontSize: 13 }}>
                    {t('header.noResults')}
                  </div>
                )}
              </div>

              {/* 底部键位提示条 */}
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 16,
                  padding: '8px 16px',
                  borderTop: '1px solid var(--ant-color-border-secondary)',
                  fontSize: 12,
                  color: 'var(--ant-color-text-secondary)',
                }}
              >
                <span style={{ display: 'inline-flex', alignItems: 'center', gap: 5 }}>
                  <kbd style={searchHintKbdStyle}>↵</kbd>
                  {t('header.searchSelect')}
                </span>
                <span style={{ display: 'inline-flex', alignItems: 'center', gap: 5 }}>
                  <kbd style={searchHintKbdStyle}>↑</kbd>
                  <kbd style={searchHintKbdStyle}>↓</kbd>
                  {t('header.searchSwitch')}
                </span>
                <span style={{ display: 'inline-flex', alignItems: 'center', gap: 5 }}>
                  <kbd style={searchHintKbdStyle}>ESC</kbd>
                  {t('header.searchClose')}
                </span>
              </div>
            </Modal>

            <div
              className="search-trigger-btn"
              onClick={() => {
                loadSearchHistory();
                setActiveIndex(-1);
                setSearchOpen(true);
              }}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 8,
                padding: '6px 12px',
                borderRadius: 20,
                backgroundColor: 'var(--ant-color-bg-container)',
                border: '1px solid var(--ant-color-border)',
                cursor: 'pointer',
                transition: 'all 0.2s',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.backgroundColor = 'var(--ant-color-fill-tertiary)';
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.backgroundColor = 'var(--ant-color-bg-container)';
              }}
            >
              <SearchOutlined
                style={{
                  color: 'var(--ant-color-text-secondary)',
                  fontSize: 14,
                }}
              />
              <span
                style={{
                  color: 'var(--ant-color-text-secondary)',
                  fontSize: 13,
                }}
              >
                {t('header.search')}
              </span>
              <kbd
                style={{
                  display: 'inline-block',
                  padding: '2px 6px',
                  fontSize: 11,
                  fontFamily: 'monospace',
                  lineHeight: 1.4,
                  color: 'var(--ant-color-text-secondary)',
                  backgroundColor: 'var(--ant-color-bg-container)',
                  border: '1px solid var(--ant-color-border)',
                  borderRadius: 3,
                }}
              >
                {t('header.searchShortcut')}
              </kbd>
            </div>
          </>
        )}

        {/* 设置按钮 */}
        <Tooltip title={t('header.settings')}>
          <Button
            type="text"
            icon={<SettingOutlined />}
            onClick={onOpenSettings}
            size="small"
            style={btnStyle}
          />
        </Tooltip>

        {/* 主题切换 */}
        {widgetConfig.themeToggle && (
          <Tooltip title={isDark ? t('header.switchToLight') : t('header.switchToDark')}>
            <Button
              type="text"
              icon={isDark ? <SunOutlined /> : <MoonOutlined />}
              onClick={onToggleTheme}
              size="small"
              style={btnStyle}
            />
          </Tooltip>
        )}

        {/* 语言切换 - 下拉菜单 */}
        {widgetConfig.languageToggle && (
          <Dropdown menu={{ items: languageMenuItems }} trigger={['click']} placement="bottomRight">
            <Tooltip title={t('header.switchLanguage')}>
              <Button type="text" icon={<GlobalOutlined />} size="small" style={btnStyle} />
            </Tooltip>
          </Dropdown>
        )}

        {/* 全屏切换 */}
        {widgetConfig.fullscreen && (
          <Tooltip title={isFullscreen ? t('header.exitFullscreen') : t('header.fullscreen')}>
            <Button
              type="text"
              icon={isFullscreen ? <FullscreenExitOutlined /> : <FullscreenOutlined />}
              onClick={onToggleFullscreen}
              size="small"
              style={btnStyle}
            />
          </Tooltip>
        )}


        {/* 用户头像 + 下拉菜单 */}
        <Dropdown menu={{ items: userMenuItems }} trigger={['click']} placement="bottomRight">
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 6,
              cursor: 'pointer',
              borderRadius: 6,
              padding: '2px 8px',
              marginLeft: 4,
              transition: 'background-color 0.2s',
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.backgroundColor = 'var(--ant-color-bg-container)';
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.backgroundColor = 'transparent';
            }}
          >
            <Avatar src={userInfo?.avatar || defaultAvatar || undefined} icon={<UserOutlined />} size="small" />
            <span
              className="hidden md:inline"
              style={{
                fontSize: 13,
                fontWeight: 500,
                color: 'var(--ant-color-text)',
                maxWidth: 100,
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
              }}
            >
              {userInfo?.username || t('header.guest')}
            </span>
          </div>
        </Dropdown>
      </div>
    </div>
  );
};

export default HeaderContent;
