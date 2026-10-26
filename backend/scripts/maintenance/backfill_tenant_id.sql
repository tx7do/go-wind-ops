-- backfill_tenant_id.sql —— 存量 ops 行租户回填(一次性运维脚本,幂等可重跑)
--
-- 背景:ops 业务表挂 TenantID mixin(go-crud TenantPrivacy)后,租户用户只能
-- 看到 tenant_id = 自己租户的行;历史行 tenant_id 为 NULL/0 时仅平台上下文
-- 可见,租户视角表现为"数据消失"。本脚本把存量行一次性归属到目标租户。
--
-- 范围:表名 ops_ 前缀且实际含 tenant_id 列的表(经 information_schema 动态
-- 发现,自动覆盖未来新增业务表;ops_agent_nodes/ops_node_metrics 平台级设计
-- 无租户列,天然不在范围内)。基座 sys_* 表不动——菜单/权限/API 等属平台
-- 资源,用户/角色由基座播种自带归属。
--
-- 用法(目标租户必须显式指定,防手滑挂错):
--   psql "postgres://<user>:<pass>@<host>:<port>/<db>" \
--        -v tenant_id=1 \
--        -f backend/scripts/maintenance/backfill_tenant_id.sql
-- 只回填 tenant_id IS NULL OR tenant_id = 0 的行,已有归属的行不受影响。

\if :{?tenant_id}
\else
\echo '缺少 -v tenant_id=<目标租户ID>,已中断。用法: psql "<dsn>" -v tenant_id=1 -f backfill_tenant_id.sql'
\quit
\endif

SET backfill.target_tenant = :'tenant_id';

BEGIN;

DO $backfill$
DECLARE
    target_tenant bigint;
    tbl           record;
    affected      bigint := 0;
    total         bigint := 0;
    tables_hit    int    := 0;
BEGIN
    target_tenant := current_setting('backfill.target_tenant', true)::bigint;

    -- 目标租户必须真实存在
    IF NOT EXISTS (SELECT 1 FROM sys_tenants WHERE id = target_tenant) THEN
        RAISE EXCEPTION '目标租户 % 不存在(sys_tenants 无此行),请确认 -v tenant_id=<id>', target_tenant;
    END IF;

    FOR tbl IN
        SELECT c.relname AS table_name
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        JOIN information_schema.columns col
          ON col.table_schema = n.nspname
         AND col.table_name  = c.relname
         AND col.column_name = 'tenant_id'
        WHERE n.nspname  = current_schema()
          AND c.relkind  = 'r'
          AND c.relname LIKE 'ops_%'
        ORDER BY c.relname
    LOOP
        EXECUTE format(
            'UPDATE %I SET tenant_id = %s WHERE tenant_id IS NULL OR tenant_id = 0',
            tbl.table_name, target_tenant);
        GET DIAGNOSTICS affected = ROW_COUNT;
        total      := total + affected;
        tables_hit := tables_hit + 1;
        RAISE NOTICE '%: 回填 % 行', tbl.table_name, affected;
    END LOOP;

    RAISE NOTICE '完成:扫过 % 张 ops 表,共回填 % 行 -> tenant_id=%(下方 COMMIT 后生效)',
        tables_hit, total, target_tenant;
END
$backfill$;

COMMIT;

-- 验证:残留未归属行应为 0
DO $verify$
DECLARE
    tbl        record;
    remain     bigint;
    sum_remain bigint := 0;
BEGIN
    FOR tbl IN
        SELECT c.relname AS table_name
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        JOIN information_schema.columns col
          ON col.table_schema = n.nspname
         AND col.table_name  = c.relname
         AND col.column_name = 'tenant_id'
        WHERE n.nspname  = current_schema()
          AND c.relkind  = 'r'
          AND c.relname LIKE 'ops_%'
        ORDER BY c.relname
    LOOP
        EXECUTE format(
            'SELECT count(*) FROM %I WHERE tenant_id IS NULL OR tenant_id = 0',
            tbl.table_name)
        INTO remain;
        sum_remain := sum_remain + COALESCE(remain, 0);
        IF remain > 0 THEN
            RAISE NOTICE '% 仍有 % 行未归属', tbl.table_name, remain;
        END IF;
    END LOOP;
    RAISE NOTICE '验证:%',
        CASE WHEN sum_remain = 0 THEN '全部 ops 行已归属,无残留'
             ELSE '仍有残留行,请检查' END;
END
$verify$;
