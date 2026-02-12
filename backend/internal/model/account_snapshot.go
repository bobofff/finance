package model

import (
	"time"

	"gorm.io/gorm"
)

type AccountSnapshot struct {
	ID        uint           `gorm:"primaryKey"`
	LedgerID  int            `gorm:"column:ledger_id;not null;default:1;index"`
	AccountID uint           `gorm:"column:account_id;not null;index"`
	AsOf      time.Time      `gorm:"column:as_of;type:date;not null;index"`
	Amount    float64        `gorm:"column:amount;not null"`
	Note      string         `gorm:"column:note"`
	CreatedAt time.Time      `gorm:"column:created_at;autoCreateTime"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (AccountSnapshot) TableName() string {
	return "fin_account_snapshots"
}
