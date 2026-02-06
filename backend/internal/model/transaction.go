package model

import (
	"time"

	"gorm.io/gorm"
)

type Transaction struct {
	ID          uint           `gorm:"primaryKey"`
	LedgerID    int            `gorm:"column:ledger_id;not null;default:1"`
	OccurredOn  time.Time      `gorm:"column:occurred_on;type:date;not null"`
	Description string         `gorm:"column:description"`
	Note        string         `gorm:"column:note"`
	CreatedAt   time.Time      `gorm:"column:created_at;autoCreateTime"`
	DeletedAt   gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (Transaction) TableName() string {
	return "fin_transactions"
}

type TransactionLine struct {
	ID            uint           `gorm:"primaryKey"`
	LedgerID      int            `gorm:"column:ledger_id;not null;default:1"`
	TransactionID uint           `gorm:"column:transaction_id;not null"`
	AccountID     uint           `gorm:"column:account_id;not null"`
	CategoryID    *int           `gorm:"column:category_id"`
	Amount        float64        `gorm:"column:amount;not null"`
	Tags          []string       `gorm:"column:tags;type:text[]"`
	Note          string         `gorm:"column:note"`
	DeletedAt     gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (TransactionLine) TableName() string {
	return "fin_transaction_lines"
}
