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

// Handler 封装账户相关的数据库实例，用于绑定到路由上。
type Handler struct {
	db *gorm.DB
}

// RegisterRoutes 将账户 CRUD 路由注册到 /api/accounts 下。
func RegisterRoutes(rg *gin.RouterGroup, db *gorm.DB) {
	h := Handler{db: db}

	rg.POST("", h.create)       // 新建账户
	rg.GET("", h.list)          // 列出所有账户
	rg.GET("/:id", h.get)       // 获取单个账户
	rg.PATCH("/:id", h.update)  // 更新账户
	rg.DELETE("/:id", h.delete) // 删除账户
}

// allowedTypes 允许的账户类型白名单。
var allowedTypes = map[string]struct{}{
	"cash":        {},
	"liability":   {},
	"debt":        {},
	"investment":  {},
	"other_asset": {},
}

// createAccountRequest 新建账户时的请求体。
type createAccountRequest struct {
	Name     string `json:"name" binding:"required"` // 账户名称（必填）
	Type     string `json:"type" binding:"required"` // 账户类型（必填）
	Currency string `json:"currency"`                // 币种，缺省为 CNY
	IsActive *bool  `json:"is_active"`               // 是否启用，缺省 true
}

// updateAccountRequest 更新账户时的请求体（全部字段可选）。
type updateAccountRequest struct {
	Name     *string `json:"name"`      // 新名称
	Type     *string `json:"type"`      // 新类型
	Currency *string `json:"currency"`  // 新币种
	IsActive *bool   `json:"is_active"` // 新启用状态
}

// create 处理创建账户：校验入参、类型是否合法，写入数据库并返回新账户。
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

// list 查询全部账户，按 id 排序返回。
func (h Handler) list(c *gin.Context) {
	var accounts []model.Account
	if err := h.db.Order("id").Find(&accounts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query accounts"})
		return
	}

	c.JSON(http.StatusOK, accounts)
}

// get 按 id 查询单个账户；不存在时返回 404。
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

// update 部分更新账户：只修改传入的字段，返回更新后的记录。
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

// delete 软删除账户：若不存在返回 404，成功返回 204。
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

// parseID 将路径参数转换为 uint，失败返回 false。
func parseID(raw string) (uint, bool) {
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, false
	}
	return uint(id), true
}

// normalizeType 将类型字符串去空格、转小写，并检查是否在允许列表。
func normalizeType(input string) (string, bool) {
	value := strings.ToLower(strings.TrimSpace(input))
	_, ok := allowedTypes[value]
	return value, ok
}
