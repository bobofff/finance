package model

import (
	"time"

	"gorm.io/gorm"
)

type StrategyTemplate struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	LedgerID  int            `gorm:"column:ledger_id;not null;default:1;index" json:"ledger_id"`
	Name      string         `gorm:"column:name;not null" json:"name"`
	Kind      string         `gorm:"column:kind;not null" json:"kind"`
	Params    string         `gorm:"column:params;type:jsonb;not null;default:'{}'" json:"params"`
	IsActive  bool           `gorm:"column:is_active;not null;default:true" json:"is_active"`
	CreatedAt time.Time      `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

func (StrategyTemplate) TableName() string {
	return "fin_strategy_templates"
}

func SeedStrategyTemplates(db *gorm.DB) error {
	var count int64
	if err := db.Model(&StrategyTemplate{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	template := StrategyTemplate{
		LedgerID: 1,
		Name:     "打野-回撤反弹(半结构化)",
		Kind:     "mean_reversion",
		Params:   defaultStrategyParams(),
		IsActive: true,
	}
	return db.Create(&template).Error
}

func defaultStrategyParams() string {
	return `{
  "version": "v1",
  "profile": {
    "market": "A股ETF",
    "style": "回撤反弹",
    "timeframe": "日线",
    "holding_period_days": 120
  },
  "entry": {
    "trigger": "收盘价站回5日均线之上",
    "timing": "次日开盘买入",
    "filters": [
      "最近N日出现明显回撤",
      "量能不低于5日均量（可选）"
    ],
    "position_size_pct": 0.1
  },
  "exit": {
    "stop_loss_pct": 0.03,
    "take_profit_pct": 0.05,
    "take_profit_pct_alt": 0.06,
    "time_stop_days": 120
  },
  "risk": {
    "max_positions": 3,
    "max_loss_pct_per_trade": 0.03
  },
  "ops": {
    "update_freq": "每日收盘后",
    "price_source": "新浪/腾讯"
  },
  "notes": "半结构化模板：可按需要扩展字段。"
}`
}
