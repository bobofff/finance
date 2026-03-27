package model

import (
	"time"

	"gorm.io/gorm"
)

// AnnualAssetGoal 定义年度净资产目标。
type AnnualAssetGoal struct {
	ID             uint           `gorm:"primaryKey"`
	LedgerID       int            `gorm:"column:ledger_id;not null;default:1;uniqueIndex:idx_annual_asset_goal_ledger_year"`
	Year           int            `gorm:"column:year;not null;uniqueIndex:idx_annual_asset_goal_ledger_year"`
	TargetNetWorth float64        `gorm:"column:target_net_worth;not null"`
	BaselineDate   time.Time      `gorm:"column:baseline_date;type:date;not null"`
	Note           string         `gorm:"column:note"`
	CreatedAt      time.Time      `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time      `gorm:"column:updated_at;autoUpdateTime"`
	DeletedAt      gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (AnnualAssetGoal) TableName() string {
	return "fin_annual_asset_goals"
}
