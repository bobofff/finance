package model

import (
	"time"

	"gorm.io/gorm"
)

type Security struct {
	ID        uint           `gorm:"primaryKey"`
	LedgerID  int            `gorm:"column:ledger_id;not null;default:1;uniqueIndex:idx_security_ledger_ticker"`
	Ticker    string         `gorm:"column:ticker;not null;unique;uniqueIndex:idx_security_ledger_ticker"`
	Name      string         `gorm:"column:name;not null"`
	Currency  string         `gorm:"column:currency;not null;default:CNY"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (Security) TableName() string {
	return "fin_securities"
}

type InvestmentLot struct {
	ID                uint           `gorm:"primaryKey"`
	LedgerID          int            `gorm:"column:ledger_id;not null;default:1"`
	TransactionLineID uint           `gorm:"column:transaction_line_id;not null"`
	SecurityID        uint           `gorm:"column:security_id;not null"`
	Quantity          float64        `gorm:"column:quantity;not null"`
	Price             float64        `gorm:"column:price;not null"` // 成本价
	TradePrice        float64        `gorm:"column:trade_price;not null;default:0"`
	Fee               float64        `gorm:"column:fee;not null;default:0"`
	Tax               float64        `gorm:"column:tax;not null;default:0"`
	DeletedAt         gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (InvestmentLot) TableName() string {
	return "fin_investment_lots"
}

type InvestmentSale struct {
	ID                uint           `gorm:"primaryKey"`
	LedgerID          int            `gorm:"column:ledger_id;not null;default:1"`
	TransactionLineID uint           `gorm:"column:transaction_line_id;not null"`
	SecurityID        uint           `gorm:"column:security_id;not null"`
	Quantity          float64        `gorm:"column:quantity;not null"`
	Price             float64        `gorm:"column:price;not null"`
	DeletedAt         gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (InvestmentSale) TableName() string {
	return "fin_investment_sales"
}

type InvestmentLotAllocation struct {
	ID        uint           `gorm:"primaryKey"`
	LedgerID  int            `gorm:"column:ledger_id;not null;default:1"`
	BuyLotID  uint           `gorm:"column:buy_lot_id;not null"`
	SaleID    uint           `gorm:"column:sale_id;not null"`
	Quantity  float64        `gorm:"column:quantity;not null"`
	CreatedAt time.Time      `gorm:"column:created_at;autoCreateTime"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (InvestmentLotAllocation) TableName() string {
	return "fin_investment_lot_allocations"
}

type SecurityPrice struct {
	LedgerID   int            `gorm:"column:ledger_id;primaryKey"`
	SecurityID uint           `gorm:"column:security_id;primaryKey"`
	PriceAt    time.Time      `gorm:"column:price_at;type:date;primaryKey"`
	ClosePrice float64        `gorm:"column:close_price;not null"`
	DeletedAt  gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (SecurityPrice) TableName() string {
	return "fin_security_prices"
}
