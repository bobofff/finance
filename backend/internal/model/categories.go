package model

import "gorm.io/gorm"

type CategoryKind string

const (
	CategoryKindIncome     CategoryKind = "income"
	CategoryKindExpense    CategoryKind = "expense"
	CategoryKindTransfer   CategoryKind = "transfer"
	CategoryKindInvestment CategoryKind = "investment"
)

// IsValid reports whether the kind is one of the allowed category types.
func (k CategoryKind) IsValid() bool {
	switch k {
	case CategoryKindIncome, CategoryKindExpense, CategoryKindTransfer, CategoryKindInvestment:
		return true
	default:
		return false
	}
}

type Category struct {
	ID        int            `gorm:"primaryKey;column:id"`
	LedgerID  int            `gorm:"column:ledger_id;not null"`
	Name      string         `gorm:"column:name;not null"`
	Kind      CategoryKind   `gorm:"column:kind;not null;check:kind IN ('income','expense','transfer','investment')"`
	ParentID  *int           `gorm:"column:parent_id"`
	Parent    *Category      `gorm:"foreignKey:ParentID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:RESTRICT"`
	Children  []Category     `gorm:"foreignKey:ParentID;references:ID"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (Category) TableName() string {
	return "fin_categories"
}
