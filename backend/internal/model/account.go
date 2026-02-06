package model

import (
	"time"

	"gorm.io/gorm"
)

type Account struct {
	ID        uint           `gorm:"primaryKey"`
	Name      string         `gorm:"column:name;not null"`
	Type      string         `gorm:"column:type;not null"`
	Currency  string         `gorm:"column:currency;not null;default:CNY"`
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
		&Category{},
		&Transaction{},
		&TransactionLine{},
		&Security{},
		&InvestmentLot{},
		&InvestmentSale{},
		&InvestmentLotAllocation{},
		&SecurityPrice{},
	)
}
