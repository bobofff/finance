package transfer

import (
	"errors"
	"net/http"
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
}

type createTransferRequest struct {
	LedgerID      *int    `json:"ledger_id"`
	OccurredOn    string  `json:"occurred_on" binding:"required"`
	FromAccountID uint    `json:"from_account_id" binding:"required,gt=0"`
	ToAccountID   uint    `json:"to_account_id" binding:"required,gt=0"`
	Amount        float64 `json:"amount" binding:"required,gt=0"`
	Description   string  `json:"description"`
	Note          string  `json:"note"`
}

type createTransferResponse struct {
	TransactionID uint `json:"transaction_id"`
}

func (h Handler) create(c *gin.Context) {
	var req createTransferRequest
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

	if req.FromAccountID == req.ToAccountID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from_account_id and to_account_id must be different"})
		return
	}

	occurredOnRaw := strings.TrimSpace(req.OccurredOn)
	occurredOn, err := time.ParseInLocation("2006-01-02", occurredOnRaw, time.Local)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "occurred_on must be YYYY-MM-DD"})
		return
	}

	if req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be greater than 0"})
		return
	}

	var response createTransferResponse

	err = h.db.Transaction(func(tx *gorm.DB) error {
		var fromAccount model.Account
		if err := tx.Where("id = ? AND ledger_id = ?", req.FromAccountID, ledgerID).First(&fromAccount).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return newRequestError("from account not found")
			}
			return err
		}
		if !fromAccount.IsActive {
			return newRequestError("from account is inactive")
		}
		if strings.ToLower(fromAccount.Type) != "cash" {
			return newRequestError("from account must be cash type")
		}

		var toAccount model.Account
		if err := tx.Where("id = ? AND ledger_id = ?", req.ToAccountID, ledgerID).First(&toAccount).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return newRequestError("to account not found")
			}
			return err
		}
		if !toAccount.IsActive {
			return newRequestError("to account is inactive")
		}
		if strings.ToLower(toAccount.Type) != "cash" {
			return newRequestError("to account must be cash type")
		}

		txRecord := model.Transaction{
			LedgerID:    ledgerID,
			OccurredOn:  occurredOn,
			Description: strings.TrimSpace(req.Description),
			Note:        strings.TrimSpace(req.Note),
		}
		if err := tx.Create(&txRecord).Error; err != nil {
			return err
		}

		fromLine := model.TransactionLine{
			LedgerID:      ledgerID,
			TransactionID: txRecord.ID,
			AccountID:     req.FromAccountID,
			Amount:        -req.Amount,
		}
		if err := tx.Create(&fromLine).Error; err != nil {
			return err
		}

		toLine := model.TransactionLine{
			LedgerID:      ledgerID,
			TransactionID: txRecord.ID,
			AccountID:     req.ToAccountID,
			Amount:        req.Amount,
		}
		if err := tx.Create(&toLine).Error; err != nil {
			return err
		}

		response = createTransferResponse{TransactionID: txRecord.ID}
		return nil
	})

	if err != nil {
		var reqErr requestError
		if errors.As(err, &reqErr) {
			c.JSON(http.StatusBadRequest, gin.H{"error": reqErr.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create transfer"})
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
