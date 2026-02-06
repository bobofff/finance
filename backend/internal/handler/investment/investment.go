package investment

import (
	"errors"
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
	rg.POST("/sales", h.createSale)
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
	Fee               float64   `gorm:"column:fee"`
	Tax               float64   `gorm:"column:tax"`
	TransactionLineID uint      `gorm:"column:transaction_line_id"`
	TransactionID     uint      `gorm:"column:transaction_id"`
	OccurredOn        time.Time `gorm:"column:occurred_on"`
	AllocatedQuantity float64   `gorm:"column:allocated_quantity"`
	RemainingQuantity float64   `gorm:"column:remaining_quantity"`
}

type lotResponse struct {
	LotID             uint    `json:"lot_id"`
	LedgerID          int     `json:"ledger_id"`
	SecurityID        uint    `json:"security_id"`
	SecurityTicker    string  `json:"security_ticker"`
	SecurityName      string  `json:"security_name"`
	Quantity          float64 `json:"quantity"`
	Price             float64 `json:"price"`
	TradePrice        float64 `json:"trade_price"`
	Fee               float64 `json:"fee"`
	Tax               float64 `json:"tax"`
	TransactionLineID uint    `json:"transaction_line_id"`
	TransactionID     uint    `json:"transaction_id"`
	OccurredOn        string  `json:"occurred_on"`
	AllocatedQuantity float64 `json:"allocated_quantity"`
	RemainingQuantity float64 `json:"remaining_quantity"`
	Status            string  `json:"status"`
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

	status := strings.ToLower(strings.TrimSpace(c.Query("status")))
	if status != "" && status != "open" && status != "closed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status must be open or closed"})
		return
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
  l.fee,
  l.tax,
  tl.id AS transaction_line_id,
  t.id AS transaction_id,
  t.occurred_on,
  COALESCE(SUM(a.quantity), 0) AS allocated_quantity,
  (l.quantity - COALESCE(SUM(a.quantity), 0)) AS remaining_quantity
FROM fin_investment_lots l
JOIN fin_transaction_lines tl ON tl.id = l.transaction_line_id AND tl.deleted_at IS NULL
JOIN fin_transactions t ON t.id = tl.transaction_id AND t.deleted_at IS NULL
JOIN fin_securities s ON s.id = l.security_id AND s.deleted_at IS NULL
LEFT JOIN fin_investment_lot_allocations a ON a.buy_lot_id = l.id AND a.deleted_at IS NULL
WHERE l.deleted_at IS NULL AND l.ledger_id = ?`

	args := []interface{}{ledgerID}
	if securityID != 0 {
		query += " AND l.security_id = ?"
		args = append(args, securityID)
	}

	query += " GROUP BY l.id, s.id, tl.id, t.id"

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
		if status != "" && status != state {
			continue
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
			Fee:               row.Fee,
			Tax:               row.Tax,
			TransactionLineID: row.TransactionLineID,
			TransactionID:     row.TransactionID,
			OccurredOn:        row.OccurredOn.Format("2006-01-02"),
			AllocatedQuantity: row.AllocatedQuantity,
			RemainingQuantity: row.RemainingQuantity,
			Status:            state,
		})
	}

	c.JSON(http.StatusOK, resp)
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
	err := tx.Where("ticker = ?", ticker).First(&security).Error
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
