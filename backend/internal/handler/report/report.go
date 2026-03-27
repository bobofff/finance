package report

import (
	"errors"
	"net/http"
	"sort"
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
	rg.GET("/balance-sheet", h.balanceSheet)
	rg.GET("/income-expense-summary", h.incomeExpenseSummary)
	rg.GET("/annual-asset-progress", h.annualAssetProgress)
}

type balanceSheetAccount struct {
	ID       uint    `json:"id"`
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Currency string  `json:"currency"`
	IsActive bool    `json:"is_active"`
	Balance  float64 `json:"balance"`
}

type balanceSheetGroup struct {
	Key      string                `json:"key"`
	Label    string                `json:"label"`
	Total    float64               `json:"total"`
	Accounts []balanceSheetAccount `json:"accounts"`
}

type balanceSheetResponse struct {
	LedgerID int                 `json:"ledger_id"`
	AsOf     string              `json:"as_of"`
	Totals   map[string]float64  `json:"totals"`
	Groups   []balanceSheetGroup `json:"groups"`
}

type snapshotRow struct {
	AccountID uint
	AsOf      time.Time
	Amount    float64
}

type annualIncomeExpenseRow struct {
	OccurredOn   time.Time `gorm:"column:occurred_on"`
	CategoryKind string    `gorm:"column:category_kind"`
	Amount       float64   `gorm:"column:amount"`
}

type incomeExpenseDetailRow struct {
	OccurredOn   time.Time `gorm:"column:occurred_on"`
	CategoryKind string    `gorm:"column:category_kind"`
	Amount       float64   `gorm:"column:amount"`
	CategoryID   int       `gorm:"column:category_id"`
	CategoryName string    `gorm:"column:category_name"`
	ParentID     *int      `gorm:"column:parent_id"`
	ParentName   *string   `gorm:"column:parent_name"`
}

type annualAssetGoalSummary struct {
	TargetNetWorth float64 `json:"target_net_worth"`
	BaselineDate   string  `json:"baseline_date"`
}

type annualAssetProgressMonth struct {
	Month             string  `json:"month"`
	Income            float64 `json:"income"`
	Expense           float64 `json:"expense"`
	NetIncome         float64 `json:"net_income"`
	RequiredNetIncome float64 `json:"required_net_income"`
	Completion        float64 `json:"completion"`
	IsFuture          bool    `json:"is_future"`
}

type annualAssetProgressResponse struct {
	LedgerID                 int                        `json:"ledger_id"`
	Year                     int                        `json:"year"`
	AsOf                     string                     `json:"as_of"`
	Goal                     annualAssetGoalSummary     `json:"goal"`
	CurrentNetWorth          float64                    `json:"current_net_worth"`
	BaselineNetWorth         float64                    `json:"baseline_net_worth"`
	RemainingGap             float64                    `json:"remaining_gap"`
	RemainingMonths          int                        `json:"remaining_months"`
	RequiredMonthlyNetIncome float64                    `json:"required_monthly_net_income"`
	Progress                 float64                    `json:"progress"`
	Months                   []annualAssetProgressMonth `json:"months"`
}

type incomeExpenseSummaryTotals struct {
	Income           float64 `json:"income"`
	Expense          float64 `json:"expense"`
	NetIncome        float64 `json:"net_income"`
	TransactionCount int64   `json:"transaction_count"`
}

type incomeExpenseSummaryDay struct {
	Date      string  `json:"date"`
	Income    float64 `json:"income"`
	Expense   float64 `json:"expense"`
	NetIncome float64 `json:"net_income"`
}

type incomeExpenseSummarySubCategory struct {
	ID     int     `json:"id"`
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
	Ratio  float64 `json:"ratio"`
}

type incomeExpenseSummaryCategory struct {
	ID       int                               `json:"id"`
	Name     string                            `json:"name"`
	Amount   float64                           `json:"amount"`
	Ratio    float64                           `json:"ratio"`
	Children []incomeExpenseSummarySubCategory `json:"children"`
}

type incomeExpenseSummaryBreakdown struct {
	Income  []incomeExpenseSummaryCategory `json:"income"`
	Expense []incomeExpenseSummaryCategory `json:"expense"`
}

type incomeExpenseSummaryResponse struct {
	LedgerID  int                           `json:"ledger_id"`
	DateFrom  string                        `json:"date_from"`
	DateTo    string                        `json:"date_to"`
	Totals    incomeExpenseSummaryTotals    `json:"totals"`
	Days      []incomeExpenseSummaryDay     `json:"days"`
	Breakdown incomeExpenseSummaryBreakdown `json:"breakdown"`
}

func (h Handler) balanceSheet(c *gin.Context) {
	ledgerID := 1
	if value := strings.TrimSpace(c.Query("ledger_id")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ledger_id"})
			return
		}
		ledgerID = parsed
	}

	asOf := time.Now()
	if value := strings.TrimSpace(c.Query("as_of")); value != "" {
		parsed, err := time.ParseInLocation("2006-01-02", value, time.Local)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "as_of must be YYYY-MM-DD"})
			return
		}
		asOf = parsed
	}

	var accounts []model.Account
	if err := h.db.Where("ledger_id = ?", ledgerID).Order("id").Find(&accounts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query accounts"})
		return
	}

	var snapshots []snapshotRow
	query := `
SELECT s.account_id, s.as_of, SUM(s.amount) AS amount
FROM fin_account_snapshots s
JOIN (
  SELECT account_id, MAX(as_of) AS max_asof
  FROM fin_account_snapshots
  WHERE ledger_id = ? AND as_of <= ? AND deleted_at IS NULL
  GROUP BY account_id
) latest
ON s.account_id = latest.account_id AND s.as_of = latest.max_asof
WHERE s.ledger_id = ? AND s.deleted_at IS NULL
GROUP BY s.account_id, s.as_of`

	if err := h.db.Raw(query, ledgerID, asOf, ledgerID).Scan(&snapshots).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query snapshots"})
		return
	}

	snapshotMap := make(map[uint]snapshotRow, len(snapshots))
	for _, row := range snapshots {
		snapshotMap[row.AccountID] = row
	}

	groups := map[string]*balanceSheetGroup{
		"asset":     {Key: "asset", Label: "资产"},
		"liability": {Key: "liability", Label: "负债"},
		"other":     {Key: "other", Label: "其他"},
	}

	totalAssets := 0.0
	totalLiabilities := 0.0

	for _, account := range accounts {
		snapshot := snapshotMap[account.ID]
		sum := 0.0

		tx := h.db.Table("fin_transaction_lines").
			Joins("JOIN fin_transactions t ON t.id = fin_transaction_lines.transaction_id AND t.deleted_at IS NULL").
			Where("fin_transaction_lines.account_id = ? AND fin_transaction_lines.ledger_id = ? AND fin_transaction_lines.deleted_at IS NULL", account.ID, ledgerID).
			Select("COALESCE(SUM(fin_transaction_lines.amount), 0)")

		if !snapshot.AsOf.IsZero() {
			tx = tx.Where("t.occurred_on >= ? AND t.occurred_on <= ?", snapshot.AsOf, asOf)
		} else {
			tx = tx.Where("t.occurred_on <= ?", asOf)
		}

		if err := tx.Scan(&sum).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query transactions"})
			return
		}

		balance := snapshot.Amount + sum

		entry := balanceSheetAccount{
			ID:       account.ID,
			Name:     account.Name,
			Type:     account.Type,
			Currency: account.Currency,
			IsActive: account.IsActive,
			Balance:  balance,
		}

		groupKey := classifyAccountType(account.Type)
		group := groups[groupKey]
		group.Accounts = append(group.Accounts, entry)
		group.Total += balance

		if groupKey == "asset" {
			totalAssets += balance
		} else if groupKey == "liability" {
			totalLiabilities += balance
		}
	}

	resp := balanceSheetResponse{
		LedgerID: ledgerID,
		AsOf:     asOf.Format("2006-01-02"),
		Totals: map[string]float64{
			"assets":      totalAssets,
			"liabilities": totalLiabilities,
			"net_worth":   totalAssets - totalLiabilities,
		},
		Groups: []balanceSheetGroup{
			*groups["asset"],
			*groups["liability"],
			*groups["other"],
		},
	}

	c.JSON(http.StatusOK, resp)
}

func (h Handler) incomeExpenseSummary(c *gin.Context) {
	ledgerID := 1
	if value := strings.TrimSpace(c.Query("ledger_id")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ledger_id"})
			return
		}
		ledgerID = parsed
	}

	now := time.Now()
	dateFrom := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	dateTo := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)

	if value := strings.TrimSpace(c.Query("date_from")); value != "" {
		parsed, err := time.ParseInLocation("2006-01-02", value, time.Local)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "date_from must be YYYY-MM-DD"})
			return
		}
		dateFrom = parsed
	}

	if value := strings.TrimSpace(c.Query("date_to")); value != "" {
		parsed, err := time.ParseInLocation("2006-01-02", value, time.Local)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "date_to must be YYYY-MM-DD"})
			return
		}
		dateTo = parsed
	}

	if dateFrom.After(dateTo) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_from cannot be after date_to"})
		return
	}

	rows, err := h.loadIncomeExpenseDetailRows(ledgerID, dateFrom, dateTo.AddDate(0, 0, 1))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query income expense summary"})
		return
	}

	type dayBucket struct {
		Income    float64
		Expense   float64
		NetIncome float64
	}

	dayKeys := make([]string, 0)
	buckets := make(map[string]*dayBucket)
	for day := dateFrom; !day.After(dateTo); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		dayKeys = append(dayKeys, key)
		buckets[key] = &dayBucket{}
	}

	breakdownAgg := map[string]map[int]*topCategoryAgg{
		"income":  {},
		"expense": {},
	}

	totals := incomeExpenseSummaryTotals{
		TransactionCount: int64(len(rows)),
	}

	for _, row := range rows {
		amount := normalizeSignedAmount(row.Amount)
		key := row.OccurredOn.Format("2006-01-02")
		bucket, ok := buckets[key]
		if !ok {
			continue
		}

		switch row.CategoryKind {
		case "income":
			bucket.Income += amount
			bucket.NetIncome += amount
			totals.Income += amount
			totals.NetIncome += amount
		case "expense":
			bucket.Expense += amount
			bucket.NetIncome -= amount
			totals.Expense += amount
			totals.NetIncome -= amount
		default:
			continue
		}

		topID := row.CategoryID
		topName := row.CategoryName
		childID := row.CategoryID
		childName := row.CategoryName

		if row.ParentID != nil && *row.ParentID > 0 {
			topID = *row.ParentID
			if row.ParentName != nil && strings.TrimSpace(*row.ParentName) != "" {
				topName = strings.TrimSpace(*row.ParentName)
			}
		} else {
			childID = 0
			childName = "未分二级分类"
		}

		kindAgg := breakdownAgg[row.CategoryKind]
		top, exists := kindAgg[topID]
		if !exists {
			top = &topCategoryAgg{
				ID:       topID,
				Name:     topName,
				Children: make(map[int]*subCategoryAgg),
			}
			kindAgg[topID] = top
		}
		top.Amount += amount

		child, childExists := top.Children[childID]
		if !childExists {
			child = &subCategoryAgg{
				ID:   childID,
				Name: childName,
			}
			top.Children[childID] = child
		}
		child.Amount += amount
	}

	days := make([]incomeExpenseSummaryDay, 0, len(dayKeys))
	for _, key := range dayKeys {
		bucket := buckets[key]
		days = append(days, incomeExpenseSummaryDay{
			Date:      key,
			Income:    bucket.Income,
			Expense:   bucket.Expense,
			NetIncome: bucket.NetIncome,
		})
	}

	resp := incomeExpenseSummaryResponse{
		LedgerID:  ledgerID,
		DateFrom:  dateFrom.Format("2006-01-02"),
		DateTo:    dateTo.Format("2006-01-02"),
		Totals:    totals,
		Days:      days,
		Breakdown: buildIncomeExpenseBreakdown(breakdownAgg, totals),
	}

	c.JSON(http.StatusOK, resp)
}

func (h Handler) annualAssetProgress(c *gin.Context) {
	ledgerID := 1
	if value := strings.TrimSpace(c.Query("ledger_id")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ledger_id"})
			return
		}
		ledgerID = parsed
	}

	asOf := time.Now()
	if value := strings.TrimSpace(c.Query("as_of")); value != "" {
		parsed, err := time.ParseInLocation("2006-01-02", value, time.Local)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "as_of must be YYYY-MM-DD"})
			return
		}
		asOf = parsed
	}

	year := asOf.Year()
	if value := strings.TrimSpace(c.Query("year")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1900 || parsed > 9999 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "year must be valid YYYY"})
			return
		}
		year = parsed
	}

	var goal model.AnnualAssetGoal
	err := h.db.Where("ledger_id = ? AND year = ?", ledgerID, year).First(&goal).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "annual goal not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query annual goal"})
		return
	}

	currentNetWorth, err := h.calculateNetWorth(ledgerID, asOf)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to calculate current net worth"})
		return
	}

	periodStart := time.Date(year, time.January, 1, 0, 0, 0, 0, time.Local)
	horizonMonthStart := resolveHorizonMonthStart(year, goal.BaselineDate)
	periodEndExclusive := horizonMonthStart.AddDate(0, 1, 0)

	baselineRefDate := goal.BaselineDate
	if baselineRefDate.Year() != year || baselineRefDate.After(asOf) {
		baselineRefDate = periodStart
	}
	baselineNetWorth, err := h.calculateNetWorth(ledgerID, baselineRefDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to calculate baseline net worth"})
		return
	}

	remainingGap := goal.TargetNetWorth - currentNetWorth
	remainingMonths := calculateRemainingMonths(asOf, periodStart, horizonMonthStart)
	requiredMonthlyNetIncome := 0.0
	if remainingGap > 0 && remainingMonths > 0 {
		requiredMonthlyNetIncome = remainingGap / float64(remainingMonths)
	}

	monthlyData, err := h.loadIncomeExpense(ledgerID, periodStart, periodEndExclusive)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query yearly cashflow"})
		return
	}

	months := buildAnnualProgressMonths(periodStart, horizonMonthStart, asOf, monthlyData, requiredMonthlyNetIncome)

	targetDelta := goal.TargetNetWorth - baselineNetWorth
	progress := 0.0
	if targetDelta == 0 {
		if currentNetWorth >= goal.TargetNetWorth {
			progress = 1
		}
	} else {
		progress = (currentNetWorth - baselineNetWorth) / targetDelta
	}

	resp := annualAssetProgressResponse{
		LedgerID: ledgerID,
		Year:     year,
		AsOf:     asOf.Format("2006-01-02"),
		Goal: annualAssetGoalSummary{
			TargetNetWorth: goal.TargetNetWorth,
			BaselineDate:   goal.BaselineDate.Format("2006-01-02"),
		},
		CurrentNetWorth:          currentNetWorth,
		BaselineNetWorth:         baselineNetWorth,
		RemainingGap:             remainingGap,
		RemainingMonths:          remainingMonths,
		RequiredMonthlyNetIncome: requiredMonthlyNetIncome,
		Progress:                 progress,
		Months:                   months,
	}

	c.JSON(http.StatusOK, resp)
}

func classifyAccountType(accountType string) string {
	switch strings.ToLower(strings.TrimSpace(accountType)) {
	case "cash", "investment", "other_asset":
		return "asset"
	case "debt", "receivable":
		return "asset"
	case "liability":
		return "liability"
	default:
		return "other"
	}
}

func (h Handler) calculateNetWorth(ledgerID int, asOf time.Time) (float64, error) {
	var accounts []model.Account
	if err := h.db.Where("ledger_id = ?", ledgerID).Order("id").Find(&accounts).Error; err != nil {
		return 0, err
	}

	var snapshots []snapshotRow
	query := `
SELECT s.account_id, s.as_of, SUM(s.amount) AS amount
FROM fin_account_snapshots s
JOIN (
  SELECT account_id, MAX(as_of) AS max_asof
  FROM fin_account_snapshots
  WHERE ledger_id = ? AND as_of <= ? AND deleted_at IS NULL
  GROUP BY account_id
) latest
ON s.account_id = latest.account_id AND s.as_of = latest.max_asof
WHERE s.ledger_id = ? AND s.deleted_at IS NULL
GROUP BY s.account_id, s.as_of`

	if err := h.db.Raw(query, ledgerID, asOf, ledgerID).Scan(&snapshots).Error; err != nil {
		return 0, err
	}

	snapshotMap := make(map[uint]snapshotRow, len(snapshots))
	for _, row := range snapshots {
		snapshotMap[row.AccountID] = row
	}

	totalAssets := 0.0
	totalLiabilities := 0.0

	for _, account := range accounts {
		snapshot := snapshotMap[account.ID]
		sum := 0.0

		tx := h.db.Table("fin_transaction_lines").
			Joins("JOIN fin_transactions t ON t.id = fin_transaction_lines.transaction_id AND t.deleted_at IS NULL").
			Where("fin_transaction_lines.account_id = ? AND fin_transaction_lines.ledger_id = ? AND fin_transaction_lines.deleted_at IS NULL", account.ID, ledgerID).
			Select("COALESCE(SUM(fin_transaction_lines.amount), 0)")

		if !snapshot.AsOf.IsZero() {
			tx = tx.Where("t.occurred_on >= ? AND t.occurred_on <= ?", snapshot.AsOf, asOf)
		} else {
			tx = tx.Where("t.occurred_on <= ?", asOf)
		}

		if err := tx.Scan(&sum).Error; err != nil {
			return 0, err
		}

		balance := snapshot.Amount + sum
		switch classifyAccountType(account.Type) {
		case "asset":
			totalAssets += balance
		case "liability":
			totalLiabilities += balance
		}
	}

	return totalAssets - totalLiabilities, nil
}

func (h Handler) loadIncomeExpense(ledgerID int, periodStart time.Time, periodEndExclusive time.Time) ([]annualIncomeExpenseRow, error) {
	var rows []annualIncomeExpenseRow
	err := h.db.Table("fin_transaction_lines tl").
		Joins("JOIN fin_transactions t ON t.id = tl.transaction_id AND t.deleted_at IS NULL").
		Joins("JOIN fin_categories c ON c.id = tl.category_id AND c.deleted_at IS NULL").
		Where("tl.ledger_id = ? AND tl.deleted_at IS NULL", ledgerID).
		Where("c.kind IN ('income', 'expense')").
		Where("t.occurred_on >= ? AND t.occurred_on < ?", periodStart, periodEndExclusive).
		Select("t.occurred_on, c.kind AS category_kind, tl.amount").
		Scan(&rows).Error

	return rows, err
}

func (h Handler) loadIncomeExpenseDetailRows(ledgerID int, periodStart time.Time, periodEndExclusive time.Time) ([]incomeExpenseDetailRow, error) {
	var rows []incomeExpenseDetailRow
	err := h.db.Table("fin_transaction_lines tl").
		Joins("JOIN fin_transactions t ON t.id = tl.transaction_id AND t.deleted_at IS NULL").
		Joins("JOIN fin_categories c ON c.id = tl.category_id AND c.deleted_at IS NULL").
		Joins("LEFT JOIN fin_categories p ON p.id = c.parent_id AND p.deleted_at IS NULL").
		Where("tl.ledger_id = ? AND tl.deleted_at IS NULL", ledgerID).
		Where("c.kind IN ('income', 'expense')").
		Where("t.occurred_on >= ? AND t.occurred_on < ?", periodStart, periodEndExclusive).
		Select("t.occurred_on, c.kind AS category_kind, tl.amount, c.id AS category_id, c.name AS category_name, c.parent_id, p.name AS parent_name").
		Scan(&rows).Error
	return rows, err
}

func buildIncomeExpenseBreakdown(agg map[string]map[int]*topCategoryAgg, totals incomeExpenseSummaryTotals) incomeExpenseSummaryBreakdown {
	return incomeExpenseSummaryBreakdown{
		Income:  buildIncomeExpenseCategoryList(agg["income"], totals.Income),
		Expense: buildIncomeExpenseCategoryList(agg["expense"], totals.Expense),
	}
}

type subCategoryAgg struct {
	ID     int
	Name   string
	Amount float64
}

type topCategoryAgg struct {
	ID       int
	Name     string
	Amount   float64
	Children map[int]*subCategoryAgg
}

func buildIncomeExpenseCategoryList(data map[int]*topCategoryAgg, totalAmount float64) []incomeExpenseSummaryCategory {
	items := make([]incomeExpenseSummaryCategory, 0, len(data))
	for _, top := range data {
		children := make([]incomeExpenseSummarySubCategory, 0, len(top.Children))
		for _, child := range top.Children {
			childRatio := 0.0
			if top.Amount > 0 {
				childRatio = child.Amount / top.Amount
			}
			children = append(children, incomeExpenseSummarySubCategory{
				ID:     child.ID,
				Name:   child.Name,
				Amount: child.Amount,
				Ratio:  childRatio,
			})
		}

		sort.Slice(children, func(i, j int) bool {
			if children[i].Amount == children[j].Amount {
				return children[i].Name < children[j].Name
			}
			return children[i].Amount > children[j].Amount
		})

		ratio := 0.0
		if totalAmount > 0 {
			ratio = top.Amount / totalAmount
		}

		items = append(items, incomeExpenseSummaryCategory{
			ID:       top.ID,
			Name:     top.Name,
			Amount:   top.Amount,
			Ratio:    ratio,
			Children: children,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Amount == items[j].Amount {
			return items[i].Name < items[j].Name
		}
		return items[i].Amount > items[j].Amount
	})
	return items
}

func normalizeSignedAmount(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func buildAnnualProgressMonths(periodStart time.Time, periodEndMonthStart time.Time, asOf time.Time, rows []annualIncomeExpenseRow, requiredMonthlyNetIncome float64) []annualAssetProgressMonth {
	type monthBucket struct {
		Income    float64
		Expense   float64
		NetIncome float64
	}

	buckets := make(map[string]*monthBucket)
	monthCount := 0
	for month := periodStart; !month.After(periodEndMonthStart); month = month.AddDate(0, 1, 0) {
		key := month.Format("2006-01")
		buckets[key] = &monthBucket{}
		monthCount++
	}

	for _, row := range rows {
		key := row.OccurredOn.Format("2006-01")
		bucket, ok := buckets[key]
		if !ok {
			continue
		}
		switch row.CategoryKind {
		case "income":
			bucket.Income += row.Amount
			bucket.NetIncome += row.Amount
		case "expense":
			bucket.Expense += -row.Amount
			bucket.NetIncome += row.Amount
		}
	}

	asOfMonthStart := time.Date(asOf.Year(), asOf.Month(), 1, 0, 0, 0, 0, time.Local)
	months := make([]annualAssetProgressMonth, 0, monthCount)
	for monthDate := periodStart; !monthDate.After(periodEndMonthStart); monthDate = monthDate.AddDate(0, 1, 0) {
		key := monthDate.Format("2006-01")
		bucket := buckets[key]

		completion := 1.0
		if requiredMonthlyNetIncome > 0 {
			completion = bucket.NetIncome / requiredMonthlyNetIncome
		}

		months = append(months, annualAssetProgressMonth{
			Month:             key,
			Income:            bucket.Income,
			Expense:           bucket.Expense,
			NetIncome:         bucket.NetIncome,
			RequiredNetIncome: requiredMonthlyNetIncome,
			Completion:        completion,
			IsFuture:          monthDate.After(asOfMonthStart),
		})
	}

	return months
}

func calculateRemainingMonths(asOf time.Time, periodStart time.Time, periodEndMonthStart time.Time) int {
	if periodEndMonthStart.Before(periodStart) {
		return 0
	}

	nextMonth := time.Date(asOf.Year(), asOf.Month(), 1, 0, 0, 0, 0, time.Local).AddDate(0, 1, 0)
	if asOf.Before(periodStart) {
		nextMonth = periodStart
	}
	if nextMonth.Before(periodStart) {
		nextMonth = periodStart
	}
	if nextMonth.After(periodEndMonthStart) {
		return 0
	}

	count := 0
	for month := nextMonth; !month.After(periodEndMonthStart); month = month.AddDate(0, 1, 0) {
		count++
	}
	return count
}

func resolveHorizonMonthStart(year int, baselineDate time.Time) time.Time {
	defaultHorizon := time.Date(year, time.December, 1, 0, 0, 0, 0, time.Local)
	if baselineDate.Year() != year+1 {
		return defaultHorizon
	}

	// baseline_date 落在次年时，按“基准日所在月份的前一个月”为目标期最后一个月。
	boundaryMonthStart := time.Date(baselineDate.Year(), baselineDate.Month(), 1, 0, 0, 0, 0, time.Local)
	extended := boundaryMonthStart.AddDate(0, -1, 0)
	if extended.After(defaultHorizon) {
		return extended
	}
	return defaultHorizon
}
