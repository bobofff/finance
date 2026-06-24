-- Optimize category taxonomy for daily bookkeeping and AI parsing.
-- Strategy:
-- 1. Keep active expense categories at root + second level.
-- 2. Remap existing transaction lines before hiding old categories.
-- 3. Soft-delete categories instead of hard deleting, so historical ids remain auditable.
-- 4. Add a small set of common income categories.

BEGIN;

-- Add missing income categories. Existing "薪资" is kept.
DO $$
DECLARE
  income_name text;
BEGIN
  FOREACH income_name IN ARRAY ARRAY[
    '奖金',
    '报销',
    '利息',
    '投资收益',
    '退款返还',
    '礼金红包',
    '其他收入'
  ]
  LOOP
    IF NOT EXISTS (
      SELECT 1
      FROM public.fin_categories
      WHERE ledger_id = 1
        AND kind = 'income'
        AND parent_id IS NULL
        AND deleted_at IS NULL
        AND name = income_name
    ) THEN
      INSERT INTO public.fin_categories (ledger_id, name, kind, parent_id, deleted_at)
      VALUES (1, income_name, 'income', NULL, NULL);
    END IF;
  END LOOP;
END $$;

-- Rename a few long/ambiguous labels to analysis-friendly labels.
UPDATE public.fin_categories
SET name = '日用家清'
WHERE id = 6 AND ledger_id = 1 AND name = '厨房与日用消耗' AND deleted_at IS NULL;

UPDATE public.fin_categories
SET name = '个护美容'
WHERE id = 9 AND ledger_id = 1 AND name = '个人形象与护理' AND deleted_at IS NULL;

UPDATE public.fin_categories
SET name = '金融保险'
WHERE id = 11 AND ledger_id = 1 AND name = '金融与保障' AND deleted_at IS NULL;

UPDATE public.fin_categories
SET name = '家庭支持'
WHERE id = 13 AND ledger_id = 1 AND name = '家庭责任与孝敬' AND deleted_at IS NULL;

UPDATE public.fin_categories
SET name = '通讯订阅'
WHERE id = 18 AND ledger_id = 1 AND name = '通讯与数字服务' AND deleted_at IS NULL;

UPDATE public.fin_categories
SET name = '生鲜食材'
WHERE id = 38 AND ledger_id = 1 AND name = '家庭做饭食材' AND deleted_at IS NULL;

-- Merge overlapping second-level branches.
-- old_id -> target_id
-- 3   大额耐用品与家庭设备        -> 106 居住住房 / 家具家电
-- 109 居住住房 / 网络通信到家    -> 115 通讯订阅 / 网络服务
-- 113 通讯订阅 / 云服务与软件    -> 101 数码电子 / 软件与系统
-- 102 数码电子 / 电子耗材        -> 104 数码电子 / 数码配件
-- 122 职场办公 / 出差自付        -> 100 旅行度假 / 出行交通
-- 123 职场办公 / 工作社交        -> 80  社交人情 / 请客应酬
-- 125 职场办公 / 通勤附加        -> 73  交通出行 / 市内公共交通
CREATE TEMP TABLE tmp_category_merge (
  old_id int PRIMARY KEY,
  target_id int NOT NULL
) ON COMMIT DROP;

INSERT INTO tmp_category_merge (old_id, target_id) VALUES
  (3, 106),
  (109, 115),
  (113, 101),
  (102, 104),
  (122, 100),
  (123, 80),
  (125, 73);

-- Expand merge branches to include descendants, then remap historical lines.
WITH RECURSIVE merge_tree AS (
  SELECT m.old_id, m.target_id
  FROM tmp_category_merge m
  JOIN public.fin_categories c ON c.id = m.old_id
  WHERE c.ledger_id = 1

  UNION ALL

  SELECT c.id AS old_id, mt.target_id
  FROM public.fin_categories c
  JOIN merge_tree mt ON c.parent_id = mt.old_id
  WHERE c.ledger_id = 1
),
dedup_merge AS (
  SELECT DISTINCT old_id, target_id
  FROM merge_tree
  WHERE old_id <> target_id
)
UPDATE public.fin_transaction_lines tl
SET category_id = dm.target_id
FROM dedup_merge dm
WHERE tl.ledger_id = 1
  AND tl.category_id = dm.old_id;

-- Hide merged categories and their descendants.
WITH RECURSIVE merge_tree AS (
  SELECT m.old_id
  FROM tmp_category_merge m
  JOIN public.fin_categories c ON c.id = m.old_id
  WHERE c.ledger_id = 1

  UNION ALL

  SELECT c.id AS old_id
  FROM public.fin_categories c
  JOIN merge_tree mt ON c.parent_id = mt.old_id
  WHERE c.ledger_id = 1
)
UPDATE public.fin_categories c
SET deleted_at = COALESCE(c.deleted_at, now())
FROM (
  SELECT DISTINCT old_id
  FROM merge_tree
) mt
WHERE c.id = mt.old_id
  AND c.ledger_id = 1
  AND c.deleted_at IS NULL;

-- Remap every third-level-or-deeper category to its second-level ancestor.
-- This keeps existing transaction analysis valid while reducing active categories.
WITH RECURSIVE category_tree AS (
  SELECT
    id,
    parent_id,
    id AS root_id,
    NULL::int AS level2_id,
    1 AS depth
  FROM public.fin_categories
  WHERE ledger_id = 1
    AND parent_id IS NULL

  UNION ALL

  SELECT
    c.id,
    c.parent_id,
    ct.root_id,
    CASE WHEN ct.depth = 1 THEN c.id ELSE ct.level2_id END AS level2_id,
    ct.depth + 1 AS depth
  FROM public.fin_categories c
  JOIN category_tree ct ON c.parent_id = ct.id
  WHERE c.ledger_id = 1
),
level_remap AS (
  SELECT id AS old_id, level2_id AS target_id
  FROM category_tree
  WHERE depth >= 3
    AND level2_id IS NOT NULL
)
UPDATE public.fin_transaction_lines tl
SET category_id = lr.target_id
FROM level_remap lr
WHERE tl.ledger_id = 1
  AND tl.category_id = lr.old_id;

-- Hide every third-level-or-deeper category. Active taxonomy remains two levels.
WITH RECURSIVE category_tree AS (
  SELECT
    id,
    parent_id,
    1 AS depth
  FROM public.fin_categories
  WHERE ledger_id = 1
    AND parent_id IS NULL

  UNION ALL

  SELECT
    c.id,
    c.parent_id,
    ct.depth + 1 AS depth
  FROM public.fin_categories c
  JOIN category_tree ct ON c.parent_id = ct.id
  WHERE c.ledger_id = 1
),
to_hide AS (
  SELECT id
  FROM category_tree
  WHERE depth >= 3
)
UPDATE public.fin_categories c
SET deleted_at = COALESCE(c.deleted_at, now())
FROM to_hide h
WHERE c.id = h.id
  AND c.ledger_id = 1
  AND c.deleted_at IS NULL;

-- Optional sanity checks. Expect active expense taxonomy to have only depth 1 and 2.
WITH RECURSIVE active_tree AS (
  SELECT
    id,
    parent_id,
    kind,
    name,
    1 AS depth
  FROM public.fin_categories
  WHERE ledger_id = 1
    AND parent_id IS NULL
    AND deleted_at IS NULL

  UNION ALL

  SELECT
    c.id,
    c.parent_id,
    c.kind,
    c.name,
    at.depth + 1 AS depth
  FROM public.fin_categories c
  JOIN active_tree at ON c.parent_id = at.id
  WHERE c.ledger_id = 1
    AND c.deleted_at IS NULL
)
SELECT kind, depth, count(*) AS active_category_count
FROM active_tree
GROUP BY kind, depth
ORDER BY kind, depth;

SELECT id, name, kind, parent_id
FROM public.fin_categories
WHERE ledger_id = 1
  AND deleted_at IS NULL
ORDER BY kind, parent_id NULLS FIRST, id;

COMMIT;
