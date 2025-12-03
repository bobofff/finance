package account

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

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

var allowedTypes = map[string]struct{}{
	"cash":        {},
	"liability":   {},
	"debt":        {},
	"investment":  {},
	"other_asset": {},
}

type createAccountRequest struct {
	Name     string `json:"name" binding:"required"`
	Type     string `json:"type" binding:"required"`
	Currency string `json:"currency"`
	IsActive *bool  `json:"is_active"`
}

type updateAccountRequest struct {
	Name     *string `json:"name"`
	Type     *string `json:"type"`
	Currency *string `json:"currency"`
	IsActive *bool   `json:"is_active"`
}

func (h Handler) create(c *gin.Context) {
	var req createAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	accountType, ok := normalizeType(req.Type)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type must be one of: cash, liability, debt, investment, other_asset"})
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	currency := strings.TrimSpace(req.Currency)
	if currency == "" {
		currency = "CNY"
	}

	account := model.Account{
		Name:     name,
		Type:     accountType,
		Currency: currency,
		IsActive: isActive,
	}

	if err := h.db.Create(&account).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create account"})
		return
	}

	c.JSON(http.StatusCreated, account)
}

func (h Handler) list(c *gin.Context) {
	var accounts []model.Account
	if err := h.db.Order("id").Find(&accounts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query accounts"})
		return
	}

	c.JSON(http.StatusOK, accounts)
}

func (h Handler) get(c *gin.Context) {
	id, ok := parseID(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var account model.Account
	err := h.db.First(&account, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query account"})
		return
	}

	c.JSON(http.StatusOK, account)
}

func (h Handler) update(c *gin.Context) {
	id, ok := parseID(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req updateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name cannot be empty"})
			return
		}
		updates["name"] = name
	}

	if req.Type != nil {
		accountType, ok := normalizeType(*req.Type)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "type must be one of: cash, liability, debt, investment, other_asset"})
			return
		}
		updates["type"] = accountType
	}

	if req.Currency != nil {
		updates["currency"] = strings.TrimSpace(*req.Currency)
	}

	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
		return
	}

	tx := h.db.Model(&model.Account{}).Where("id = ?", id).Updates(updates)
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update account"})
		return
	}
	if tx.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}

	var account model.Account
	if err := h.db.First(&account, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load account"})
		return
	}

	c.JSON(http.StatusOK, account)
}

func (h Handler) delete(c *gin.Context) {
	id, ok := parseID(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	tx := h.db.Delete(&model.Account{}, id)
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete account"})
		return
	}
	if tx.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

func parseID(raw string) (uint, bool) {
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, false
	}
	return uint(id), true
}

func normalizeType(input string) (string, bool) {
	value := strings.ToLower(strings.TrimSpace(input))
	_, ok := allowedTypes[value]
	return value, ok
}
