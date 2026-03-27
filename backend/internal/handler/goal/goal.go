package goal

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
	rg.PUT("/annual-asset/:year", h.upsertAnnualAssetGoal)
	rg.GET("/annual-asset/:year", h.getAnnualAssetGoal)
}

type upsertAnnualAssetGoalRequest struct {
	LedgerID       *int    `json:"ledger_id"`
	TargetNetWorth float64 `json:"target_net_worth" binding:"required"`
	BaselineDate   string  `json:"baseline_date"`
	Note           string  `json:"note"`
}

type annualAssetGoalResponse struct {
	ID             uint    `json:"id"`
	LedgerID       int     `json:"ledger_id"`
	Year           int     `json:"year"`
	TargetNetWorth float64 `json:"target_net_worth"`
	BaselineDate   string  `json:"baseline_date"`
	Note           string  `json:"note"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

func (h Handler) upsertAnnualAssetGoal(c *gin.Context) {
	year, ok := parseYearParam(c.Param("year"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "year must be a valid YYYY"})
		return
	}

	var req upsertAnnualAssetGoalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ledgerID, ok := parseLedgerID(req.LedgerID)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ledger_id must be positive"})
		return
	}
	if req.TargetNetWorth <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target_net_worth must be greater than 0"})
		return
	}

	baselineDate, ok := parseBaselineDate(req.BaselineDate, year)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "baseline_date must be YYYY-MM-DD and within goal year or next year"})
		return
	}

	note := strings.TrimSpace(req.Note)

	var goal model.AnnualAssetGoal
	err := h.db.Where("ledger_id = ? AND year = ?", ledgerID, year).First(&goal).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load annual goal"})
		return
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		goal = model.AnnualAssetGoal{
			LedgerID:       ledgerID,
			Year:           year,
			TargetNetWorth: req.TargetNetWorth,
			BaselineDate:   baselineDate,
			Note:           note,
		}
		if err := h.db.Create(&goal).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create annual goal"})
			return
		}
	} else {
		goal.TargetNetWorth = req.TargetNetWorth
		goal.BaselineDate = baselineDate
		goal.Note = note
		if err := h.db.Save(&goal).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update annual goal"})
			return
		}
	}

	c.JSON(http.StatusOK, toAnnualAssetGoalResponse(goal))
}

func (h Handler) getAnnualAssetGoal(c *gin.Context) {
	year, ok := parseYearParam(c.Param("year"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "year must be a valid YYYY"})
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

	var goal model.AnnualAssetGoal
	err := h.db.Where("ledger_id = ? AND year = ?", ledgerID, year).First(&goal).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "annual goal not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load annual goal"})
		return
	}

	c.JSON(http.StatusOK, toAnnualAssetGoalResponse(goal))
}

func parseLedgerID(value *int) (int, bool) {
	ledgerID := 1
	if value != nil {
		if *value <= 0 {
			return 0, false
		}
		ledgerID = *value
	}
	return ledgerID, true
}

func parseYearParam(raw string) (int, bool) {
	year, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || year < 1900 || year > 9999 {
		return 0, false
	}
	return year, true
}

func parseBaselineDate(raw string, year int) (time.Time, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Date(year, time.January, 1, 0, 0, 0, 0, time.Local), true
	}
	// 兼容前端可能传入 RFC3339（如 2026-01-01T00:00:00.000Z），优先截取日期部分。
	if len(value) >= len("2006-01-02") {
		candidate := value[:10]
		if _, err := time.Parse("2006-01-02", candidate); err == nil {
			value = candidate
		}
	}
	date, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	if date.Year() != year && date.Year() != year+1 {
		return time.Time{}, false
	}
	return date, true
}

func toAnnualAssetGoalResponse(goal model.AnnualAssetGoal) annualAssetGoalResponse {
	return annualAssetGoalResponse{
		ID:             goal.ID,
		LedgerID:       goal.LedgerID,
		Year:           goal.Year,
		TargetNetWorth: goal.TargetNetWorth,
		BaselineDate:   goal.BaselineDate.Format("2006-01-02"),
		Note:           goal.Note,
		CreatedAt:      goal.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      goal.UpdatedAt.Format(time.RFC3339),
	}
}
