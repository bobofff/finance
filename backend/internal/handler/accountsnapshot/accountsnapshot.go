package accountsnapshot

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"finance-backend/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
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

type createSnapshotRequest struct {
	LedgerID  *int    `json:"ledger_id"`
	AccountID uint    `json:"account_id" binding:"required,gt=0"`
	AsOf      string  `json:"as_of" binding:"required"`
	Amount    float64 `json:"amount"`
	Note      string  `json:"note"`
}

type updateSnapshotRequest struct {
	AsOf   *string  `json:"as_of"`
	Amount *float64 `json:"amount"`
	Note   *string  `json:"note"`
}

func (h Handler) create(c *gin.Context) {
	var req createSnapshotRequest
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

	asOfRaw := strings.TrimSpace(req.AsOf)
	asOf, err := time.ParseInLocation("2006-01-02", asOfRaw, time.Local)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "as_of must be YYYY-MM-DD"})
		return
	}

	if err := validateAccount(h.db, ledgerID, req.AccountID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	snapshot := model.AccountSnapshot{
		LedgerID:  ledgerID,
		AccountID: req.AccountID,
		AsOf:      asOf,
		Amount:    req.Amount,
		Note:      strings.TrimSpace(req.Note),
	}

	if err := h.db.Create(&snapshot).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create snapshot"})
		return
	}

	c.JSON(http.StatusCreated, snapshot)
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

	var accountID uint
	if value := strings.TrimSpace(c.Query("account_id")); value != "" {
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil || parsed == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account_id"})
			return
		}
		accountID = uint(parsed)
	}

	query := h.db.Where("ledger_id = ?", ledgerID)
	if accountID != 0 {
		query = query.Where("account_id = ?", accountID)
	}

	var snapshots []model.AccountSnapshot
	if err := query.Order("as_of desc, id desc").Find(&snapshots).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query snapshots"})
		return
	}

	c.JSON(http.StatusOK, snapshots)
}

func (h Handler) get(c *gin.Context) {
	id, ok := parseID(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var snapshot model.AccountSnapshot
	err := h.db.First(&snapshot, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "snapshot not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query snapshot"})
		return
	}

	c.JSON(http.StatusOK, snapshot)
}

func (h Handler) update(c *gin.Context) {
	id, ok := parseID(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req updateSnapshotRequest
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var raw map[string]json.RawMessage
	if err := c.ShouldBindBodyWith(&raw, binding.JSON); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(raw) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
		return
	}

	var snapshot model.AccountSnapshot
	err := h.db.First(&snapshot, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "snapshot not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load snapshot"})
		return
	}

	if _, ok := raw["as_of"]; ok {
		if req.AsOf == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "as_of cannot be null"})
			return
		}
		asOf, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(*req.AsOf), time.Local)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "as_of must be YYYY-MM-DD"})
			return
		}
		snapshot.AsOf = asOf
	}

	if _, ok := raw["amount"]; ok {
		if req.Amount == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "amount cannot be null"})
			return
		}
		snapshot.Amount = *req.Amount
	}

	if _, ok := raw["note"]; ok {
		if req.Note == nil {
			snapshot.Note = ""
		} else {
			snapshot.Note = strings.TrimSpace(*req.Note)
		}
	}

	if err := h.db.Save(&snapshot).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update snapshot"})
		return
	}

	c.JSON(http.StatusOK, snapshot)
}

func (h Handler) delete(c *gin.Context) {
	id, ok := parseID(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	tx := h.db.Delete(&model.AccountSnapshot{}, id)
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete snapshot"})
		return
	}
	if tx.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "snapshot not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

func parseID(raw string) (uint, bool) {
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		return 0, false
	}
	return uint(value), true
}

func validateAccount(db *gorm.DB, ledgerID int, accountID uint) error {
	var account model.Account
	if err := db.Where("id = ? AND ledger_id = ?", accountID, ledgerID).First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("account not found")
		}
		return err
	}
	return nil
}
