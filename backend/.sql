-- 创建数据库
CREATE DATABASE finance;
\c finance;

-- 账户
CREATE TABLE fin_accounts (
  id            SERIAL PRIMARY KEY,
  name          TEXT NOT NULL,
  type          TEXT NOT NULL CHECK (type IN ('cash','liability','debt','investment','other_asset')),
  currency      TEXT NOT NULL DEFAULT 'CNY',
  is_active     BOOLEAN NOT NULL DEFAULT TRUE,
  created_at    TIMESTAMP NOT NULL DEFAULT now(),
  deleted_at    TIMESTAMP NULL
);
COMMENT ON TABLE fin_accounts IS '财务账户，包含现金/负债/债务/投资等账户元数据';
CREATE INDEX idx_fin_accounts_deleted_at ON fin_accounts(deleted_at);

-- 分类
CREATE TABLE fin_categories (
  id            SERIAL PRIMARY KEY,
  name          TEXT NOT NULL,
  kind          TEXT NOT NULL CHECK (kind IN ('income','expense','transfer','investment')),
  parent_id     INT REFERENCES fin_categories(id),
  deleted_at    TIMESTAMP NULL
);
COMMENT ON TABLE fin_categories IS '收支/转账/投资分类，可自引用形成层级';
CREATE INDEX idx_fin_categories_deleted_at ON fin_categories(deleted_at);

-- 交易主表
CREATE TABLE fin_transactions (
  id            SERIAL PRIMARY KEY,
  occurred_at   TIMESTAMP NOT NULL,
  description   TEXT,
  note          TEXT,
  created_at    TIMESTAMP NOT NULL DEFAULT now(),
  deleted_at    TIMESTAMP NULL
);
COMMENT ON TABLE fin_transactions IS '交易主表，记录发生时间与摘要';
CREATE INDEX idx_fin_transactions_deleted_at ON fin_transactions(deleted_at);

-- 分录
CREATE TABLE fin_transaction_lines (
  id             SERIAL PRIMARY KEY,
  transaction_id INT NOT NULL REFERENCES fin_transactions(id) ON DELETE CASCADE,
  account_id     INT NOT NULL REFERENCES fin_accounts(id),
  category_id    INT REFERENCES fin_categories(id),
  amount         NUMERIC(18,2) NOT NULL, -- 收入正，支出负；转账/投资用借贷平衡
  tags           TEXT[] DEFAULT '{}',
  note           TEXT,
  deleted_at     TIMESTAMP NULL
);
COMMENT ON TABLE fin_transaction_lines IS '交易分录，连接交易与账户/分类/标签/金额';
CREATE INDEX idx_fin_transaction_lines_deleted_at ON fin_transaction_lines(deleted_at);

-- 证券主数据
CREATE TABLE fin_securities (
  id         SERIAL PRIMARY KEY,
  ticker     TEXT NOT NULL UNIQUE,
  name       TEXT NOT NULL,
  currency   TEXT NOT NULL DEFAULT 'CNY',
  deleted_at TIMESTAMP NULL
);
COMMENT ON TABLE fin_securities IS '可投资标的元数据，如股票/基金';
CREATE INDEX idx_fin_securities_deleted_at ON fin_securities(deleted_at);

-- 投资批次（数量与价格）
CREATE TABLE fin_investment_lots (
  id                   SERIAL PRIMARY KEY,
  transaction_line_id  INT NOT NULL REFERENCES fin_transaction_lines(id) ON DELETE CASCADE,
  security_id          INT NOT NULL REFERENCES fin_securities(id),
  quantity             NUMERIC(18,4) NOT NULL,
  price                NUMERIC(18,4) NOT NULL,
  deleted_at           TIMESTAMP NULL
);
COMMENT ON TABLE fin_investment_lots IS '投资分录的数量与成交价，支持持仓与成本核算';
CREATE INDEX idx_fin_investment_lots_deleted_at ON fin_investment_lots(deleted_at);

-- 价格历史
CREATE TABLE fin_security_prices (
  security_id  INT NOT NULL REFERENCES fin_securities(id),
  price_at     DATE NOT NULL,
  close_price  NUMERIC(18,4) NOT NULL,
  deleted_at   TIMESTAMP NULL,
  PRIMARY KEY (security_id, price_at)
);
COMMENT ON TABLE fin_security_prices IS '标的每日收盘价，用于任意时点估值';
CREATE INDEX idx_fin_security_prices_deleted_at ON fin_security_prices(deleted_at);
