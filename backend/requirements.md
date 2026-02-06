# Finance Backend 需求说明

本文梳理个人资产系统需求、现有骨架能力与数据库设计，便于后续迭代。

## 业务需求
- 账户与期初：管理现金/负债/债务/投资等账户，记录期初资产/负债，随时查看当前资产负债表。
- 收入/支出（按日）：单笔收支精确到“日”；按月/年统计分类维度和总览。
- 转账：账户间转账不影响损益，但影响账户余额。
- 投资：记录入金/出金、买入/卖出，跟踪持仓、已实现/未实现盈亏；可按月/年聚合。
- 时间分析：按月/年查看现金流、损益、资产变动趋势。
- 多币种：账户和标的可带币种；如需跨币种汇总，需额外汇率数据（暂未落地）。

## 配置要求
- 主要环境变量（见 `.env.example`）：`APP_ENV`、`HTTP_PORT`、`DB_DRIVER`、`DB_HOST`、`DB_PORT`、`DB_USER`、`DB_PASSWORD`、`DB_NAME`、`DB_SSLMODE`、`DB_TIMEZONE`。
- `.env` 会在程序启动时自动加载，不覆盖已有环境变量。

## 数据模型（SQL 文件为准）
- `fin_accounts`：账户主数据。字段：`id`、`name`、`type`（`cash|liability|debt|investment|other_asset`）、`currency`（默认 CNY）、`is_active`、`created_at`、`deleted_at`。
- `fin_account_snapshots`：账户期初/快照。字段：`id`、`account_id`、`as_of`(date)、`amount`、`note`，唯一 `(account_id, as_of)`。
- `fin_categories`：收支/转账/投资分类（自引用层级）。字段：`id`、`name`、`kind`（`income|expense|transfer|investment`）、`parent_id`、`deleted_at`；同层级 `(parent_id, name)` 唯一。
- `fin_transactions`：交易主表，`occurred_on`(date) 表示记账日，含摘要/备注、软删标记。
- `fin_transaction_lines`：分录。字段：`id`、`transaction_id`、`account_id`、`category_id`、`amount`(收入正、支出负；转账/投资以借贷平衡)、`tags`、`note`、`deleted_at`；索引覆盖 `transaction_id`、`account_id`、`category_id`。
- 投资：`fin_securities`（标的）、`fin_investment_lots`（买入批次）、`fin_investment_sales`（卖出记录）、`fin_investment_lot_allocations`（批次匹配）、`fin_security_prices`（历史价格）。

> 关键口径：收支按日存储；同一 transaction 下分录金额需在业务层保证平衡（借贷和为 0）；转账用两条分录表示转出/转入；投资入金/出金可用转账口径（现金账户与投资账户间）+ 买卖分录。已实现收益不落账，按卖出记录与批次匹配结果计算。

## API 现状
- `GET /api/health`：健康检查。

### 账户（/api/accounts）
- `POST /api/accounts`：创建账户。字段：`name`(必填)、`type`(必填)、`currency`(默认 CNY)、`is_active`(默认 true)。
- `GET /api/accounts`：按 `id` 升序返回全部账户。
- `GET /api/accounts/:id`：查询单个账户。
- `PATCH /api/accounts/:id`：部分更新（字段同上）。
- `DELETE /api/accounts/:id`：软删除。

## 待办/需求空白
- 分类接口：`internal/handler/categories` 空实现；补齐 CRUD、枚举校验、父子关系校验、软删除、路由注册。
- 交易/分录/投资接口：模型与业务逻辑尚未实现；需基于 SQL 草案补齐（含日粒度校验、分录平衡校验、入金/出金与买卖逻辑）。
- 汇率/多用户：如需跨币种或多人账本，需新增汇率表和 user_id/tenant_id 字段。
- 错误响应/鉴权/日志：统一错误格式、权限控制和审计待补充。

## 运行与测试建议
- 启动：`go run ./cmd/server`（自动 AutoMigrate）。
- 测试：`GOCACHE=$(pwd)/.gocache go test ./...` 规避主目录权限限制。
- 初始化：先创建数据库与用户；表结构可由 AutoMigrate 生成，或用根目录 `.sql` 初始化。
