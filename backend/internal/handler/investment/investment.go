package investment

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"finance-backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Handler struct {
	db *gorm.DB
}

func RegisterRoutes(rg *gin.RouterGroup, db *gorm.DB) {
	h := Handler{db: db}

	rg.GET("/lots", h.listLots)
	rg.POST("/buys", h.createBuy)
	rg.PATCH("/buys/:id", h.updateBuy)
	rg.DELETE("/buys/:id", h.deleteBuy)
	rg.POST("/sales", h.createSale)
	rg.POST("/prices/refresh", h.refreshPrices)
	rg.GET("/strategies", h.listStrategies)
	rg.POST("/strategies", h.createStrategy)
	rg.PATCH("/strategies/:id", h.updateStrategy)
	rg.DELETE("/strategies/:id", h.deleteStrategy)
}

type lotRow struct {
	LotID             uint      `gorm:"column:lot_id"`
	LedgerID          int       `gorm:"column:ledger_id"`
	SecurityID        uint      `gorm:"column:security_id"`
	SecurityTicker    string    `gorm:"column:security_ticker"`
	SecurityName      string    `gorm:"column:security_name"`
	Quantity          float64   `gorm:"column:quantity"`
	Price             float64   `gorm:"column:price"`
	TradePrice        float64   `gorm:"column:trade_price"`
	Tag               string    `gorm:"column:tag"`
	Fee               float64   `gorm:"column:fee"`
	Tax               float64   `gorm:"column:tax"`
	TransactionLineID uint      `gorm:"column:transaction_line_id"`
	TransactionID     uint      `gorm:"column:transaction_id"`
	OccurredOn        time.Time `gorm:"column:occurred_on"`
	AllocatedQuantity float64   `gorm:"column:allocated_quantity"`
	RemainingQuantity float64   `gorm:"column:remaining_quantity"`
	CurrentPrice      float64   `gorm:"column:current_price"`
	MA5               *float64  `gorm:"column:ma_5"`
	High55            *float64  `gorm:"column:high_55"`
	High20            *float64  `gorm:"column:high_20"`
	Low10             *float64  `gorm:"column:low_10"`
	Low20             *float64  `gorm:"column:low_20"`
}

type lotResponse struct {
	LotID             uint     `json:"lot_id"`
	LedgerID          int      `json:"ledger_id"`
	SecurityID        uint     `json:"security_id"`
	SecurityTicker    string   `json:"security_ticker"`
	SecurityName      string   `json:"security_name"`
	Quantity          float64  `json:"quantity"`
	Price             float64  `json:"price"`
	TradePrice        float64  `json:"trade_price"`
	Tag               string   `json:"tag"`
	Fee               float64  `json:"fee"`
	Tax               float64  `json:"tax"`
	TransactionLineID uint     `json:"transaction_line_id"`
	TransactionID     uint     `json:"transaction_id"`
	OccurredOn        string   `json:"occurred_on"`
	AllocatedQuantity float64  `json:"allocated_quantity"`
	RemainingQuantity float64  `json:"remaining_quantity"`
	Status            string   `json:"status"`
	CurrentPrice      float64  `json:"current_price"`
	MA5               *float64 `json:"ma_5"`
	High55            *float64 `json:"high_55"`
	High20            *float64 `json:"high_20"`
	Low10             *float64 `json:"low_10"`
	Low20             *float64 `json:"low_20"`
}

func (h Handler) listLots(c *gin.Context) {
	ledgerID := 1
	if value := strings.TrimSpace(c.Query("ledger_id")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ledger_id"})
			return
		}
		ledgerID = parsed
	}

	var securityID uint
	if value := strings.TrimSpace(c.Query("security_id")); value != "" {
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil || parsed == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid security_id"})
			return
		}
		securityID = uint(parsed)
	}

	var cashAccountID uint
	if value := strings.TrimSpace(c.Query("cash_account_id")); value != "" {
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil || parsed == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cash_account_id"})
			return
		}
		cashAccountID = uint(parsed)
	}

	status := strings.ToLower(strings.TrimSpace(c.Query("status")))
	if status != "" && status != "open" && status != "closed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status must be open or closed"})
		return
	}
	tag := strings.TrimSpace(c.Query("tag"))
	if tag != "" && !validateLotTag(tag, c) {
		return
	}
	keyword := strings.TrimSpace(c.Query("keyword"))
	buyDateFrom, err := parseDateQuery(c.Query("buy_date_from"), c)
	if err != nil {
		return
	}
	buyDateTo, err := parseDateQuery(c.Query("buy_date_to"), c)
	if err != nil {
		return
	}
	if !buyDateFrom.IsZero() && !buyDateTo.IsZero() && buyDateFrom.After(buyDateTo) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "buy_date_from cannot be after buy_date_to"})
		return
	}

	page := parsePage(c.Query("page"))
	pageSize := parsePageSize(c.Query("page_size"))
	offset := (page - 1) * pageSize

	filters := []string{"l.deleted_at IS NULL", "l.ledger_id = ?"}
	filterArgs := []interface{}{ledgerID}

	if securityID != 0 {
		filters = append(filters, "l.security_id = ?")
		filterArgs = append(filterArgs, securityID)
	}
	if tag != "" {
		filters = append(filters, "l.tag = ?")
		filterArgs = append(filterArgs, tag)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		filters = append(filters, "(s.ticker ILIKE ? OR s.name ILIKE ?)")
		filterArgs = append(filterArgs, like, like)
	}
	if cashAccountID != 0 {
		filters = append(filters, `EXISTS (
  SELECT 1 FROM fin_transaction_lines tcl
  WHERE tcl.transaction_id = tl.transaction_id
    AND tcl.deleted_at IS NULL
    AND tcl.account_id = ?
)`)
		filterArgs = append(filterArgs, cashAccountID)
	}
	if !buyDateFrom.IsZero() {
		filters = append(filters, "t.occurred_on >= ?")
		filterArgs = append(filterArgs, buyDateFrom)
	}
	if !buyDateTo.IsZero() {
		filters = append(filters, "t.occurred_on <= ?")
		filterArgs = append(filterArgs, buyDateTo)
	}

	whereClause := "WHERE " + strings.Join(filters, " AND ")
	havingClause := ""
	if status == "open" {
		havingClause = "HAVING (l.quantity - COALESCE(SUM(a.quantity), 0)) > 0"
	} else if status == "closed" {
		havingClause = "HAVING (l.quantity - COALESCE(SUM(a.quantity), 0)) <= 0"
	}

	countQuery := `
SELECT COUNT(*) FROM (
  SELECT l.id
  FROM fin_investment_lots l
  JOIN fin_transaction_lines tl ON tl.id = l.transaction_line_id AND tl.deleted_at IS NULL
  JOIN fin_transactions t ON t.id = tl.transaction_id AND t.deleted_at IS NULL
  JOIN fin_securities s ON s.id = l.security_id AND s.deleted_at IS NULL
  LEFT JOIN fin_investment_lot_allocations a ON a.buy_lot_id = l.id AND a.deleted_at IS NULL
  ` + whereClause + `
  GROUP BY l.id, t.id, tl.id, l.quantity
  ` + havingClause + `
) sub`

	var total int64
	if err := h.db.Raw(countQuery, filterArgs...).Scan(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count lots"})
		return
	}

	type summaryRow struct {
		LotCount        int64   `gorm:"column:lot_count"`
		TotalQuantity   float64 `gorm:"column:total_quantity"`
		TotalCost       float64 `gorm:"column:total_cost"`
		TotalCostPriced float64 `gorm:"column:total_cost_priced"`
		TotalMarket     float64 `gorm:"column:total_market"`
		PricedCount     int64   `gorm:"column:priced_count"`
	}

	summaryQuery := `
SELECT
  COUNT(*) AS lot_count,
  COALESCE(SUM(x.quantity), 0) AS total_quantity,
  COALESCE(SUM(x.price * x.quantity), 0) AS total_cost,
  COALESCE(SUM(CASE WHEN x.close_price IS NOT NULL THEN x.price * x.quantity ELSE 0 END), 0) AS total_cost_priced,
  COALESCE(SUM(CASE WHEN x.close_price IS NOT NULL THEN x.close_price * x.quantity ELSE 0 END), 0) AS total_market,
  COALESCE(SUM(CASE WHEN x.close_price IS NOT NULL THEN 1 ELSE 0 END), 0) AS priced_count
FROM (
  SELECT l.id, l.quantity, l.price, cp.close_price
  FROM fin_investment_lots l
  JOIN fin_transaction_lines tl ON tl.id = l.transaction_line_id AND tl.deleted_at IS NULL
  JOIN fin_transactions t ON t.id = tl.transaction_id AND t.deleted_at IS NULL
  JOIN fin_securities s ON s.id = l.security_id AND s.deleted_at IS NULL
  LEFT JOIN (
    SELECT p.security_id, p.close_price
    FROM fin_security_prices p
    JOIN (
      SELECT security_id, MAX(price_at) AS max_price_at
      FROM fin_security_prices
      WHERE ledger_id = ? AND deleted_at IS NULL
      GROUP BY security_id
    ) latest
    ON p.security_id = latest.security_id AND p.price_at = latest.max_price_at
    WHERE p.ledger_id = ? AND p.deleted_at IS NULL
  ) cp ON cp.security_id = l.security_id
  LEFT JOIN fin_investment_lot_allocations a ON a.buy_lot_id = l.id AND a.deleted_at IS NULL
  ` + whereClause + `
  GROUP BY l.id, l.quantity, l.price, cp.close_price, t.id, tl.id
  ` + havingClause + `
) x`

	var summary summaryRow
	summaryArgs := []interface{}{ledgerID, ledgerID}
	summaryArgs = append(summaryArgs, filterArgs...)
	if err := h.db.Raw(summaryQuery, summaryArgs...).Scan(&summary).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to summarize lots"})
		return
	}

	hasMarket := summary.PricedCount > 0
	profit := 0.0
	profitPct := 0.0
	if hasMarket {
		profit = summary.TotalMarket - summary.TotalCostPriced
		if summary.TotalCostPriced > 0 {
			profitPct = profit / summary.TotalCostPriced
		}
	}

	query := `
SELECT
  l.id AS lot_id,
  l.ledger_id,
  l.security_id,
  s.ticker AS security_ticker,
  s.name AS security_name,
  l.quantity,
  l.price,
  l.trade_price,
  l.tag,
  l.fee,
  l.tax,
  tl.id AS transaction_line_id,
  t.id AS transaction_id,
  t.occurred_on,
  COALESCE(SUM(a.quantity), 0) AS allocated_quantity,
  (l.quantity - COALESCE(SUM(a.quantity), 0)) AS remaining_quantity,
  cp.close_price AS current_price,
  ind.ma_5,
  ind.high_55,
  ind.high_20,
  ind.low_10,
  ind.low_20
FROM fin_investment_lots l
JOIN fin_transaction_lines tl ON tl.id = l.transaction_line_id AND tl.deleted_at IS NULL
JOIN fin_transactions t ON t.id = tl.transaction_id AND t.deleted_at IS NULL
JOIN fin_securities s ON s.id = l.security_id AND s.deleted_at IS NULL
LEFT JOIN (
  SELECT p.security_id, p.close_price
  FROM fin_security_prices p
  JOIN (
    SELECT security_id, MAX(price_at) AS max_price_at
    FROM fin_security_prices
    WHERE ledger_id = ? AND deleted_at IS NULL
    GROUP BY security_id
  ) latest
  ON p.security_id = latest.security_id AND p.price_at = latest.max_price_at
  WHERE p.ledger_id = ? AND p.deleted_at IS NULL
) cp ON cp.security_id = l.security_id
LEFT JOIN (
  SELECT i.security_id, i.ma_5, i.high_55, i.high_20, i.low_10, i.low_20
  FROM fin_security_indicators i
  JOIN (
    SELECT security_id, MAX(as_of) AS max_as_of
    FROM fin_security_indicators
    WHERE ledger_id = ? AND deleted_at IS NULL
    GROUP BY security_id
  ) latest_i
  ON i.security_id = latest_i.security_id AND i.as_of = latest_i.max_as_of
  WHERE i.ledger_id = ? AND i.deleted_at IS NULL
) ind ON ind.security_id = l.security_id
LEFT JOIN fin_investment_lot_allocations a ON a.buy_lot_id = l.id AND a.deleted_at IS NULL
` + whereClause

	query += " GROUP BY l.id, s.id, tl.id, t.id, l.tag, cp.close_price, ind.ma_5, ind.high_55, ind.high_20, ind.low_10, ind.low_20 " + havingClause + " ORDER BY s.ticker DESC, t.occurred_on DESC, l.id DESC LIMIT ? OFFSET ?"

	args := []interface{}{ledgerID, ledgerID, ledgerID, ledgerID}
	args = append(args, filterArgs...)
	args = append(args, pageSize, offset)

	var rows []lotRow
	if err := h.db.Raw(query, args...).Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query lots"})
		return
	}

	resp := make([]lotResponse, 0, len(rows))
	for _, row := range rows {
		state := "open"
		if row.RemainingQuantity <= 0 {
			state = "closed"
		}
		resp = append(resp, lotResponse{
			LotID:             row.LotID,
			LedgerID:          row.LedgerID,
			SecurityID:        row.SecurityID,
			SecurityTicker:    row.SecurityTicker,
			SecurityName:      row.SecurityName,
			Quantity:          row.Quantity,
			Price:             row.Price,
			TradePrice:        row.TradePrice,
			Tag:               row.Tag,
			Fee:               row.Fee,
			Tax:               row.Tax,
			TransactionLineID: row.TransactionLineID,
			TransactionID:     row.TransactionID,
			OccurredOn:        row.OccurredOn.Format("2006-01-02"),
			AllocatedQuantity: row.AllocatedQuantity,
			RemainingQuantity: row.RemainingQuantity,
			Status:            state,
			CurrentPrice:      row.CurrentPrice,
			MA5:               row.MA5,
			High55:            row.High55,
			High20:            row.High20,
			Low10:             row.Low10,
			Low20:             row.Low20,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  resp,
		"total": total,
		"summary": gin.H{
			"total_quantity": summary.TotalQuantity,
			"total_cost":     summary.TotalCost,
			"total_market":   summary.TotalMarket,
			"profit":         profit,
			"profit_pct":     profitPct,
			"has_market":     hasMarket,
			"partial_market": hasMarket && summary.PricedCount != summary.LotCount,
		},
	})
}

type saleAllocation struct {
	BuyLotID uint    `json:"buy_lot_id" binding:"required,gt=0"`
	Quantity float64 `json:"quantity" binding:"required,gt=0"`
}

type createSaleRequest struct {
	LedgerID            *int             `json:"ledger_id"`
	OccurredOn          string           `json:"occurred_on" binding:"required"`
	SecurityID          uint             `json:"security_id" binding:"required,gt=0"`
	CashAccountID       uint             `json:"cash_account_id" binding:"required,gt=0"`
	InvestmentAccountID uint             `json:"investment_account_id" binding:"required,gt=0"`
	Price               float64          `json:"price" binding:"required,gt=0"`
	Fee                 float64          `json:"fee"`
	FeeCategoryID       *int             `json:"fee_category_id"`
	Tax                 float64          `json:"tax"`
	TaxCategoryID       *int             `json:"tax_category_id"`
	Description         string           `json:"description"`
	Note                string           `json:"note"`
	Allocations         []saleAllocation `json:"allocations" binding:"required,min=1,dive"`
}

type createSaleResponse struct {
	TransactionID uint    `json:"transaction_id"`
	SaleID        uint    `json:"sale_id"`
	Quantity      float64 `json:"quantity"`
	Price         float64 `json:"price"`
	GrossAmount   float64 `json:"gross_amount"`
	CostAmount    float64 `json:"cost_amount"`
	Fee           float64 `json:"fee"`
	Tax           float64 `json:"tax"`
}

type createBuyRequest struct {
	LedgerID            *int    `json:"ledger_id"`
	OccurredOn          string  `json:"occurred_on" binding:"required"`
	SecurityID          *uint   `json:"security_id"`
	SecurityTicker      string  `json:"security_ticker"`
	SecurityName        string  `json:"security_name"`
	CashAccountID       uint    `json:"cash_account_id" binding:"required,gt=0"`
	InvestmentAccountID uint    `json:"investment_account_id" binding:"required,gt=0"`
	Quantity            float64 `json:"quantity" binding:"required,gt=0"`
	Price               float64 `json:"price" binding:"required,gt=0"`
	Tag                 string  `json:"tag"`
	Fee                 float64 `json:"fee"`
	FeeCategoryID       *int    `json:"fee_category_id"`
	Tax                 float64 `json:"tax"`
	TaxCategoryID       *int    `json:"tax_category_id"`
	Description         string  `json:"description"`
	Note                string  `json:"note"`
}

type createBuyResponse struct {
	TransactionID uint    `json:"transaction_id"`
	LotID         uint    `json:"lot_id"`
	Quantity      float64 `json:"quantity"`
	Price         float64 `json:"price"`
	CostPrice     float64 `json:"cost_price"`
	GrossAmount   float64 `json:"gross_amount"`
	CostAmount    float64 `json:"cost_amount"`
	Fee           float64 `json:"fee"`
	Tax           float64 `json:"tax"`
}

func (h Handler) createBuy(c *gin.Context) {
	var req createBuyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ledgerID := 1
	if req.LedgerID != nil {
		if *req.LedgerID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ledger_id must be positive"})
			return
		}
		ledgerID = *req.LedgerID
	}

	occurredOnRaw := strings.TrimSpace(req.OccurredOn)
	occurredOn, err := time.ParseInLocation("2006-01-02", occurredOnRaw, time.Local)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "occurred_on must be YYYY-MM-DD"})
		return
	}

	if req.Quantity <= 0 || req.Price <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "quantity and price must be greater than 0"})
		return
	}
	if req.Fee < 0 || req.Tax < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "fee and tax cannot be negative"})
		return
	}

	tag := strings.TrimSpace(req.Tag)
	if !validateLotTag(tag, c) {
		return
	}

	var response createBuyResponse

	err = h.db.Transaction(func(tx *gorm.DB) error {
		security, err := resolveSecurity(tx, ledgerID, req.SecurityID, req.SecurityTicker, req.SecurityName)
		if err != nil {
			return err
		}

		var cashAccount model.Account
		if err := tx.Where("id = ? AND ledger_id = ?", req.CashAccountID, ledgerID).First(&cashAccount).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return newRequestError("cash account not found")
			}
			return err
		}
		if !cashAccount.IsActive {
			return newRequestError("cash account is inactive")
		}
		if strings.ToLower(cashAccount.Type) != "cash" {
			return newRequestError("cash_account_id must be a cash account")
		}
		if model.CoalesceCashKind(string(cashAccount.CashKind)) != model.CashKindBroker {
			return newRequestError("cash_account_id must be a broker cash account")
		}

		var investmentAccount model.Account
		if err := tx.Where("id = ? AND ledger_id = ?", req.InvestmentAccountID, ledgerID).First(&investmentAccount).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return newRequestError("investment account not found")
			}
			return err
		}
		if !investmentAccount.IsActive {
			return newRequestError("investment account is inactive")
		}
		if strings.ToLower(investmentAccount.Type) != "investment" {
			return newRequestError("investment_account_id must be an investment account")
		}

		if req.FeeCategoryID != nil {
			if err := validateExpenseCategory(tx, ledgerID, *req.FeeCategoryID); err != nil {
				return err
			}
		}
		if req.TaxCategoryID != nil {
			if err := validateExpenseCategory(tx, ledgerID, *req.TaxCategoryID); err != nil {
				return err
			}
		}

		grossAmount := req.Quantity * req.Price
		costAmount := grossAmount + req.Fee + req.Tax
		costPrice := costAmount / req.Quantity

		txRecord := model.Transaction{
			LedgerID:    ledgerID,
			OccurredOn:  occurredOn,
			Description: strings.TrimSpace(req.Description),
			Note:        strings.TrimSpace(req.Note),
		}
		if err := tx.Create(&txRecord).Error; err != nil {
			return err
		}

		cashLine := model.TransactionLine{
			LedgerID:      ledgerID,
			TransactionID: txRecord.ID,
			AccountID:     req.CashAccountID,
			Amount:        -grossAmount,
		}
		if err := tx.Create(&cashLine).Error; err != nil {
			return err
		}

		if req.Fee > 0 {
			feeLine := model.TransactionLine{
				LedgerID:      ledgerID,
				TransactionID: txRecord.ID,
				AccountID:     req.CashAccountID,
				CategoryID:    req.FeeCategoryID,
				Amount:        -req.Fee,
			}
			if err := tx.Create(&feeLine).Error; err != nil {
				return err
			}
		}

		if req.Tax > 0 {
			taxLine := model.TransactionLine{
				LedgerID:      ledgerID,
				TransactionID: txRecord.ID,
				AccountID:     req.CashAccountID,
				CategoryID:    req.TaxCategoryID,
				Amount:        -req.Tax,
			}
			if err := tx.Create(&taxLine).Error; err != nil {
				return err
			}
		}

		investmentLine := model.TransactionLine{
			LedgerID:      ledgerID,
			TransactionID: txRecord.ID,
			AccountID:     req.InvestmentAccountID,
			Amount:        costAmount,
		}
		if err := tx.Create(&investmentLine).Error; err != nil {
			return err
		}

		lot := model.InvestmentLot{
			LedgerID:          ledgerID,
			TransactionLineID: investmentLine.ID,
			SecurityID:        security.ID,
			Quantity:          req.Quantity,
			Price:             costPrice,
			TradePrice:        req.Price,
			Tag:               tag,
			Fee:               req.Fee,
			Tax:               req.Tax,
		}
		if err := tx.Create(&lot).Error; err != nil {
			return err
		}

		response = createBuyResponse{
			TransactionID: txRecord.ID,
			LotID:         lot.ID,
			Quantity:      req.Quantity,
			Price:         req.Price,
			CostPrice:     costPrice,
			GrossAmount:   grossAmount,
			CostAmount:    costAmount,
			Fee:           req.Fee,
			Tax:           req.Tax,
		}

		return nil
	})

	if err != nil {
		var reqErr requestError
		if errors.As(err, &reqErr) {
			c.JSON(http.StatusBadRequest, gin.H{"error": reqErr.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create buy"})
		return
	}

	c.JSON(http.StatusCreated, response)
}

func (h Handler) updateBuy(c *gin.Context) {
	lotID, ok := parseUintID(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lot id"})
		return
	}

	var req createBuyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ledgerID := 1
	if req.LedgerID != nil {
		if *req.LedgerID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ledger_id must be positive"})
			return
		}
		ledgerID = *req.LedgerID
	}

	occurredOnRaw := strings.TrimSpace(req.OccurredOn)
	occurredOn, err := time.ParseInLocation("2006-01-02", occurredOnRaw, time.Local)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "occurred_on must be YYYY-MM-DD"})
		return
	}

	if req.Quantity <= 0 || req.Price <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "quantity and price must be greater than 0"})
		return
	}
	if req.Fee < 0 || req.Tax < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "fee and tax cannot be negative"})
		return
	}

	tag := strings.TrimSpace(req.Tag)
	if !validateLotTag(tag, c) {
		return
	}

	var response createBuyResponse

	err = h.db.Transaction(func(tx *gorm.DB) error {
		var lot model.InvestmentLot
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND ledger_id = ?", lotID, ledgerID).
			First(&lot).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return newRequestError("buy lot not found")
			}
			return err
		}

		var allocated float64
		if err := tx.Table("fin_investment_lot_allocations").
			Select("COALESCE(SUM(quantity), 0)").
			Where("buy_lot_id = ? AND deleted_at IS NULL", lotID).
			Scan(&allocated).Error; err != nil {
			return err
		}
		if allocated > 0 {
			return newRequestError("buy lot already allocated, cannot edit")
		}

		security, err := resolveSecurity(tx, ledgerID, req.SecurityID, req.SecurityTicker, req.SecurityName)
		if err != nil {
			return err
		}

		var investmentLine model.TransactionLine
		if err := tx.Where("id = ? AND ledger_id = ?", lot.TransactionLineID, ledgerID).First(&investmentLine).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return newRequestError("transaction line not found")
			}
			return err
		}

		var txRecord model.Transaction
		if err := tx.Where("id = ? AND ledger_id = ?", investmentLine.TransactionID, ledgerID).First(&txRecord).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return newRequestError("transaction not found")
			}
			return err
		}

		var cashAccount model.Account
		if err := tx.Where("id = ? AND ledger_id = ?", req.CashAccountID, ledgerID).First(&cashAccount).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return newRequestError("cash account not found")
			}
			return err
		}
		if !cashAccount.IsActive {
			return newRequestError("cash account is inactive")
		}
		if strings.ToLower(cashAccount.Type) != "cash" {
			return newRequestError("cash_account_id must be a cash account")
		}
		if model.CoalesceCashKind(string(cashAccount.CashKind)) != model.CashKindBroker {
			return newRequestError("cash_account_id must be a broker cash account")
		}

		var investmentAccount model.Account
		if err := tx.Where("id = ? AND ledger_id = ?", req.InvestmentAccountID, ledgerID).First(&investmentAccount).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return newRequestError("investment account not found")
			}
			return err
		}
		if !investmentAccount.IsActive {
			return newRequestError("investment account is inactive")
		}
		if strings.ToLower(investmentAccount.Type) != "investment" {
			return newRequestError("investment_account_id must be an investment account")
		}

		if req.FeeCategoryID != nil {
			if err := validateExpenseCategory(tx, ledgerID, *req.FeeCategoryID); err != nil {
				return err
			}
		}
		if req.TaxCategoryID != nil {
			if err := validateExpenseCategory(tx, ledgerID, *req.TaxCategoryID); err != nil {
				return err
			}
		}

		grossAmount := req.Quantity * req.Price
		costAmount := grossAmount + req.Fee + req.Tax
		costPrice := costAmount / req.Quantity

		txRecord.OccurredOn = occurredOn
		txRecord.Description = strings.TrimSpace(req.Description)
		txRecord.Note = strings.TrimSpace(req.Note)
		if err := tx.Save(&txRecord).Error; err != nil {
			return err
		}

		investmentLine.AccountID = req.InvestmentAccountID
		investmentLine.Amount = costAmount
		if err := tx.Save(&investmentLine).Error; err != nil {
			return err
		}

		if err := tx.Where("transaction_id = ? AND ledger_id = ? AND id <> ?", txRecord.ID, ledgerID, investmentLine.ID).
			Delete(&model.TransactionLine{}).Error; err != nil {
			return err
		}

		cashLine := model.TransactionLine{
			LedgerID:      ledgerID,
			TransactionID: txRecord.ID,
			AccountID:     req.CashAccountID,
			Amount:        -grossAmount,
		}
		if err := tx.Create(&cashLine).Error; err != nil {
			return err
		}

		if req.Fee > 0 {
			feeLine := model.TransactionLine{
				LedgerID:      ledgerID,
				TransactionID: txRecord.ID,
				AccountID:     req.CashAccountID,
				CategoryID:    req.FeeCategoryID,
				Amount:        -req.Fee,
			}
			if err := tx.Create(&feeLine).Error; err != nil {
				return err
			}
		}

		if req.Tax > 0 {
			taxLine := model.TransactionLine{
				LedgerID:      ledgerID,
				TransactionID: txRecord.ID,
				AccountID:     req.CashAccountID,
				CategoryID:    req.TaxCategoryID,
				Amount:        -req.Tax,
			}
			if err := tx.Create(&taxLine).Error; err != nil {
				return err
			}
		}

		lot.SecurityID = security.ID
		lot.Quantity = req.Quantity
		lot.Price = costPrice
		lot.TradePrice = req.Price
		lot.Tag = tag
		lot.Fee = req.Fee
		lot.Tax = req.Tax
		if err := tx.Save(&lot).Error; err != nil {
			return err
		}

		response = createBuyResponse{
			TransactionID: txRecord.ID,
			LotID:         lot.ID,
			Quantity:      req.Quantity,
			Price:         req.Price,
			CostPrice:     costPrice,
			GrossAmount:   grossAmount,
			CostAmount:    costAmount,
			Fee:           req.Fee,
			Tax:           req.Tax,
		}

		return nil
	})

	if err != nil {
		var reqErr requestError
		if errors.As(err, &reqErr) {
			c.JSON(http.StatusBadRequest, gin.H{"error": reqErr.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update buy"})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h Handler) deleteBuy(c *gin.Context) {
	lotID, ok := parseUintID(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lot id"})
		return
	}

	ledgerID := 1
	if value := strings.TrimSpace(c.Query("ledger_id")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ledger_id"})
			return
		}
		ledgerID = parsed
	}

	err := h.db.Transaction(func(tx *gorm.DB) error {
		var lot model.InvestmentLot
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND ledger_id = ?", lotID, ledgerID).
			First(&lot).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return newRequestError("buy lot not found")
			}
			return err
		}

		var allocated float64
		if err := tx.Table("fin_investment_lot_allocations").
			Select("COALESCE(SUM(quantity), 0)").
			Where("buy_lot_id = ? AND deleted_at IS NULL", lotID).
			Scan(&allocated).Error; err != nil {
			return err
		}
		if allocated > 0 {
			return newRequestError("buy lot already allocated, cannot delete")
		}

		var line model.TransactionLine
		if err := tx.Where("id = ? AND ledger_id = ?", lot.TransactionLineID, ledgerID).First(&line).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return newRequestError("transaction line not found")
			}
			return err
		}

		var cashAccountID uint
		if err := tx.Table("fin_transaction_lines tl").
			Joins("JOIN fin_accounts a ON a.id = tl.account_id AND a.deleted_at IS NULL").
			Where("tl.transaction_id = ? AND tl.ledger_id = ? AND tl.deleted_at IS NULL", line.TransactionID, ledgerID).
			Where("LOWER(a.type) = ?", "cash").
			Select("tl.account_id").
			Limit(1).
			Scan(&cashAccountID).Error; err != nil {
			return err
		}
		if cashAccountID == 0 {
			return newRequestError("cash account not found for buy lot")
		}

		refundAmount := line.Amount
		if refundAmount < 0 {
			refundAmount = -refundAmount
		}

		refundTx := model.Transaction{
			LedgerID:    ledgerID,
			OccurredOn:  time.Now(),
			Description: fmt.Sprintf("删除买入批次退款 %d", lotID),
		}
		if err := tx.Create(&refundTx).Error; err != nil {
			return err
		}

		refundCashLine := model.TransactionLine{
			LedgerID:      ledgerID,
			TransactionID: refundTx.ID,
			AccountID:     cashAccountID,
			Amount:        refundAmount,
		}
		if err := tx.Create(&refundCashLine).Error; err != nil {
			return err
		}

		refundInvestmentLine := model.TransactionLine{
			LedgerID:      ledgerID,
			TransactionID: refundTx.ID,
			AccountID:     line.AccountID,
			Amount:        -refundAmount,
		}
		if err := tx.Create(&refundInvestmentLine).Error; err != nil {
			return err
		}

		if err := tx.Delete(&model.InvestmentLot{}, lotID).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		var reqErr requestError
		if errors.As(err, &reqErr) {
			c.JSON(http.StatusBadRequest, gin.H{"error": reqErr.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete buy"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h Handler) createSale(c *gin.Context) {
	var req createSaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ledgerID := 1
	if req.LedgerID != nil {
		if *req.LedgerID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ledger_id must be positive"})
			return
		}
		ledgerID = *req.LedgerID
	}

	occurredOnRaw := strings.TrimSpace(req.OccurredOn)
	occurredOn, err := time.ParseInLocation("2006-01-02", occurredOnRaw, time.Local)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "occurred_on must be YYYY-MM-DD"})
		return
	}

	if req.Price <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "price must be greater than 0"})
		return
	}
	if req.Fee < 0 || req.Tax < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "fee and tax cannot be negative"})
		return
	}

	allocationMap := make(map[uint]float64)
	for _, alloc := range req.Allocations {
		if alloc.Quantity <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "allocation quantity must be greater than 0"})
			return
		}
		allocationMap[alloc.BuyLotID] += alloc.Quantity
	}
	if len(allocationMap) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "allocations cannot be empty"})
		return
	}

	lotIDs := make([]uint, 0, len(allocationMap))
	for id := range allocationMap {
		lotIDs = append(lotIDs, id)
	}
	sort.Slice(lotIDs, func(i, j int) bool { return lotIDs[i] < lotIDs[j] })

	var response createSaleResponse

	err = h.db.Transaction(func(tx *gorm.DB) error {
		var security model.Security
		if err := tx.Where("id = ? AND ledger_id = ?", req.SecurityID, ledgerID).First(&security).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return newRequestError("security not found")
			}
			return err
		}

		var cashAccount model.Account
		if err := tx.Where("id = ? AND ledger_id = ?", req.CashAccountID, ledgerID).First(&cashAccount).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return newRequestError("cash account not found")
			}
			return err
		}
		if !cashAccount.IsActive {
			return newRequestError("cash account is inactive")
		}
		if strings.ToLower(cashAccount.Type) != "cash" {
			return newRequestError("cash_account_id must be a cash account")
		}
		if model.CoalesceCashKind(string(cashAccount.CashKind)) != model.CashKindBroker {
			return newRequestError("cash_account_id must be a broker cash account")
		}

		var investmentAccount model.Account
		if err := tx.Where("id = ? AND ledger_id = ?", req.InvestmentAccountID, ledgerID).First(&investmentAccount).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return newRequestError("investment account not found")
			}
			return err
		}
		if !investmentAccount.IsActive {
			return newRequestError("investment account is inactive")
		}
		if strings.ToLower(investmentAccount.Type) != "investment" {
			return newRequestError("investment_account_id must be an investment account")
		}

		if req.FeeCategoryID != nil {
			if err := validateExpenseCategory(tx, ledgerID, *req.FeeCategoryID); err != nil {
				return err
			}
		}
		if req.TaxCategoryID != nil {
			if err := validateExpenseCategory(tx, ledgerID, *req.TaxCategoryID); err != nil {
				return err
			}
		}

		var lots []model.InvestmentLot
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id IN ? AND ledger_id = ?", lotIDs, ledgerID).
			Find(&lots).Error; err != nil {
			return err
		}
		if len(lots) != len(lotIDs) {
			return newRequestError("one or more buy lots not found")
		}

		lotMap := make(map[uint]model.InvestmentLot, len(lots))
		for _, lot := range lots {
			lotMap[lot.ID] = lot
		}

		type allocSum struct {
			BuyLotID     uint    `gorm:"column:buy_lot_id"`
			AllocatedQty float64 `gorm:"column:allocated_qty"`
		}

		var sums []allocSum
		if err := tx.Table("fin_investment_lot_allocations").
			Select("buy_lot_id, COALESCE(SUM(quantity), 0) AS allocated_qty").
			Where("buy_lot_id IN ? AND deleted_at IS NULL", lotIDs).
			Group("buy_lot_id").
			Scan(&sums).Error; err != nil {
			return err
		}

		allocatedMap := make(map[uint]float64, len(sums))
		for _, sum := range sums {
			allocatedMap[sum.BuyLotID] = sum.AllocatedQty
		}

		totalQty := 0.0
		totalCost := 0.0
		for _, lotID := range lotIDs {
			lot := lotMap[lotID]
			if lot.SecurityID != req.SecurityID {
				return newRequestError("selected lots must share the same security_id")
			}
			requestedQty := allocationMap[lotID]
			remaining := lot.Quantity - allocatedMap[lotID]
			if requestedQty > remaining+1e-8 {
				return newRequestError("allocation quantity exceeds remaining lot quantity")
			}
			totalQty += requestedQty
			totalCost += requestedQty * lot.Price
		}

		if totalQty <= 0 {
			return newRequestError("total quantity must be greater than 0")
		}

		grossAmount := totalQty * req.Price

		txRecord := model.Transaction{
			LedgerID:    ledgerID,
			OccurredOn:  occurredOn,
			Description: strings.TrimSpace(req.Description),
			Note:        strings.TrimSpace(req.Note),
		}
		if err := tx.Create(&txRecord).Error; err != nil {
			return err
		}

		cashLine := model.TransactionLine{
			LedgerID:      ledgerID,
			TransactionID: txRecord.ID,
			AccountID:     req.CashAccountID,
			Amount:        grossAmount,
		}
		if err := tx.Create(&cashLine).Error; err != nil {
			return err
		}

		if req.Fee > 0 {
			feeLine := model.TransactionLine{
				LedgerID:      ledgerID,
				TransactionID: txRecord.ID,
				AccountID:     req.CashAccountID,
				CategoryID:    req.FeeCategoryID,
				Amount:        -req.Fee,
			}
			if err := tx.Create(&feeLine).Error; err != nil {
				return err
			}
		}

		if req.Tax > 0 {
			taxLine := model.TransactionLine{
				LedgerID:      ledgerID,
				TransactionID: txRecord.ID,
				AccountID:     req.CashAccountID,
				CategoryID:    req.TaxCategoryID,
				Amount:        -req.Tax,
			}
			if err := tx.Create(&taxLine).Error; err != nil {
				return err
			}
		}

		investmentLine := model.TransactionLine{
			LedgerID:      ledgerID,
			TransactionID: txRecord.ID,
			AccountID:     req.InvestmentAccountID,
			Amount:        -totalCost,
		}
		if err := tx.Create(&investmentLine).Error; err != nil {
			return err
		}

		sale := model.InvestmentSale{
			LedgerID:          ledgerID,
			TransactionLineID: cashLine.ID,
			SecurityID:        req.SecurityID,
			Quantity:          totalQty,
			Price:             req.Price,
		}
		if err := tx.Create(&sale).Error; err != nil {
			return err
		}

		allocations := make([]model.InvestmentLotAllocation, 0, len(allocationMap))
		for _, lotID := range lotIDs {
			allocations = append(allocations, model.InvestmentLotAllocation{
				LedgerID: ledgerID,
				BuyLotID: lotID,
				SaleID:   sale.ID,
				Quantity: allocationMap[lotID],
			})
		}
		if err := tx.Create(&allocations).Error; err != nil {
			return err
		}

		response = createSaleResponse{
			TransactionID: txRecord.ID,
			SaleID:        sale.ID,
			Quantity:      totalQty,
			Price:         req.Price,
			GrossAmount:   grossAmount,
			CostAmount:    totalCost,
			Fee:           req.Fee,
			Tax:           req.Tax,
		}

		return nil
	})

	if err != nil {
		var reqErr requestError
		if errors.As(err, &reqErr) {
			c.JSON(http.StatusBadRequest, gin.H{"error": reqErr.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create sale"})
		return
	}

	c.JSON(http.StatusCreated, response)
}

type refreshPricesResponse struct {
	Requested int      `json:"requested"`
	Updated   int      `json:"updated"`
	Skipped   int      `json:"skipped"`
	Failed    []string `json:"failed,omitempty"`
}

type openSecurityRow struct {
	SecurityID uint   `gorm:"column:security_id"`
	Ticker     string `gorm:"column:ticker"`
}

func (h Handler) refreshPrices(c *gin.Context) {
	ledgerID := 1
	if value := strings.TrimSpace(c.Query("ledger_id")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ledger_id"})
			return
		}
		ledgerID = parsed
	}
	historyDays := 0
	if value := strings.TrimSpace(c.Query("history_days")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid history_days"})
			return
		}
		if parsed > 365 {
			parsed = 365
		}
		historyDays = parsed
	}

	query := `
SELECT l.security_id, s.ticker
FROM fin_investment_lots l
JOIN fin_securities s ON s.id = l.security_id AND s.deleted_at IS NULL
LEFT JOIN fin_investment_lot_allocations a ON a.buy_lot_id = l.id AND a.deleted_at IS NULL
WHERE l.deleted_at IS NULL AND l.ledger_id = ?
GROUP BY l.id, s.ticker
HAVING l.quantity - COALESCE(SUM(a.quantity), 0) > 0`

	var rows []openSecurityRow
	if err := h.db.Raw(query, ledgerID).Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query open lots"})
		return
	}

	securityMap := make(map[uint]string, len(rows))
	for _, row := range rows {
		if row.SecurityID == 0 || strings.TrimSpace(row.Ticker) == "" {
			continue
		}
		securityMap[row.SecurityID] = row.Ticker
	}
	securityIDs := make([]uint, 0, len(securityMap))
	for securityID := range securityMap {
		securityIDs = append(securityIDs, securityID)
	}

	if len(securityMap) == 0 {
		c.JSON(http.StatusOK, refreshPricesResponse{Requested: 0, Updated: 0, Skipped: 0})
		return
	}

	securityToCode := make(map[uint]string, len(securityMap))
	skipped := 0
	for securityID, ticker := range securityMap {
		code, ok := toSinaCode(ticker)
		if !ok {
			skipped++
			continue
		}
		securityToCode[securityID] = code
	}

	if len(securityToCode) == 0 {
		c.JSON(http.StatusOK, refreshPricesResponse{Requested: len(securityMap), Updated: 0, Skipped: skipped})
		return
	}

	codeToSecurity := make(map[string]uint, len(securityToCode))
	codes := make([]string, 0, len(securityToCode))
	for securityID, code := range securityToCode {
		codeToSecurity[code] = securityID
		codes = append(codes, code)
	}

	priceMap := make(map[uint]float64, len(codes))
	failed := make([]string, 0)
	client := &http.Client{Timeout: 8 * time.Second}
	const batchSize = 50
	for start := 0; start < len(codes); start += batchSize {
		end := start + batchSize
		if end > len(codes) {
			end = len(codes)
		}
		batch := codes[start:end]
		prices, err := fetchSinaPrices(client, batch)
		if err != nil {
			failed = append(failed, fmt.Sprintf("batch %d-%d: %v", start, end-1, err))
			continue
		}
		for code, price := range prices {
			if securityID, ok := codeToSecurity[code]; ok && price > 0 {
				priceMap[securityID] = price
			}
		}
	}

	if len(priceMap) == 0 {
		c.JSON(http.StatusOK, refreshPricesResponse{Requested: len(securityMap), Updated: 0, Skipped: skipped, Failed: failed})
		return
	}

	now := time.Now().In(time.Local)
	priceAt := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)

	needHistory := make(map[uint]bool, len(securityIDs))
	if historyDays > 1 {
		lookbackDays := historyDays * 2
		if lookbackDays < historyDays {
			lookbackDays = historyDays
		}
		cutoff := priceAt.AddDate(0, 0, -lookbackDays+1)

		type countRow struct {
			SecurityID uint  `gorm:"column:security_id"`
			Count      int64 `gorm:"column:cnt"`
		}
		var counts []countRow
		if err := h.db.Table("fin_security_prices").
			Select("security_id, COUNT(*) AS cnt").
			Where("ledger_id = ? AND security_id IN ? AND deleted_at IS NULL AND price_at >= ?", ledgerID, securityIDs, cutoff).
			Group("security_id").
			Scan(&counts).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query price history"})
			return
		}

		countMap := make(map[uint]int64, len(counts))
		for _, row := range counts {
			countMap[row.SecurityID] = row.Count
		}
		for _, securityID := range securityIDs {
			if countMap[securityID] < int64(historyDays) {
				needHistory[securityID] = true
			}
		}
	}
	records := make([]model.SecurityPrice, 0, len(priceMap))
	for securityID, price := range priceMap {
		records = append(records, model.SecurityPrice{
			LedgerID:   ledgerID,
			SecurityID: securityID,
			PriceAt:    priceAt,
			ClosePrice: price,
		})
	}

	if historyDays > 1 {
		for securityID, code := range securityToCode {
			if !needHistory[securityID] {
				continue
			}
			history, err := fetchSinaHistory(client, code, historyDays)
			if err != nil {
				failed = append(failed, fmt.Sprintf("%s history: %v", code, err))
				continue
			}
			for _, item := range history {
				records = append(records, model.SecurityPrice{
					LedgerID:   ledgerID,
					SecurityID: securityID,
					PriceAt:    item.Date,
					ClosePrice: item.Close,
				})
			}
		}
	}

	if err := h.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "ledger_id"}, {Name: "security_id"}, {Name: "price_at"}},
		DoUpdates: clause.AssignmentColumns([]string{"close_price"}),
	}).Create(&records).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upsert prices"})
		return
	}

	if err := h.updateIndicators(ledgerID, priceAt, securityIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update indicators"})
		return
	}

	c.JSON(http.StatusOK, refreshPricesResponse{
		Requested: len(securityMap),
		Updated:   len(records),
		Skipped:   skipped,
		Failed:    failed,
	})
}

type priceHistoryRow struct {
	SecurityID uint    `gorm:"column:security_id"`
	ClosePrice float64 `gorm:"column:close_price"`
}

func (h Handler) updateIndicators(ledgerID int, asOf time.Time, securityIDs []uint) error {
	if len(securityIDs) == 0 {
		return nil
	}

	var rows []priceHistoryRow
	if err := h.db.Table("fin_security_prices").
		Select("security_id, close_price").
		Where("ledger_id = ? AND security_id IN ? AND deleted_at IS NULL", ledgerID, securityIDs).
		Order("security_id, price_at DESC").
		Scan(&rows).Error; err != nil {
		return err
	}

	seriesMap := make(map[uint][]float64, len(securityIDs))
	for _, row := range rows {
		seriesMap[row.SecurityID] = append(seriesMap[row.SecurityID], row.ClosePrice)
	}

	indicators := make([]model.SecurityIndicator, 0, len(securityIDs))
	for _, securityID := range securityIDs {
		values := seriesMap[securityID]
		ma5 := movingAverage(values, 5)
		high55, _ := windowStats(values, 55)
		high20, _ := windowStats(values, 20)
		_, low10 := windowStats(values, 10)
		_, low20 := windowStats(values, 20)

		indicators = append(indicators, model.SecurityIndicator{
			LedgerID:   ledgerID,
			SecurityID: securityID,
			AsOf:       asOf,
			MA5:        ma5,
			High55:     high55,
			High20:     high20,
			Low10:      low10,
			Low20:      low20,
		})
	}

	if len(indicators) == 0 {
		return nil
	}

	return h.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "ledger_id"},
			{Name: "security_id"},
			{Name: "as_of"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"ma_5", "high_55", "high_20", "low_10", "low_20", "updated_at"}),
	}).Create(&indicators).Error
}

func windowStats(values []float64, n int) (*float64, *float64) {
	if len(values) < n || n <= 0 {
		return nil, nil
	}
	maxV := values[0]
	minV := values[0]
	for _, v := range values[1:n] {
		if v > maxV {
			maxV = v
		}
		if v < minV {
			minV = v
		}
	}
	return &maxV, &minV
}

func movingAverage(values []float64, n int) *float64 {
	if len(values) < n || n <= 0 {
		return nil
	}
	sum := 0.0
	for _, v := range values[:n] {
		sum += v
	}
	avg := sum / float64(n)
	return &avg
}

type historyItem struct {
	Date  time.Time
	Close float64
}

type sinaHistoryItem struct {
	Day   string `json:"day"`
	Date  string `json:"date"`
	Close string `json:"close"`
}

func fetchSinaHistory(client *http.Client, code string, days int) ([]historyItem, error) {
	if days <= 0 {
		return nil, nil
	}
	urls := []string{
		fmt.Sprintf("https://quotes.sina.cn/cn/api/json_v2.php/CN_MarketData.getKLineData?symbol=%s&scale=240&ma=no&datalen=%d", code, days),
		fmt.Sprintf("https://money.finance.sina.com.cn/quotes_service/api/json_v2.php/CN_MarketData.getKLineData?symbol=%s&scale=240&ma=no&datalen=%d", code, days),
		fmt.Sprintf("http://quotes.sina.cn/cn/api/json_v2.php/CN_MarketData.getKLineData?symbol=%s&scale=240&ma=no&datalen=%d", code, days),
	}
	var lastErr error
	for _, url := range urls {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Referer", "https://finance.sina.com.cn")
		req.Header.Set("User-Agent", "Mozilla/5.0")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("unexpected status: %d", resp.StatusCode)
			continue
		}

		var raw []sinaHistoryItem
		if err := json.Unmarshal(body, &raw); err != nil {
			lastErr = err
			continue
		}
		items := make([]historyItem, 0, len(raw))
		for _, item := range raw {
			dateStr := strings.TrimSpace(item.Day)
			if dateStr == "" {
				dateStr = strings.TrimSpace(item.Date)
			}
			if dateStr == "" {
				continue
			}
			day, err := time.ParseInLocation("2006-01-02", dateStr, time.Local)
			if err != nil {
				continue
			}
			closeStr := strings.TrimSpace(item.Close)
			if closeStr == "" {
				continue
			}
			closePrice, err := strconv.ParseFloat(closeStr, 64)
			if err != nil || closePrice <= 0 {
				continue
			}
			items = append(items, historyItem{Date: day, Close: closePrice})
		}
		return items, nil
	}
	return nil, lastErr
}

type requestError struct {
	message string
}

func (e requestError) Error() string {
	return e.message
}

func newRequestError(message string) error {
	return requestError{message: message}
}

func parseUintID(raw string) (uint, bool) {
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		return 0, false
	}
	return uint(value), true
}

func parsePage(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return 1
	}
	return parsed
}

func parsePageSize(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return 20
	}
	if parsed > 200 {
		return 200
	}
	return parsed
}

func parseDateQuery(value string, c *gin.Context) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date must be YYYY-MM-DD"})
		return time.Time{}, err
	}
	return parsed, nil
}

func validateLotTag(value string, c *gin.Context) bool {
	if value == "" {
		return true
	}
	switch value {
	case "建仓", "定投", "打野":
		return true
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag must be one of: 建仓, 定投, 打野"})
		return false
	}
}

func toSinaCode(ticker string) (string, bool) {
	value := strings.ToLower(strings.TrimSpace(ticker))
	if value == "" {
		return "", false
	}
	if strings.HasPrefix(value, "sh") || strings.HasPrefix(value, "sz") || strings.HasPrefix(value, "bj") {
		if len(value) > 2 {
			return value, true
		}
	}
	if strings.Contains(value, ".") {
		parts := strings.Split(value, ".")
		if len(parts) == 2 {
			code := parts[0]
			exch := strings.ToLower(parts[1])
			switch exch {
			case "sh":
				return "sh" + code, true
			case "sz":
				return "sz" + code, true
			case "bj":
				return "bj" + code, true
			}
		}
	}
	if len(value) != 6 {
		return "", false
	}
	if _, err := strconv.Atoi(value); err != nil {
		return "", false
	}
	switch value[0] {
	case '5', '6', '9':
		return "sh" + value, true
	case '0', '1', '2', '3':
		return "sz" + value, true
	case '4', '8':
		return "bj" + value, true
	default:
		return "", false
	}
}

func fetchSinaPrices(client *http.Client, codes []string) (map[string]float64, error) {
	if len(codes) == 0 {
		return map[string]float64{}, nil
	}
	urls := []string{
		"https://hq.sinajs.cn/list=" + strings.Join(codes, ","),
		"http://hq.sinajs.cn/list=" + strings.Join(codes, ","),
	}
	var lastErr error
	for _, url := range urls {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Referer", "https://finance.sina.com.cn")
		req.Header.Set("User-Agent", "Mozilla/5.0")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("unexpected status: %d", resp.StatusCode)
			continue
		}
		return parseSinaResponse(string(body)), nil
	}
	return nil, lastErr
}

func parseSinaResponse(body string) map[string]float64 {
	result := make(map[string]float64)
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx := strings.Index(line, "hq_str_")
		if idx == -1 {
			continue
		}
		rest := line[idx+len("hq_str_"):]
		eq := strings.Index(rest, "=")
		if eq == -1 {
			continue
		}
		code := strings.TrimSpace(rest[:eq])
		firstQuote := strings.Index(line, "\"")
		lastQuote := strings.LastIndex(line, "\"")
		if firstQuote == -1 || lastQuote <= firstQuote {
			continue
		}
		payload := line[firstQuote+1 : lastQuote]
		if payload == "" {
			continue
		}
		fields := strings.Split(payload, ",")
		if len(fields) < 4 {
			continue
		}
		price, err := strconv.ParseFloat(strings.TrimSpace(fields[3]), 64)
		if err != nil {
			continue
		}
		result[code] = price
	}
	return result
}

func resolveSecurity(tx *gorm.DB, ledgerID int, securityID *uint, tickerRaw string, nameRaw string) (model.Security, error) {
	if securityID != nil && *securityID > 0 {
		var security model.Security
		if err := tx.Where("id = ? AND ledger_id = ?", *securityID, ledgerID).First(&security).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return model.Security{}, newRequestError("security not found")
			}
			return model.Security{}, err
		}
		return security, nil
	}

	ticker := strings.ToUpper(strings.TrimSpace(tickerRaw))
	name := strings.TrimSpace(nameRaw)
	if ticker == "" || name == "" {
		return model.Security{}, newRequestError("security_ticker and security_name are required")
	}

	var security model.Security
	err := tx.Where("ticker = ? AND ledger_id = ?", ticker, ledgerID).First(&security).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		security = model.Security{
			LedgerID: ledgerID,
			Ticker:   ticker,
			Name:     name,
		}
		if err := tx.Create(&security).Error; err != nil {
			return model.Security{}, err
		}
		return security, nil
	}
	if err != nil {
		return model.Security{}, err
	}

	if security.LedgerID != ledgerID {
		return model.Security{}, newRequestError("security exists in another ledger")
	}
	if name != "" && security.Name != name {
		security.Name = name
		if err := tx.Save(&security).Error; err != nil {
			return model.Security{}, err
		}
	}
	return security, nil
}

func validateExpenseCategory(tx *gorm.DB, ledgerID int, categoryID int) error {
	var category model.Category
	if err := tx.Where("id = ? AND ledger_id = ?", categoryID, ledgerID).First(&category).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return newRequestError("category not found")
		}
		return err
	}
	if category.Kind != model.CategoryKindExpense {
		return newRequestError("category must be expense kind")
	}
	return nil
}
