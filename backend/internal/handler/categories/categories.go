package categories

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	db *gorm.DB
}

type createCategoriesRequest struct {
	LedgerID *int   `json:"ledger_id"`
	Name     string `json:"name" binding:"required"`
	Kind     string `json:"kind" binding:"required"`
	ParentID *int   `json:"parent_id"`
}

type updateCategoriesRequest struct {
	LedgerID *int    `json:"ledger_id"`
	Name     *string `json:"name"`
	Kind     *string `json:"kind"`
	ParentID *int    `json:"parent_id"`
}

func RegisterRoutes(rg *gin.RouterGroup, db *gorm.DB) {
	h := Handler{db: db}

	rg.POST("", h.create)       // 新建账户
	rg.GET("", h.list)          // 列出所有账户
	rg.GET("/:id", h.get)       // 获取单个账户
	rg.PATCH("/:id", h.update)  // 更新账户
	rg.DELETE("/:id", h.delete) // 删除账户
}

func (h Handler) create(c *gin.Context) {
	var req createCategoriesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
}
func (h Handler) list(c *gin.Context) {

}

func (h Handler) get(c *gin.Context) {

}

func (h Handler) update(c *gin.Context) {
	var req updateCategoriesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
}

func (h Handler) delete(c *gin.Context) {

}
