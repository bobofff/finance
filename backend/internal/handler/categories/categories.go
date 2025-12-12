package categories

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	db *gorm.DB
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

}
func (h Handler) list(c *gin.Context) {

}

func (h Handler) get(c *gin.Context) {

}

func (h Handler) update(c *gin.Context) {

}

func (h Handler) delete(c *gin.Context) {

}
