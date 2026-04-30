package investment

import (
	"testing"
	"time"

	"finance-backend/internal/model"
)

func TestDedupeSecurityPricesKeepsFirstRecordPerSecurityDate(t *testing.T) {
	baseDay := time.Date(2026, time.April, 22, 0, 0, 0, 0, time.Local)
	otherDay := baseDay.AddDate(0, 0, -1)

	records := []model.SecurityPrice{
		{LedgerID: 1, SecurityID: 7, PriceAt: baseDay, ClosePrice: 1.413},
		{LedgerID: 1, SecurityID: 7, PriceAt: otherDay, ClosePrice: 1.342},
		{LedgerID: 1, SecurityID: 7, PriceAt: baseDay, ClosePrice: 1.401},
		{LedgerID: 1, SecurityID: 2, PriceAt: baseDay, ClosePrice: 10.004},
		{LedgerID: 2, SecurityID: 7, PriceAt: baseDay, ClosePrice: 9.999},
	}

	got := dedupeSecurityPrices(records)

	if len(got) != 4 {
		t.Fatalf("expected 4 records after dedupe, got %d", len(got))
	}

	if got[0].ClosePrice != 1.413 {
		t.Fatalf("expected first duplicate to be kept, got close_price=%v", got[0].ClosePrice)
	}

	if got[1].PriceAt.Format("2006-01-02") != "2026-04-21" {
		t.Fatalf("expected distinct historical day to remain, got %s", got[1].PriceAt.Format("2006-01-02"))
	}

	if got[2].SecurityID != 2 || got[3].LedgerID != 2 {
		t.Fatalf("expected records from other security and ledger to remain, got %#v", got)
	}
}
