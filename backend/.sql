-- 创建数据库
CREATE DATABASE finance;
\c finance;

-- 账本（预留多用户/多账本扩展）
CREATE TABLE fin_ledgers (
  id          SERIAL PRIMARY KEY,
  name        TEXT NOT NULL,
  description TEXT,
  created_at  TIMESTAMP NOT NULL DEFAULT now(),
  deleted_at  TIMESTAMP NULL
);
COMMENT ON TABLE fin_ledgers IS '账本（预留多用户/多账本扩展）';
CREATE INDEX idx_fin_ledgers_deleted_at ON fin_ledgers(deleted_at);

-- 默认账本
INSERT INTO fin_ledgers (id, name) VALUES (1, 'default')
ON CONFLICT (id) DO NOTHING;
SELECT setval(pg_get_serial_sequence('fin_ledgers', 'id'), (SELECT MAX(id) FROM fin_ledgers));

-- 账户
CREATE TABLE fin_accounts (
  id            SERIAL PRIMARY KEY,
  ledger_id     INT NOT NULL DEFAULT 1 REFERENCES fin_ledgers(id) ON DELETE RESTRICT,
  name          TEXT NOT NULL,
  type          TEXT NOT NULL CHECK (type IN ('cash','liability','debt','investment','other_asset')),
  currency      TEXT NOT NULL DEFAULT 'CNY',
  is_active     BOOLEAN NOT NULL DEFAULT TRUE,
  created_at    TIMESTAMP NOT NULL DEFAULT now(),
  deleted_at    TIMESTAMP NULL
);
COMMENT ON TABLE fin_accounts IS '财务账户，包含现金/负债/债务/投资等账户元数据';
CREATE INDEX idx_fin_accounts_ledger_id ON fin_accounts(ledger_id);
CREATE INDEX idx_fin_accounts_deleted_at ON fin_accounts(deleted_at);

-- 账户期初 / 快照
CREATE TABLE fin_account_snapshots (
  id         SERIAL PRIMARY KEY,
  ledger_id  INT NOT NULL DEFAULT 1 REFERENCES fin_ledgers(id) ON DELETE RESTRICT,
  account_id INT NOT NULL REFERENCES fin_accounts(id) ON DELETE CASCADE,
  as_of      DATE NOT NULL,
  amount     NUMERIC(18,2) NOT NULL,
  note       TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT now()
);
COMMENT ON TABLE fin_account_snapshots IS '账户期初或任意时点快照，用于资产基准';
CREATE UNIQUE INDEX idx_fin_account_snapshots_unique ON fin_account_snapshots(ledger_id, account_id, as_of);
CREATE INDEX idx_fin_account_snapshots_ledger_id ON fin_account_snapshots(ledger_id);

-- 分类
CREATE TABLE fin_categories (
  id            SERIAL PRIMARY KEY,
  ledger_id     INT NOT NULL DEFAULT 1 REFERENCES fin_ledgers(id) ON DELETE RESTRICT,
  name          TEXT NOT NULL,
  kind          TEXT NOT NULL CHECK (kind IN ('income','expense','transfer','investment')),
  parent_id     INT REFERENCES fin_categories(id),
  deleted_at    TIMESTAMP NULL
);
COMMENT ON TABLE fin_categories IS '收支/转账/投资分类，可自引用形成层级';
CREATE UNIQUE INDEX idx_fin_categories_parent_name ON fin_categories(ledger_id, parent_id, name);
CREATE INDEX idx_fin_categories_ledger_id ON fin_categories(ledger_id);
CREATE INDEX idx_fin_categories_parent_id ON fin_categories(parent_id);
CREATE INDEX idx_fin_categories_deleted_at ON fin_categories(deleted_at);

-- 交易主表
CREATE TABLE fin_transactions (
  id            SERIAL PRIMARY KEY,
  ledger_id     INT NOT NULL DEFAULT 1 REFERENCES fin_ledgers(id) ON DELETE RESTRICT,
  occurred_on   DATE NOT NULL, -- 记账日（按日精度）
  description   TEXT,
  note          TEXT,
  created_at    TIMESTAMP NOT NULL DEFAULT now(),
  deleted_at    TIMESTAMP NULL
);
COMMENT ON TABLE fin_transactions IS '交易主表，记录记账日与摘要';
CREATE INDEX idx_fin_transactions_ledger_id ON fin_transactions(ledger_id);
CREATE INDEX idx_fin_transactions_occurred_on ON fin_transactions(occurred_on);
CREATE INDEX idx_fin_transactions_deleted_at ON fin_transactions(deleted_at);

-- 分录
CREATE TABLE fin_transaction_lines (
  id             SERIAL PRIMARY KEY,
  ledger_id      INT NOT NULL DEFAULT 1 REFERENCES fin_ledgers(id) ON DELETE RESTRICT,
  transaction_id INT NOT NULL REFERENCES fin_transactions(id) ON DELETE CASCADE,
  account_id     INT NOT NULL REFERENCES fin_accounts(id),
  category_id    INT REFERENCES fin_categories(id),
  amount         NUMERIC(18,2) NOT NULL CHECK (amount <> 0), -- 收入正，支出负；转账/投资用借贷平衡
  tags           TEXT[] DEFAULT '{}',
  note           TEXT,
  deleted_at     TIMESTAMP NULL
);
COMMENT ON TABLE fin_transaction_lines IS '交易分录，连接交易与账户/分类/标签/金额';
CREATE INDEX idx_fin_transaction_lines_ledger_id ON fin_transaction_lines(ledger_id);
CREATE INDEX idx_fin_transaction_lines_tx ON fin_transaction_lines(transaction_id);
CREATE INDEX idx_fin_transaction_lines_account ON fin_transaction_lines(account_id);
CREATE INDEX idx_fin_transaction_lines_category ON fin_transaction_lines(category_id);
CREATE INDEX idx_fin_transaction_lines_deleted_at ON fin_transaction_lines(deleted_at);

-- 证券主数据
CREATE TABLE fin_securities (
  id         SERIAL PRIMARY KEY,
  ledger_id  INT NOT NULL DEFAULT 1 REFERENCES fin_ledgers(id) ON DELETE RESTRICT,
  ticker     TEXT NOT NULL UNIQUE,
  name       TEXT NOT NULL,
  currency   TEXT NOT NULL DEFAULT 'CNY',
  deleted_at TIMESTAMP NULL
);
COMMENT ON TABLE fin_securities IS '可投资标的元数据，如股票/基金';
CREATE INDEX idx_fin_securities_ledger_id ON fin_securities(ledger_id);
CREATE INDEX idx_fin_securities_deleted_at ON fin_securities(deleted_at);

-- 投资批次（数量与价格）
CREATE TABLE fin_investment_lots (
  id                   SERIAL PRIMARY KEY,
  ledger_id            INT NOT NULL DEFAULT 1 REFERENCES fin_ledgers(id) ON DELETE RESTRICT,
  transaction_line_id  INT NOT NULL REFERENCES fin_transaction_lines(id) ON DELETE CASCADE,
  security_id          INT NOT NULL REFERENCES fin_securities(id),
  quantity             NUMERIC(18,4) NOT NULL,
  price                NUMERIC(18,4) NOT NULL,
  trade_price          NUMERIC(18,4) NOT NULL DEFAULT 0,
  fee                  NUMERIC(18,4) NOT NULL DEFAULT 0,
  tax                  NUMERIC(18,4) NOT NULL DEFAULT 0,
  deleted_at           TIMESTAMP NULL
);
COMMENT ON TABLE fin_investment_lots IS '买入批次数量与成交价，支持持仓与成本核算';
CREATE INDEX idx_fin_investment_lots_ledger_id ON fin_investment_lots(ledger_id);
CREATE INDEX idx_fin_investment_lots_security_id ON fin_investment_lots(security_id);
CREATE INDEX idx_fin_investment_lots_deleted_at ON fin_investment_lots(deleted_at);

-- 投资卖出记录（数量与价格）
CREATE TABLE fin_investment_sales (
  id                   SERIAL PRIMARY KEY,
  ledger_id            INT NOT NULL DEFAULT 1 REFERENCES fin_ledgers(id) ON DELETE RESTRICT,
  transaction_line_id  INT NOT NULL REFERENCES fin_transaction_lines(id) ON DELETE CASCADE,
  security_id          INT NOT NULL REFERENCES fin_securities(id),
  quantity             NUMERIC(18,4) NOT NULL,
  price                NUMERIC(18,4) NOT NULL,
  deleted_at           TIMESTAMP NULL
);
COMMENT ON TABLE fin_investment_sales IS '卖出记录数量与成交价，用于已实现盈亏核算';
CREATE INDEX idx_fin_investment_sales_ledger_id ON fin_investment_sales(ledger_id);
CREATE INDEX idx_fin_investment_sales_line_id ON fin_investment_sales(transaction_line_id);
CREATE INDEX idx_fin_investment_sales_security_id ON fin_investment_sales(security_id);
CREATE INDEX idx_fin_investment_sales_deleted_at ON fin_investment_sales(deleted_at);

-- 买入批次与卖出记录匹配（支持部分卖出）
CREATE TABLE fin_investment_lot_allocations (
  id          SERIAL PRIMARY KEY,
  ledger_id   INT NOT NULL DEFAULT 1 REFERENCES fin_ledgers(id) ON DELETE RESTRICT,
  buy_lot_id  INT NOT NULL REFERENCES fin_investment_lots(id) ON DELETE CASCADE,
  sale_id     INT NOT NULL REFERENCES fin_investment_sales(id) ON DELETE CASCADE,
  quantity    NUMERIC(18,4) NOT NULL,
  created_at  TIMESTAMP NOT NULL DEFAULT now(),
  deleted_at  TIMESTAMP NULL
);
COMMENT ON TABLE fin_investment_lot_allocations IS '买入批次与卖出记录的匹配分配，用于成本归集';
CREATE INDEX idx_fin_investment_lot_allocations_ledger_id ON fin_investment_lot_allocations(ledger_id);
CREATE INDEX idx_fin_investment_lot_allocations_buy_lot_id ON fin_investment_lot_allocations(buy_lot_id);
CREATE INDEX idx_fin_investment_lot_allocations_sale_id ON fin_investment_lot_allocations(sale_id);
CREATE INDEX idx_fin_investment_lot_allocations_deleted_at ON fin_investment_lot_allocations(deleted_at);

-- 价格历史
CREATE TABLE fin_security_prices (
  ledger_id   INT NOT NULL DEFAULT 1 REFERENCES fin_ledgers(id) ON DELETE RESTRICT,
  security_id  INT NOT NULL REFERENCES fin_securities(id),
  price_at     DATE NOT NULL,
  close_price  NUMERIC(18,4) NOT NULL,
  deleted_at   TIMESTAMP NULL,
  PRIMARY KEY (ledger_id, security_id, price_at)
);
COMMENT ON TABLE fin_security_prices IS '标的每日收盘价，用于任意时点估值';
CREATE INDEX idx_fin_security_prices_ledger_id ON fin_security_prices(ledger_id);
CREATE INDEX idx_fin_security_prices_deleted_at ON fin_security_prices(deleted_at);
