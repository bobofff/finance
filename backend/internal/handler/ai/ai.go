package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"finance-backend/internal/logging"
	"finance-backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	confidenceThreshold  = 0.75
	maxModelCallAttempts = 2
	modelRetryBackoff    = 250 * time.Millisecond
)

type Config struct {
	OpenAIAPIKey          string
	OpenAIBaseURL         string
	OpenAIModel           string
	RequestTimeoutSeconds int
	UseEnvProxy           bool
	Timezone              string
}

type Handler struct {
	db     *gorm.DB
	client *http.Client
	config Config
}

func RegisterRoutes(rg *gin.RouterGroup, db *gorm.DB, cfg Config) {
	h := &Handler{
		db:     db,
		client: newHTTPClient(cfg),
		config: cfg,
	}

	rg.POST("/parse-transaction", h.parseTransaction)
}

type parseTransactionRequest struct {
	LedgerID *int   `json:"ledger_id"`
	Text     string `json:"text" binding:"required"`
}

type parseTransactionResponse struct {
	LedgerID   int                 `json:"ledger_id"`
	SourceText string              `json:"source_text"`
	Draft      transactionDraftDTO `json:"draft"`
}

type transactionDraftDTO struct {
	Kind                string   `json:"kind"`
	OccurredOn          string   `json:"occurred_on"`
	AccountID           *uint    `json:"account_id,omitempty"`
	AccountName         string   `json:"account_name"`
	CategoryID          *int     `json:"category_id,omitempty"`
	CategoryName        string   `json:"category_name"`
	Amount              float64  `json:"amount"`
	Description         string   `json:"description"`
	Note                string   `json:"note"`
	MissingFields       []string `json:"missing_fields"`
	LowConfidenceFields []string `json:"low_confidence_fields"`
}

type accountOption struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Label string `json:"label"`
}

type categoryOption struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	Path  string `json:"path"`
	Label string `json:"label"`
}

type aiPromptPayload struct {
	SourceText        string           `json:"source_text"`
	Today             string           `json:"today"`
	Timezone          string           `json:"timezone"`
	AvailableAccounts []accountOption  `json:"available_accounts"`
	AvailableCats     []categoryOption `json:"available_categories"`
}

type modelTransactionDraft struct {
	Kind                  string  `json:"kind"`
	OccurredOn            string  `json:"occurred_on"`
	AccountName           string  `json:"account_name"`
	CategoryName          string  `json:"category_name"`
	Amount                float64 `json:"amount"`
	Description           string  `json:"description"`
	Note                  string  `json:"note"`
	KindConfidence        float64 `json:"kind_confidence"`
	OccurredOnConfidence  float64 `json:"occurred_on_confidence"`
	AccountConfidence     float64 `json:"account_confidence"`
	CategoryConfidence    float64 `json:"category_confidence"`
	AmountConfidence      float64 `json:"amount_confidence"`
	DescriptionConfidence float64 `json:"description_confidence"`
	NoteConfidence        float64 `json:"note_confidence"`
}

type openAIChatCompletionRequest struct {
	Model          string                `json:"model"`
	Messages       []openAIChatMessage   `json:"messages"`
	Temperature    float64               `json:"temperature"`
	ResponseFormat *openAIResponseFormat `json:"response_format,omitempty"`
}

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponseFormat struct {
	Type       string               `json:"type"`
	JSONSchema openAIResponseSchema `json:"json_schema"`
}

type openAIResponseSchema struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type openAIChatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage *openAIUsage         `json:"usage,omitempty"`
	Error *openAIErrorResponse `json:"error,omitempty"`
}

type openAIErrorResponse struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    any    `json:"code"`
}

type openAIUsage struct {
	PromptTokens            int                            `json:"prompt_tokens"`
	CompletionTokens        int                            `json:"completion_tokens"`
	TotalTokens             int                            `json:"total_tokens"`
	PromptTokensDetails     *openAIPromptTokensDetails     `json:"prompt_tokens_details,omitempty"`
	CompletionTokensDetails *openAICompletionTokensDetails `json:"completion_tokens_details,omitempty"`
}

type openAIPromptTokensDetails struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
	AudioTokens  int `json:"audio_tokens,omitempty"`
}

type openAICompletionTokensDetails struct {
	ReasoningTokens          int `json:"reasoning_tokens,omitempty"`
	AudioTokens              int `json:"audio_tokens,omitempty"`
	AcceptedPredictionTokens int `json:"accepted_prediction_tokens,omitempty"`
	RejectedPredictionTokens int `json:"rejected_prediction_tokens,omitempty"`
}

func (h *Handler) parseTransaction(c *gin.Context) {
	var req parseTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logging.AbortWithError(c, http.StatusBadRequest, err.Error(), err)
		return
	}

	text := strings.TrimSpace(req.Text)
	if text == "" {
		err := errors.New("text is required")
		logging.AbortWithError(c, http.StatusBadRequest, err.Error(), err)
		return
	}

	ledgerID := normalizeLedgerID(req.LedgerID, c)
	if ledgerID == 0 {
		return
	}

	accounts, categories, err := h.loadCandidates(ledgerID)
	if err != nil {
		logging.AbortWithError(c, http.StatusInternalServerError, "failed to load ai candidates", err)
		return
	}

	location := h.timeLocation()
	now := time.Now().In(location)
	payload := aiPromptPayload{
		SourceText:        text,
		Today:             now.Format("2006-01-02"),
		Timezone:          location.String(),
		AvailableAccounts: accounts,
		AvailableCats:     categories,
	}

	rawDraft, err := h.callModel(c.Request.Context(), logging.FromContext(c), payload)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, errAIUnavailable) {
			status = http.StatusServiceUnavailable
		}
		logging.AbortWithError(c, status, err.Error(), err)
		return
	}

	finalDraft, err := h.finalizeDraft(rawDraft, accounts, categories)
	if err != nil {
		logging.AbortWithError(c, http.StatusBadRequest, err.Error(), err)
		return
	}

	c.JSON(http.StatusOK, parseTransactionResponse{
		LedgerID:   ledgerID,
		SourceText: text,
		Draft:      finalDraft,
	})
}

func (h *Handler) finalizeDraft(raw modelTransactionDraft, accounts []accountOption, categories []categoryOption) (transactionDraftDTO, error) {
	draft := transactionDraftDTO{
		Kind:                normalizeTransactionKind(raw.Kind),
		OccurredOn:          strings.TrimSpace(raw.OccurredOn),
		AccountName:         strings.TrimSpace(raw.AccountName),
		CategoryName:        strings.TrimSpace(raw.CategoryName),
		Amount:              math.Abs(raw.Amount),
		Description:         strings.TrimSpace(raw.Description),
		Note:                strings.TrimSpace(raw.Note),
		MissingFields:       []string{},
		LowConfidenceFields: []string{},
	}

	accountByLabel, accountByName := buildAccountIndexes(accounts)
	categoryByLabel, categoryByName := buildCategoryIndexes(categories)

	if draft.AccountName != "" {
		if account, ok := accountByLabel[normalizeKey(draft.AccountName)]; ok {
			id := account.ID
			draft.AccountID = &id
		} else if account, ok := uniqueAccountByName(accountByName, draft.AccountName); ok {
			id := account.ID
			draft.AccountID = &id
			draft.AccountName = account.Name
		}
	}

	if draft.CategoryName != "" {
		if category, ok := categoryByLabel[normalizeKey(draft.CategoryName)]; ok {
			id := category.ID
			draft.CategoryID = &id
			draft.CategoryName = category.Label
		} else if category, ok := uniqueCategoryByName(categoryByName, draft.CategoryName); ok {
			id := category.ID
			draft.CategoryID = &id
			draft.CategoryName = category.Label
		}
	}

	if draft.CategoryID != nil {
		if category, ok := categoryByID(categories, *draft.CategoryID); ok {
			draft.CategoryName = category.Label
			if category.Kind == string(model.CategoryKindIncome) || category.Kind == string(model.CategoryKindExpense) {
				draft.Kind = category.Kind
			}
		}
	}

	if draft.Kind == "" {
		draft.Kind = string(model.CategoryKindExpense)
	}

	draft.Kind = normalizeTransactionKind(draft.Kind)
	if draft.Kind == "" {
		draft.Kind = string(model.CategoryKindExpense)
	}

	if draft.Kind == string(model.CategoryKindExpense) {
		draft.Amount = -math.Abs(draft.Amount)
	} else {
		draft.Amount = math.Abs(draft.Amount)
	}

	if draft.OccurredOn != "" {
		if _, err := time.Parse("2006-01-02", draft.OccurredOn); err != nil {
			draft.OccurredOn = ""
		}
	}

	if draft.AccountID == nil {
		draft.MissingFields = append(draft.MissingFields, "account")
	}
	if draft.CategoryID == nil {
		draft.MissingFields = append(draft.MissingFields, "category")
	}
	if draft.OccurredOn == "" {
		draft.MissingFields = append(draft.MissingFields, "occurred_on")
	}
	if draft.Amount == 0 {
		draft.MissingFields = append(draft.MissingFields, "amount")
	}

	if raw.KindConfidence < confidenceThreshold {
		draft.LowConfidenceFields = append(draft.LowConfidenceFields, "kind")
	}
	if raw.OccurredOnConfidence < confidenceThreshold {
		draft.LowConfidenceFields = append(draft.LowConfidenceFields, "occurred_on")
	}
	if raw.AccountConfidence < confidenceThreshold {
		draft.LowConfidenceFields = append(draft.LowConfidenceFields, "account")
	}
	if raw.CategoryConfidence < confidenceThreshold {
		draft.LowConfidenceFields = append(draft.LowConfidenceFields, "category")
	}
	if raw.AmountConfidence < confidenceThreshold {
		draft.LowConfidenceFields = append(draft.LowConfidenceFields, "amount")
	}
	if raw.DescriptionConfidence < confidenceThreshold {
		draft.LowConfidenceFields = append(draft.LowConfidenceFields, "description")
	}
	if raw.NoteConfidence < confidenceThreshold {
		draft.LowConfidenceFields = append(draft.LowConfidenceFields, "note")
	}

	draft.MissingFields = uniqueSortedStrings(draft.MissingFields)
	draft.LowConfidenceFields = uniqueSortedStrings(draft.LowConfidenceFields)

	return draft, nil
}

func (h *Handler) callModel(ctx context.Context, logger *slog.Logger, payload aiPromptPayload) (modelTransactionDraft, error) {
	if strings.TrimSpace(h.config.OpenAIAPIKey) == "" {
		return modelTransactionDraft{}, errAIUnavailable
	}

	promptBody, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return modelTransactionDraft{}, err
	}

	systemPrompt := strings.TrimSpace(`
你是一个中文财务记账解析器。
你的任务是把用户的一句话转换成结构化草稿，供人工确认后再入账。
必须遵守以下规则：
1. 只输出 JSON，不要输出任何解释、代码块或多余文字。
2. kind 只能是 income 或 expense。
3. amount 必须是正数，表示绝对金额，不要带正负号。
4. account_name 和 category_name 必须从候选列表的 label 中选择，不能自己编造；找不到就留空字符串。
5. occurred_on 必须输出 YYYY-MM-DD 格式的绝对日期；“今天”“昨天”等相对日期要按当前日期换算。
6. description 只保留简短摘要，例如“煎饼果子”“工资”。
7. note 只保留对记账有帮助的补充信息；没有就留空。
8. 字段不确定时就留空，并通过 confidence 降低置信度。
`)

	userPrompt := fmt.Sprintf("请根据下面的 JSON 输入，输出结构化记账草稿：\n%s", string(promptBody))

	requestBody := openAIChatCompletionRequest{
		Model: h.model(),
		Messages: []openAIChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0,
		ResponseFormat: &openAIResponseFormat{
			Type: "json_schema",
			JSONSchema: openAIResponseSchema{
				Name:   "parse_transaction",
				Strict: true,
				Schema: transactionDraftSchema(),
			},
		},
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return modelTransactionDraft{}, err
	}

	endpoint := h.baseURL() + "/chat/completions"
	start := time.Now()
	result := modelCallResult{}
	var callErr error
	defer func() {
		h.logModelInteraction(
			logger,
			requestBody.Model,
			endpoint,
			string(promptBody),
			result.ResponseText,
			result.ResponseBodyText,
			result.StatusCode,
			time.Since(start),
			result.Usage,
			callErr,
		)
	}()

	for attempt := 1; attempt <= maxModelCallAttempts; attempt++ {
		result, callErr = h.executeModelCall(ctx, endpoint, body)
		if callErr == nil {
			return result.Draft, nil
		}

		if attempt >= maxModelCallAttempts || !shouldRetryModelCall(ctx, callErr) {
			return modelTransactionDraft{}, callErr
		}

		if logger != nil {
			logger.Warn(
				"retrying ai request after transient upstream failure",
				slog.Int("attempt", attempt),
				slog.Int("max_attempts", maxModelCallAttempts),
				slog.String("error", callErr.Error()),
			)
		}

		if err := sleepWithContext(ctx, modelRetryBackoff); err != nil {
			callErr = err
			return modelTransactionDraft{}, err
		}
	}

	return modelTransactionDraft{}, callErr
}

func (h *Handler) logModelInteraction(
	logger *slog.Logger,
	model string,
	endpoint string,
	requestPayload string,
	responseContent string,
	responseBody string,
	statusCode int,
	latency time.Duration,
	usage *openAIUsage,
	callErr error,
) {
	if logger == nil {
		return
	}

	attrs := []any{
		slog.String("provider", "openai"),
		slog.String("operation", "parse_transaction"),
		slog.String("model", strings.TrimSpace(model)),
		slog.String("endpoint", endpoint),
		slog.Bool("use_env_proxy", h.config.UseEnvProxy),
		slog.Duration("latency", latency),
		slog.String("request_payload", truncateForLog(strings.TrimSpace(requestPayload), 16*1024)),
	}

	if statusCode > 0 {
		attrs = append(attrs, slog.Int("status", statusCode))
	}
	if strings.TrimSpace(responseContent) != "" {
		attrs = append(attrs, slog.String("response_content", truncateForLog(strings.TrimSpace(responseContent), 16*1024)))
	}
	if strings.TrimSpace(responseBody) != "" && (callErr != nil || strings.TrimSpace(responseContent) == "") {
		attrs = append(attrs, slog.String("response_body", truncateForLog(strings.TrimSpace(responseBody), 16*1024)))
	}
	if usage != nil {
		attrs = append(attrs,
			slog.Int("prompt_tokens", usage.PromptTokens),
			slog.Int("completion_tokens", usage.CompletionTokens),
			slog.Int("total_tokens", usage.TotalTokens),
		)
		if usage.PromptTokensDetails != nil && usage.PromptTokensDetails.CachedTokens > 0 {
			attrs = append(attrs, slog.Int("cached_prompt_tokens", usage.PromptTokensDetails.CachedTokens))
		}
		if usage.CompletionTokensDetails != nil {
			if usage.CompletionTokensDetails.ReasoningTokens > 0 {
				attrs = append(attrs, slog.Int("reasoning_tokens", usage.CompletionTokensDetails.ReasoningTokens))
			}
			if usage.CompletionTokensDetails.AcceptedPredictionTokens > 0 {
				attrs = append(attrs, slog.Int("accepted_prediction_tokens", usage.CompletionTokensDetails.AcceptedPredictionTokens))
			}
			if usage.CompletionTokensDetails.RejectedPredictionTokens > 0 {
				attrs = append(attrs, slog.Int("rejected_prediction_tokens", usage.CompletionTokensDetails.RejectedPredictionTokens))
			}
		}
	}

	if callErr != nil {
		attrs = append(attrs, slog.String("error", callErr.Error()))
		logger.Error("ai interaction failed", attrs...)
		return
	}

	logger.Info("ai interaction completed", attrs...)
}

func (h *Handler) loadCandidates(ledgerID int) ([]accountOption, []categoryOption, error) {
	var accounts []model.Account
	if err := h.db.Where("ledger_id = ? AND is_active = ? AND deleted_at IS NULL", ledgerID, true).Order("id").Find(&accounts).Error; err != nil {
		return nil, nil, err
	}

	var categories []model.Category
	if err := h.db.Where("ledger_id = ? AND deleted_at IS NULL AND kind IN ?", ledgerID, []model.CategoryKind{model.CategoryKindIncome, model.CategoryKindExpense}).Order("id").Find(&categories).Error; err != nil {
		return nil, nil, err
	}

	accountOptions := make([]accountOption, 0, len(accounts))
	for _, account := range accounts {
		accountOptions = append(accountOptions, accountOption{
			ID:    account.ID,
			Name:  account.Name,
			Type:  account.Type,
			Label: fmt.Sprintf("%s [%s]", account.Name, account.Type),
		})
	}

	categoryByIDMap := make(map[int]model.Category, len(categories))
	for _, category := range categories {
		categoryByIDMap[category.ID] = category
	}

	categoryOptions := make([]categoryOption, 0, len(categories))
	for _, category := range categories {
		categoryOptions = append(categoryOptions, categoryOption{
			ID:    category.ID,
			Name:  category.Name,
			Kind:  string(category.Kind),
			Path:  buildCategoryPath(category, categoryByIDMap),
			Label: fmt.Sprintf("%s (%s)", buildCategoryPath(category, categoryByIDMap), category.Kind),
		})
	}

	return accountOptions, categoryOptions, nil
}

func buildCategoryPath(category model.Category, byID map[int]model.Category) string {
	segments := []string{strings.TrimSpace(category.Name)}
	current := category
	visited := map[int]struct{}{category.ID: struct{}{}}

	for current.ParentID != nil {
		parent, ok := byID[*current.ParentID]
		if !ok {
			break
		}
		if _, seen := visited[parent.ID]; seen {
			break
		}
		visited[parent.ID] = struct{}{}
		segments = append([]string{strings.TrimSpace(parent.Name)}, segments...)
		current = parent
	}

	filtered := make([]string, 0, len(segments))
	for _, segment := range segments {
		if strings.TrimSpace(segment) != "" {
			filtered = append(filtered, strings.TrimSpace(segment))
		}
	}
	return strings.Join(filtered, " / ")
}

func buildAccountIndexes(accounts []accountOption) (map[string]accountOption, map[string][]accountOption) {
	byLabel := make(map[string]accountOption, len(accounts))
	byName := make(map[string][]accountOption, len(accounts))
	for _, account := range accounts {
		byLabel[normalizeKey(account.Label)] = account
		byName[normalizeKey(account.Name)] = append(byName[normalizeKey(account.Name)], account)
	}
	return byLabel, byName
}

func buildCategoryIndexes(categories []categoryOption) (map[string]categoryOption, map[string][]categoryOption) {
	byLabel := make(map[string]categoryOption, len(categories))
	byName := make(map[string][]categoryOption, len(categories))
	for _, category := range categories {
		byLabel[normalizeKey(category.Label)] = category
		byName[normalizeKey(category.Name)] = append(byName[normalizeKey(category.Name)], category)
	}
	return byLabel, byName
}

func uniqueAccountByName(index map[string][]accountOption, name string) (accountOption, bool) {
	items := index[normalizeKey(name)]
	if len(items) != 1 {
		return accountOption{}, false
	}
	return items[0], true
}

func uniqueCategoryByName(index map[string][]categoryOption, name string) (categoryOption, bool) {
	items := index[normalizeKey(name)]
	if len(items) != 1 {
		return categoryOption{}, false
	}
	return items[0], true
}

func categoryByID(categories []categoryOption, id int) (categoryOption, bool) {
	for _, category := range categories {
		if category.ID == id {
			return category, true
		}
	}
	return categoryOption{}, false
}

func normalizeTransactionKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case string(model.CategoryKindIncome):
		return string(model.CategoryKindIncome)
	case string(model.CategoryKindExpense):
		return string(model.CategoryKindExpense)
	default:
		return ""
	}
}

func normalizeKey(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func uniqueSortedStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}

	seen := make(map[string]struct{}, len(values))
	uniq := make([]string, 0, len(values))
	for _, value := range values {
		key := normalizeKey(value)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		uniq = append(uniq, value)
	}
	sort.Strings(uniq)
	return uniq
}

func (h *Handler) model() string {
	if strings.TrimSpace(h.config.OpenAIModel) != "" {
		return strings.TrimSpace(h.config.OpenAIModel)
	}
	return "gpt-5-mini"
}

func (h *Handler) baseURL() string {
	baseURL := strings.TrimSpace(h.config.OpenAIBaseURL)
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return strings.TrimRight(baseURL, "/")
}

func (h *Handler) timeLocation() *time.Location {
	if strings.TrimSpace(h.config.Timezone) != "" {
		if loc, err := time.LoadLocation(strings.TrimSpace(h.config.Timezone)); err == nil {
			return loc
		}
	}
	return time.Local
}

func timeoutFromConfig(seconds int) time.Duration {
	if seconds <= 0 {
		seconds = 30
	}
	return time.Duration(seconds) * time.Second
}

func newHTTPClient(cfg Config) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if !cfg.UseEnvProxy {
		transport.Proxy = nil
	}

	return &http.Client{
		Timeout:   timeoutFromConfig(cfg.RequestTimeoutSeconds),
		Transport: transport,
	}
}

func normalizeLedgerID(value *int, c *gin.Context) int {
	ledgerID := 1
	if value != nil {
		if *value <= 0 {
			err := errors.New("ledger_id must be positive")
			logging.AbortWithError(c, http.StatusBadRequest, err.Error(), err)
			return 0
		}
		ledgerID = *value
	}
	return ledgerID
}

func truncateForLog(value string, maxRunes int) string {
	if maxRunes <= 0 {
		return value
	}

	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}

	return string(runes[:maxRunes]) + "...(truncated)"
}

func transactionDraftSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"kind": map[string]any{
				"type": "string",
				"enum": []string{"income", "expense"},
			},
			"occurred_on": map[string]any{
				"type": "string",
			},
			"account_name": map[string]any{
				"type": "string",
			},
			"category_name": map[string]any{
				"type": "string",
			},
			"amount": map[string]any{
				"type": "number",
			},
			"description": map[string]any{
				"type": "string",
			},
			"note": map[string]any{
				"type": "string",
			},
			"kind_confidence": map[string]any{
				"type": "number",
			},
			"occurred_on_confidence": map[string]any{
				"type": "number",
			},
			"account_confidence": map[string]any{
				"type": "number",
			},
			"category_confidence": map[string]any{
				"type": "number",
			},
			"amount_confidence": map[string]any{
				"type": "number",
			},
			"description_confidence": map[string]any{
				"type": "number",
			},
			"note_confidence": map[string]any{
				"type": "number",
			},
		},
		"required": []string{
			"kind",
			"occurred_on",
			"account_name",
			"category_name",
			"amount",
			"description",
			"note",
			"kind_confidence",
			"occurred_on_confidence",
			"account_confidence",
			"category_confidence",
			"amount_confidence",
			"description_confidence",
			"note_confidence",
		},
	}
}

type modelCallResult struct {
	Draft            modelTransactionDraft
	StatusCode       int
	ResponseText     string
	ResponseBodyText string
	Usage            *openAIUsage
}

func (h *Handler) executeModelCall(ctx context.Context, endpoint string, body []byte) (modelCallResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return modelCallResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(h.config.OpenAIAPIKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return modelCallResult{}, err
	}
	defer resp.Body.Close()

	result := modelCallResult{StatusCode: resp.StatusCode}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		result.ResponseBodyText = strings.TrimSpace(string(responseBody))
		return result, err
	}
	result.ResponseBodyText = strings.TrimSpace(string(responseBody))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var openAIErr openAIChatCompletionResponse
		if err := json.Unmarshal(responseBody, &openAIErr); err == nil && openAIErr.Error != nil {
			if strings.TrimSpace(openAIErr.Error.Message) != "" {
				return result, fmt.Errorf("openai api error: status=%s type=%s message=%s", resp.Status, openAIErr.Error.Type, openAIErr.Error.Message)
			}
		}
		if snippet := strings.TrimSpace(string(responseBody)); snippet != "" {
			return result, fmt.Errorf("openai api returned status %s: %s", resp.Status, truncateForLog(snippet, 1024))
		}
		return result, fmt.Errorf("openai api returned status %s", resp.Status)
	}

	var completion openAIChatCompletionResponse
	if err := json.Unmarshal(responseBody, &completion); err != nil {
		return result, err
	}
	result.Usage = completion.Usage
	if len(completion.Choices) == 0 {
		return result, errors.New("openai api returned no choices")
	}

	content := strings.TrimSpace(completion.Choices[0].Message.Content)
	result.ResponseText = content
	if content == "" {
		return result, errors.New("openai api returned empty content")
	}

	if err := json.Unmarshal([]byte(content), &result.Draft); err != nil {
		return result, fmt.Errorf("failed to parse model response: %w", err)
	}

	return result, nil
}

func shouldRetryModelCall(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	if ctx != nil && ctx.Err() != nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	message := strings.ToLower(strings.TrimSpace(err.Error()))
	for _, token := range []string{
		"connection reset by peer",
		"broken pipe",
		"server closed idle connection",
		"http2: client connection lost",
		"stream error",
	} {
		if strings.Contains(message, token) {
			return true
		}
	}

	return false
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

var errAIUnavailable = errors.New("OPENAI_API_KEY is not configured")
