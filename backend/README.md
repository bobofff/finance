# Finance Backend Skeleton

Go (Gin + GORM) skeleton for the finance system. Business logic is intentionally omitted; this only sets up the project layout.

## Structure
- `cmd/server` – entrypoint.
- `internal/config` – env-based configuration.
- `internal/db` – GORM connection helpers (PostgreSQL/MySQL).
- `internal/router` – Gin router setup.
- `internal/handler/health` – sample health endpoint.
- `frontend/` – placeholder directory for the future SPA/FE project.

## Quick start
1) Set environment variables (see `.env.example`), e.g.:
   ```bash
   export APP_ENV=development
   export HTTP_PORT=8888
   export DB_DRIVER=postgres
   export DB_HOST=127.0.0.1
   export DB_PORT=5432
   export DB_USER=finance
   export DB_PASSWORD=finance
   export DB_NAME=finance
   ```
   Or create a `.env` file (copy from `.env.example`); the server now loads it automatically.
2) Run the server:
   ```bash
   go run ./cmd/server
   ```
3) Test health:
   ```bash
   curl http://localhost:8888/api/health
   ```

## 思路与需求
- 目标：记录期初资金与按日收支；任意时点可查询资产/负债总额；支持按时间段分析消费。
- 口径：资产与负债分开展示；投资按公允价值计入；暂不支持多币种汇总。
- 投资定价：手动维护标的价格，按 `as_of` 取最近价格估值。
- 多账本预留：引入 `fin_ledgers`，核心表带 `ledger_id`（默认 1），便于后续扩展多用户/多账本。
- 核心表：账户、账户快照、分类、交易、分录、证券、投资批次、价格历史（详见 `.sql`）。
- 时点资产计算：最近快照 + 当日及以前分录累计；投资账户在此基础上叠加持仓市值。

## 操作明细（简单模式：单分录）
### 录入一笔收入或支出
1) 前置数据
   - 账户已存在且未软删（`fin_accounts`）。
   - 分类已存在且未软删（`fin_categories`），并确保 `kind=income` 或 `kind=expense`。
   - 使用默认账本 `ledger_id=1`（后续可切换其他账本）。
2) 创建交易主表
   - 表：`fin_transactions`
   - 字段：`ledger_id`、`occurred_on`、`description`/`note`
3) 创建分录
   - 表：`fin_transaction_lines`
   - 字段：`ledger_id`、`transaction_id`、`account_id`、`category_id`、`amount`
   - 规则：收入 `amount > 0`，支出 `amount < 0`
4) 结果
   - 该分录直接影响账户余额与各类统计。

### 推荐校验
- `ledger_id` 存在；`account_id`、`category_id` 存在且未软删。
- `category.kind` 与交易类型一致（收入/支出）。
- `amount != 0`，且符号正确（收入为正、支出为负）。
- `occurred_on` 为合法日期。

## Notes
- `DB_DRIVER` supports `postgres` or `mysql`.
- Connection pool defaults are set in `internal/db/db.go`; tweak as needed.
- Add your own modules (services/repositories/handlers) under `internal/`.

## Hot reload (Air)
- Install (requires network): `go install github.com/cosmtrek/air@latest` (set `GOPROXY` if需要国内镜像，如 `export GOPROXY=https://goproxy.cn,direct`).
- Config file: `.air.toml` (already included).
- Run with hot reload:
  ```bash
  air
  ```

