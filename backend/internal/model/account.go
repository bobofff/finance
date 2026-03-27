package model

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

type CashKind string

const (
	CashKindBank   CashKind = "bank"
	CashKindBroker CashKind = "broker"
)

func NormalizeCashKind(input string) (CashKind, bool) {
	value := strings.ToLower(strings.TrimSpace(input))
	switch value {
	case string(CashKindBank):
		return CashKindBank, true
	case string(CashKindBroker):
		return CashKindBroker, true
	default:
		return "", false
	}
}

func CoalesceCashKind(value string) CashKind {
	if kind, ok := NormalizeCashKind(value); ok {
		return kind
	}
	return CashKindBank
}

type Account struct {
	ID        uint           `gorm:"primaryKey"`
	LedgerID  int            `gorm:"column:ledger_id;not null;default:1;index"`
	Name      string         `gorm:"column:name;not null"`
	Type      string         `gorm:"column:type;not null"`
	Currency  string         `gorm:"column:currency;not null;default:CNY"`
	CashKind  CashKind       `gorm:"column:cash_kind;default:bank"`
	IsActive  bool           `gorm:"column:is_active;not null;default:true"`
	CreatedAt time.Time      `gorm:"column:created_at;autoCreateTime"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (Account) TableName() string {
	return "fin_accounts"
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&Account{},
		&AccountSnapshot{},
		&AnnualAssetGoal{},
		&Category{},
		&Transaction{},
		&TransactionLine{},
		&Security{},
		&InvestmentLot{},
		&InvestmentSale{},
		&InvestmentLotAllocation{},
		&SecurityPrice{},
		&SecurityIndicator{},
		&StrategyTemplate{},
	)
}
