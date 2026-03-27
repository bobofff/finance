package investment

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"finance-backend/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"gorm.io/gorm"
)

type createStrategyRequest struct {
	LedgerID *int            `json:"ledger_id" binding:"omitempty,gt=0"`
	Name     string          `json:"name" binding:"required"`
	Kind     string          `json:"kind" binding:"required"`
	Params   json.RawMessage `json:"params"`
	IsActive *bool           `json:"is_active"`
}

type updateStrategyRequest struct {
	LedgerID *int            `json:"ledger_id"`
	Name     *string         `json:"name"`
	Kind     *string         `json:"kind"`
	Params   json.RawMessage `json:"params"`
	IsActive *bool           `json:"is_active"`
}

func (h Handler) listStrategies(c *gin.Context) {
	ledgerID := 1
	if value := strings.TrimSpace(c.Query("ledger_id")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ledger_id"})
			return
		}
		ledgerID = parsed
	}

	var strategies []model.StrategyTemplate
	if err := h.db.
		Model(&model.StrategyTemplate{}).
		Where("ledger_id = ?", ledgerID).
		Order("id").
		Find(&strategies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query strategies"})
		return
	}

	c.JSON(http.StatusOK, strategies)
}

func (h Handler) createStrategy(c *gin.Context) {
	var req createStrategyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	name := strings.TrimSpace(req.Name)
	kind := strings.TrimSpace(req.Kind)
	if name == "" || kind == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and kind are required"})
		return
	}

	params, err := normalizeStrategyParams(req.Params)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	strategy := model.StrategyTemplate{
		Name:   name,
		Kind:   kind,
		Params: params,
	}
	if req.LedgerID == nil {
		strategy.LedgerID = 1
	} else {
		strategy.LedgerID = *req.LedgerID
	}
	if req.IsActive != nil {
		strategy.IsActive = *req.IsActive
	}

	if err := h.db.Create(&strategy).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create strategy"})
		return
	}

	c.JSON(http.StatusCreated, strategy)
}

func (h Handler) updateStrategy(c *gin.Context) {
	id, ok := parseStrategyID(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req updateStrategyRequest
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

	var strategy model.StrategyTemplate
	err := h.db.First(&strategy, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "strategy not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load strategy"})
		return
	}

	if _, ok := raw["ledger_id"]; ok {
		if req.LedgerID == nil || *req.LedgerID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ledger_id"})
			return
		}
		strategy.LedgerID = *req.LedgerID
	}
	if _, ok := raw["name"]; ok {
		if req.Name == nil || strings.TrimSpace(*req.Name) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid name"})
			return
		}
		strategy.Name = strings.TrimSpace(*req.Name)
	}
	if _, ok := raw["kind"]; ok {
		if req.Kind == nil || strings.TrimSpace(*req.Kind) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid kind"})
			return
		}
		strategy.Kind = strings.TrimSpace(*req.Kind)
	}
	if _, ok := raw["params"]; ok {
		params, err := normalizeStrategyParams(req.Params)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		strategy.Params = params
	}
	if _, ok := raw["is_active"]; ok {
		if req.IsActive == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid is_active"})
			return
		}
		strategy.IsActive = *req.IsActive
	}

	if err := h.db.Save(&strategy).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update strategy"})
		return
	}

	c.JSON(http.StatusOK, strategy)
}

func (h Handler) deleteStrategy(c *gin.Context) {
	id, ok := parseStrategyID(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var strategy model.StrategyTemplate
	err := h.db.First(&strategy, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "strategy not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load strategy"})
		return
	}

	if err := h.db.Delete(&strategy).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete strategy"})
		return
	}

	c.Status(http.StatusNoContent)
}

func normalizeStrategyParams(raw json.RawMessage) (string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "{}", nil
	}
	if !json.Valid(trimmed) {
		return "", errors.New("params must be valid json")
	}
	return string(trimmed), nil
}

func parseStrategyID(raw string) (uint, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed == 0 {
		return 0, false
	}
	return uint(parsed), true
}
