package categories

import (
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

type Handler struct {
	db *gorm.DB
}

type createCategoriesRequest struct {
	LedgerID *int               `json:"ledger_id" binding:"omitempty,gt=0"`
	Name     string             `json:"name" binding:"required"`
	Kind     model.CategoryKind `json:"kind" binding:"required"`
	ParentID *int               `json:"parent_id"`
}

type updateCategoriesRequest struct {
	LedgerID *int                `json:"ledger_id"`
	Name     *string             `json:"name"`
	Kind     *model.CategoryKind `json:"kind"`
	ParentID *int                `json:"parent_id"`
}

func RegisterRoutes(rg *gin.RouterGroup, db *gorm.DB) {
	h := Handler{db: db}

	rg.POST("", h.create)       // 新建数据
	rg.GET("", h.list)          // 列出所有数据
	rg.GET("/:id", h.get)       // 获取单个数据
	rg.PATCH("/:id", h.update)  // 更新数据
	rg.DELETE("/:id", h.delete) // 删除数据
}

func (h Handler) create(c *gin.Context) {
	var req createCategoriesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var category model.Category
	if req.LedgerID == nil {
		category.LedgerID = 1
	} else {
		category.LedgerID = *req.LedgerID
	}
	category.Name = req.Name
	category.Kind = req.Kind
	category.ParentID = req.ParentID

	if err := h.db.Create(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create category"})
		return
	}
}
func (h Handler) list(c *gin.Context) {
	var categories []model.Category
	if err := h.db.
		Model(&model.Category{}).
		Select("id, ledger_id, name, kind, parent_id, deleted_at").
		Order("id").
		Find(&categories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query categories"})
		return
	}

	c.JSON(http.StatusOK, categories)
}

func (h Handler) get(c *gin.Context) {

}

func (h Handler) update(c *gin.Context) {
	id, ok := parseID(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req updateCategoriesRequest
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

	var category model.Category
	err := h.db.First(&category, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load category"})
		return
	}

	if _, ok := raw["ledger_id"]; ok {
		if req.LedgerID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ledger_id cannot be null"})
			return
		}
		category.LedgerID = *req.LedgerID
	}

	if _, ok := raw["name"]; ok {
		if req.Name == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name cannot be null"})
			return
		}
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name cannot be empty"})
			return
		}
		category.Name = name
	}

	if _, ok := raw["kind"]; ok {
		if req.Kind == nil || !req.Kind.IsValid() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "kind must be one of: income, expense, transfer, investment"})
			return
		}
		category.Kind = *req.Kind
	}

	if _, ok := raw["parent_id"]; ok {
		if string(raw["parent_id"]) == "null" {
			category.ParentID = nil
		} else {
			if req.ParentID == nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "parent_id must be a number or null"})
				return
			}
			if *req.ParentID == 0 {
				category.ParentID = nil
			} else {
				if int(id) == *req.ParentID {
					c.JSON(http.StatusBadRequest, gin.H{"error": "parent_id cannot be self"})
					return
				}
				var parent model.Category
				parentErr := h.db.First(&parent, *req.ParentID).Error
				if errors.Is(parentErr, gorm.ErrRecordNotFound) {
					c.JSON(http.StatusBadRequest, gin.H{"error": "parent category not found"})
					return
				}
				if parentErr != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load parent category"})
					return
				}
				category.ParentID = req.ParentID
			}
		}
	}

	if string(category.Kind) != "" && category.ParentID != nil {
		var parent model.Category
		if err := h.db.First(&parent, *category.ParentID).Error; err == nil {
			if parent.Kind != category.Kind {
				c.JSON(http.StatusBadRequest, gin.H{"error": "parent kind must match category kind"})
				return
			}
		}
	}

	if err := h.db.Save(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update category"})
		return
	}

	c.JSON(http.StatusOK, category)
}

func (h Handler) delete(c *gin.Context) {
	id, ok := parseID(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var category model.Category
	err := h.db.First(&category, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load category"})
		return
	}

	var childCount int64
	if err := h.db.Model(&model.Category{}).
		Where("parent_id = ? AND deleted_at IS NULL", id).
		Count(&childCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check child categories"})
		return
	}
	if childCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category has child categories"})
		return
	}

	tx := h.db.Delete(&model.Category{}, id)
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete category"})
		return
	}
	if tx.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
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
