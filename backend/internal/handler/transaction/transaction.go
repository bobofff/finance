package transaction

import (
	"errors"
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

	rg.POST("", h.create)
	rg.GET("", h.list)
	rg.GET("/:id", h.get)
	rg.PATCH("/:id", h.update)
	rg.DELETE("/:id", h.delete)
}

type createTransactionRequest struct {
	LedgerID    *int    `json:"ledger_id"`
	OccurredOn  string  `json:"occurred_on" binding:"required"`
	AccountID   uint    `json:"account_id" binding:"required,gt=0"`
	CategoryID  int     `json:"category_id" binding:"required,gt=0"`
	Amount      float64 `json:"amount" binding:"required"`
	Description string  `json:"description"`
	Note        string  `json:"note"`
}

type updateTransactionRequest struct {
	OccurredOn  *string  `json:"occurred_on"`
	AccountID   *uint    `json:"account_id"`
	CategoryID  *int     `json:"category_id"`
	Amount      *float64 `json:"amount"`
	Description *string  `json:"description"`
	Note        *string  `json:"note"`
}

type transactionRow struct {
	TransactionID uint      `json:"transaction_id"`
	LineID        uint      `json:"line_id"`
	OccurredOn    time.Time `json:"occurred_on"`
	AccountID     uint      `json:"account_id"`
	AccountName   string    `json:"account_name"`
	CategoryID    int       `json:"category_id"`
	CategoryName  string    `json:"category_name"`
	CategoryKind  string    `json:"category_kind"`
	Amount        float64   `json:"amount"`
	Description   string    `json:"description"`
	Note          string    `json:"note"`
	CreatedAt     time.Time `json:"created_at"`
}

type listResponse struct {
	Data  []transactionRowResponse `json:"data"`
	Total int64                    `json:"total"`
}

type transactionRowResponse struct {
	TransactionID uint    `json:"transaction_id"`
	LineID        uint    `json:"line_id"`
	OccurredOn    string  `json:"occurred_on"`
	AccountID     uint    `json:"account_id"`
	AccountName   string  `json:"account_name"`
	CategoryID    int     `json:"category_id"`
	CategoryName  string  `json:"category_name"`
	CategoryKind  string  `json:"category_kind"`
	Amount        float64 `json:"amount"`
	Description   string  `json:"description"`
	Note          string  `json:"note"`
	CreatedAt     string  `json:"created_at"`
}

func (h Handler) create(c *gin.Context) {
	var req createTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ledgerID := normalizeLedgerID(req.LedgerID, c)
	if ledgerID == 0 {
		return
	}

	occurredOn, ok := parseDate(req.OccurredOn, c)
	if !ok {
		return
	}

	account, ok := validateAccount(h.db, ledgerID, req.AccountID, c)
	if !ok {
		return
	}
	if !account.IsActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "account is inactive"})
		return
	}

	category, ok := validateCategory(h.db, ledgerID, req.CategoryID, c)
	if !ok {
		return
	}

	if !validateAmount(category.Kind, req.Amount, c) {
		return
	}

	var response transactionRowResponse

	err := h.db.Transaction(func(tx *gorm.DB) error {
		txRecord := model.Transaction{
			LedgerID:    ledgerID,
			OccurredOn:  occurredOn,
			Description: strings.TrimSpace(req.Description),
			Note:        strings.TrimSpace(req.Note),
		}
		if err := tx.Create(&txRecord).Error; err != nil {
			return err
		}

		line := model.TransactionLine{
			LedgerID:      ledgerID,
			TransactionID: txRecord.ID,
			AccountID:     req.AccountID,
			CategoryID:    &req.CategoryID,
			Amount:        req.Amount,
		}
		if err := tx.Create(&line).Error; err != nil {
			return err
		}

		response = transactionRowResponse{
			TransactionID: txRecord.ID,
			LineID:        line.ID,
			OccurredOn:    txRecord.OccurredOn.Format("2006-01-02"),
			AccountID:     account.ID,
			AccountName:   account.Name,
			CategoryID:    category.ID,
			CategoryName:  category.Name,
			CategoryKind:  string(category.Kind),
			Amount:        line.Amount,
			Description:   txRecord.Description,
			Note:          txRecord.Note,
			CreatedAt:     txRecord.CreatedAt.Format(time.RFC3339),
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create transaction"})
		return
	}

	c.JSON(http.StatusCreated, response)
}

func (h Handler) list(c *gin.Context) {
	ledgerID := 1
	if value := strings.TrimSpace(c.Query("ledger_id")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ledger_id"})
			return
		}
		ledgerID = parsed
	}

	var (
		accountID  uint
		categoryID int
	)

	if value := strings.TrimSpace(c.Query("account_id")); value != "" {
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil || parsed == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account_id"})
			return
		}
		accountID = uint(parsed)
	}

	if value := strings.TrimSpace(c.Query("category_id")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category_id"})
			return
		}
		categoryID = parsed
	}

	kind := strings.TrimSpace(strings.ToLower(c.Query("kind")))
	if kind != "" && kind != "income" && kind != "expense" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kind must be income or expense"})
		return
	}

	dateFrom, ok := parseDateQuery(c.Query("date_from"), c)
	if !ok {
		return
	}
	dateTo, ok := parseDateQuery(c.Query("date_to"), c)
	if !ok {
		return
	}

	page := parsePage(c.Query("page"))
	pageSize := parsePageSize(c.Query("page_size"))
	offset := (page - 1) * pageSize

	base := h.db.Table("fin_transaction_lines tl").
		Joins("JOIN fin_transactions t ON t.id = tl.transaction_id AND t.deleted_at IS NULL").
		Joins("JOIN fin_accounts a ON a.id = tl.account_id AND a.deleted_at IS NULL").
		Joins("JOIN fin_categories c ON c.id = tl.category_id AND c.deleted_at IS NULL").
		Where("tl.ledger_id = ? AND tl.deleted_at IS NULL", ledgerID).
		Where("c.kind IN ('income','expense')")

	if accountID != 0 {
		base = base.Where("tl.account_id = ?", accountID)
	}
	if categoryID != 0 {
		base = base.Where("tl.category_id = ?", categoryID)
	}
	if kind != "" {
		base = base.Where("c.kind = ?", kind)
	}
	if !dateFrom.IsZero() {
		base = base.Where("t.occurred_on >= ?", dateFrom)
	}
	if !dateTo.IsZero() {
		base = base.Where("t.occurred_on <= ?", dateTo)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count transactions"})
		return
	}

	var rows []transactionRow
	if err := base.Select(`
    t.id AS transaction_id,
    tl.id AS line_id,
    t.occurred_on,
    a.id AS account_id,
    a.name AS account_name,
    c.id AS category_id,
    c.name AS category_name,
    c.kind AS category_kind,
    tl.amount,
    t.description,
    t.note,
    t.created_at
  `).
		Order("t.occurred_on desc, t.id desc").
		Limit(pageSize).
		Offset(offset).
		Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query transactions"})
		return
	}

	resp := make([]transactionRowResponse, 0, len(rows))
	for _, row := range rows {
		resp = append(resp, transactionRowResponse{
			TransactionID: row.TransactionID,
			LineID:        row.LineID,
			OccurredOn:    row.OccurredOn.Format("2006-01-02"),
			AccountID:     row.AccountID,
			AccountName:   row.AccountName,
			CategoryID:    row.CategoryID,
			CategoryName:  row.CategoryName,
			CategoryKind:  row.CategoryKind,
			Amount:        row.Amount,
			Description:   row.Description,
			Note:          row.Note,
			CreatedAt:     row.CreatedAt.Format(time.RFC3339),
		})
	}

	c.JSON(http.StatusOK, listResponse{Data: resp, Total: total})
}

func (h Handler) get(c *gin.Context) {
	id, ok := parseID(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var row transactionRow
	err := h.db.Table("fin_transaction_lines tl").
		Joins("JOIN fin_transactions t ON t.id = tl.transaction_id AND t.deleted_at IS NULL").
		Joins("JOIN fin_accounts a ON a.id = tl.account_id AND a.deleted_at IS NULL").
		Joins("JOIN fin_categories c ON c.id = tl.category_id AND c.deleted_at IS NULL").
		Where("t.id = ? AND tl.deleted_at IS NULL", id).
		Select(`
      t.id AS transaction_id,
      tl.id AS line_id,
      t.occurred_on,
      a.id AS account_id,
      a.name AS account_name,
      c.id AS category_id,
      c.name AS category_name,
      c.kind AS category_kind,
      tl.amount,
      t.description,
      t.note,
      t.created_at
    `).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query transaction"})
		return
	}

	resp := transactionRowResponse{
		TransactionID: row.TransactionID,
		LineID:        row.LineID,
		OccurredOn:    row.OccurredOn.Format("2006-01-02"),
		AccountID:     row.AccountID,
		AccountName:   row.AccountName,
		CategoryID:    row.CategoryID,
		CategoryName:  row.CategoryName,
		CategoryKind:  row.CategoryKind,
		Amount:        row.Amount,
		Description:   row.Description,
		Note:          row.Note,
		CreatedAt:     row.CreatedAt.Format(time.RFC3339),
	}
	c.JSON(http.StatusOK, resp)
}

func (h Handler) update(c *gin.Context) {
	id, ok := parseID(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req updateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var txRecord model.Transaction
	if err := h.db.First(&txRecord, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load transaction"})
		return
	}

	var line model.TransactionLine
	if err := h.db.Where("transaction_id = ? AND ledger_id = ?", txRecord.ID, txRecord.LedgerID).
		First(&line).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load transaction line"})
		return
	}

	ledgerID := txRecord.LedgerID
	if req.OccurredOn != nil {
		parsed, ok := parseDate(*req.OccurredOn, c)
		if !ok {
			return
		}
		txRecord.OccurredOn = parsed
	}

	if req.Description != nil {
		txRecord.Description = strings.TrimSpace(*req.Description)
	}
	if req.Note != nil {
		txRecord.Note = strings.TrimSpace(*req.Note)
	}

	if req.AccountID != nil {
		account, ok := validateAccount(h.db, ledgerID, *req.AccountID, c)
		if !ok {
			return
		}
		if !account.IsActive {
			c.JSON(http.StatusBadRequest, gin.H{"error": "account is inactive"})
			return
		}
		line.AccountID = *req.AccountID
	}

	var category model.Category
	if req.CategoryID != nil {
		var ok bool
		category, ok = validateCategory(h.db, ledgerID, *req.CategoryID, c)
		if !ok {
			return
		}
		line.CategoryID = req.CategoryID
	} else {
		if line.CategoryID != nil {
			if err := h.db.First(&category, *line.CategoryID).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load category"})
				return
			}
		}
	}

	if req.Amount != nil {
		if !validateAmount(category.Kind, *req.Amount, c) {
			return
		}
		line.Amount = *req.Amount
	} else if req.CategoryID != nil {
		if !validateAmount(category.Kind, line.Amount, c) {
			return
		}
	}

	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&txRecord).Error; err != nil {
			return err
		}
		if err := tx.Save(&line).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h Handler) delete(c *gin.Context) {
	id, ok := parseID(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	err := h.db.Transaction(func(tx *gorm.DB) error {
		var lines []model.TransactionLine
		if err := tx.Where("transaction_id = ?", id).Find(&lines).Error; err != nil {
			return err
		}
		if len(lines) == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Delete(&model.TransactionLine{}, "transaction_id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Delete(&model.Transaction{}, id).Error; err != nil {
			return err
		}
		return nil
	})

	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete transaction"})
		return
	}

	c.Status(http.StatusNoContent)
}

func normalizeLedgerID(value *int, c *gin.Context) int {
	ledgerID := 1
	if value != nil {
		if *value <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ledger_id must be positive"})
			return 0
		}
		ledgerID = *value
	}
	return ledgerID
}

func parseDate(value string, c *gin.Context) (time.Time, bool) {
	parsed, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(value), time.Local)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "occurred_on must be YYYY-MM-DD"})
		return time.Time{}, false
	}
	return parsed, true
}

func parseDateQuery(value string, c *gin.Context) (time.Time, bool) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, true
	}
	parsed, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(value), time.Local)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date must be YYYY-MM-DD"})
		return time.Time{}, false
	}
	return parsed, true
}

func validateAccount(db *gorm.DB, ledgerID int, accountID uint, c *gin.Context) (model.Account, bool) {
	var account model.Account
	if err := db.Where("id = ? AND ledger_id = ?", accountID, ledgerID).First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "account not found"})
			return model.Account{}, false
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load account"})
		return model.Account{}, false
	}
	return account, true
}

func validateCategory(db *gorm.DB, ledgerID int, categoryID int, c *gin.Context) (model.Category, bool) {
	var category model.Category
	if err := db.Where("id = ? AND ledger_id = ?", categoryID, ledgerID).First(&category).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "category not found"})
			return model.Category{}, false
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load category"})
		return model.Category{}, false
	}
	if category.Kind != model.CategoryKindIncome && category.Kind != model.CategoryKindExpense {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category must be income or expense"})
		return model.Category{}, false
	}
	return category, true
}

func validateAmount(kind model.CategoryKind, amount float64, c *gin.Context) bool {
	if amount == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount cannot be 0"})
		return false
	}
	if kind == model.CategoryKindIncome && amount < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "income amount must be positive"})
		return false
	}
	if kind == model.CategoryKindExpense && amount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expense amount must be negative"})
		return false
	}
	return true
}

func parseID(raw string) (uint, bool) {
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
