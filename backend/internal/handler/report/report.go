package report

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"finance-backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	db *gorm.DB
}

func RegisterRoutes(rg *gin.RouterGroup, db *gorm.DB) {
	h := Handler{db: db}
	rg.GET("/balance-sheet", h.balanceSheet)
}

type balanceSheetAccount struct {
	ID       uint    `json:"id"`
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Currency string  `json:"currency"`
	IsActive bool    `json:"is_active"`
	Balance  float64 `json:"balance"`
}

type balanceSheetGroup struct {
	Key      string                `json:"key"`
	Label    string                `json:"label"`
	Total    float64               `json:"total"`
	Accounts []balanceSheetAccount `json:"accounts"`
}

type balanceSheetResponse struct {
	LedgerID int                 `json:"ledger_id"`
	AsOf     string              `json:"as_of"`
	Totals   map[string]float64  `json:"totals"`
	Groups   []balanceSheetGroup `json:"groups"`
}

type snapshotRow struct {
	AccountID uint
	AsOf      time.Time
	Amount    float64
}

func (h Handler) balanceSheet(c *gin.Context) {
	ledgerID := 1
	if value := strings.TrimSpace(c.Query("ledger_id")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ledger_id"})
			return
		}
		ledgerID = parsed
	}

	asOf := time.Now()
	if value := strings.TrimSpace(c.Query("as_of")); value != "" {
		parsed, err := time.ParseInLocation("2006-01-02", value, time.Local)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "as_of must be YYYY-MM-DD"})
			return
		}
		asOf = parsed
	}

	var accounts []model.Account
	if err := h.db.Where("ledger_id = ?", ledgerID).Order("id").Find(&accounts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query accounts"})
		return
	}

	var snapshots []snapshotRow
	query := `
SELECT s.account_id, s.as_of, SUM(s.amount) AS amount
FROM fin_account_snapshots s
JOIN (
  SELECT account_id, MAX(as_of) AS max_asof
  FROM fin_account_snapshots
  WHERE ledger_id = ? AND as_of <= ?
  GROUP BY account_id
) latest
ON s.account_id = latest.account_id AND s.as_of = latest.max_asof
WHERE s.ledger_id = ?
GROUP BY s.account_id, s.as_of`

	if err := h.db.Raw(query, ledgerID, asOf, ledgerID).Scan(&snapshots).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query snapshots"})
		return
	}

	snapshotMap := make(map[uint]snapshotRow, len(snapshots))
	for _, row := range snapshots {
		snapshotMap[row.AccountID] = row
	}

	groups := map[string]*balanceSheetGroup{
		"asset":     {Key: "asset", Label: "资产"},
		"liability": {Key: "liability", Label: "负债"},
		"other":     {Key: "other", Label: "其他"},
	}

	totalAssets := 0.0
	totalLiabilities := 0.0

	for _, account := range accounts {
		snapshot := snapshotMap[account.ID]
		sum := 0.0

		tx := h.db.Table("fin_transaction_lines").
			Joins("JOIN fin_transactions t ON t.id = fin_transaction_lines.transaction_id AND t.deleted_at IS NULL").
			Where("fin_transaction_lines.account_id = ? AND fin_transaction_lines.ledger_id = ? AND fin_transaction_lines.deleted_at IS NULL", account.ID, ledgerID).
			Select("COALESCE(SUM(fin_transaction_lines.amount), 0)")

		if !snapshot.AsOf.IsZero() {
			tx = tx.Where("t.occurred_on > ? AND t.occurred_on <= ?", snapshot.AsOf, asOf)
		} else {
			tx = tx.Where("t.occurred_on <= ?", asOf)
		}

		if err := tx.Scan(&sum).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query transactions"})
			return
		}

		balance := snapshot.Amount + sum

		entry := balanceSheetAccount{
			ID:       account.ID,
			Name:     account.Name,
			Type:     account.Type,
			Currency: account.Currency,
			IsActive: account.IsActive,
			Balance:  balance,
		}

		groupKey := classifyAccountType(account.Type)
		group := groups[groupKey]
		group.Accounts = append(group.Accounts, entry)
		group.Total += balance

		if groupKey == "asset" {
			totalAssets += balance
		} else if groupKey == "liability" {
			totalLiabilities += balance
		}
	}

	resp := balanceSheetResponse{
		LedgerID: ledgerID,
		AsOf:     asOf.Format("2006-01-02"),
		Totals: map[string]float64{
			"assets":      totalAssets,
			"liabilities": totalLiabilities,
			"net_worth":   totalAssets - totalLiabilities,
		},
		Groups: []balanceSheetGroup{
			*groups["asset"],
			*groups["liability"],
			*groups["other"],
		},
	}

	c.JSON(http.StatusOK, resp)
}

func classifyAccountType(accountType string) string {
	switch strings.ToLower(strings.TrimSpace(accountType)) {
	case "cash", "investment", "other_asset":
		return "asset"
	case "debt":
		return "asset"
	case "liability":
		return "liability"
	default:
		return "other"
	}
}
