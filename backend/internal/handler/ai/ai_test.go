package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestCallModelRetriesTransientEOF(t *testing.T) {
	t.Parallel()

	responseBody := buildChatCompletionBody(t, modelTransactionDraft{
		Kind:                  "expense",
		OccurredOn:            "2026-04-22",
		AccountName:           "微信 [cash]",
		CategoryName:          "餐饮 / 三餐 (expense)",
		Amount:                12.5,
		Description:           "早餐",
		Note:                  "",
		KindConfidence:        0.99,
		OccurredOnConfidence:  0.99,
		AccountConfidence:     0.95,
		CategoryConfidence:    0.95,
		AmountConfidence:      0.99,
		DescriptionConfidence: 0.90,
		NoteConfidence:        0.80,
	})

	attempts := 0
	handler := &Handler{
		client: &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				attempts++
				if req.URL.String() != "https://api.deepseek.com/chat/completions" {
					t.Fatalf("expected DeepSeek official endpoint, got %s", req.URL.String())
				}
				if got := req.Header.Get("Authorization"); got != "Bearer test-key" {
					t.Fatalf("expected DeepSeek bearer token, got %q", got)
				}

				var requestBody deepSeekChatCompletionRequest
				if err := json.NewDecoder(req.Body).Decode(&requestBody); err != nil {
					t.Fatalf("decode model request: %v", err)
				}
				if requestBody.Model != "deepseek-v4-flash" {
					t.Fatalf("expected deepseek-v4-flash model, got %q", requestBody.Model)
				}
				if requestBody.ResponseFormat == nil || requestBody.ResponseFormat.Type != "json_object" {
					t.Fatalf("expected json_object response format, got %#v", requestBody.ResponseFormat)
				}
				if requestBody.Thinking == nil || requestBody.Thinking.Type != "disabled" {
					t.Fatalf("expected disabled thinking mode, got %#v", requestBody.Thinking)
				}

				if attempts == 1 {
					return nil, io.EOF
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(responseBody)),
					Header:     make(http.Header),
				}, nil
			}),
		},
		config: Config{
			DeepSeekAPIKey:  "test-key",
			DeepSeekBaseURL: "https://api.deepseek.com",
			DeepSeekModel:   "deepseek-v4-flash",
		},
	}

	draft, err := handler.callModel(context.Background(), nil, aiPromptPayload{
		SourceText: "早餐 12.5",
		Today:      "2026-04-22",
		Timezone:   "Asia/Shanghai",
	})
	if err != nil {
		t.Fatalf("callModel returned error: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
	if draft.Amount != 12.5 {
		t.Fatalf("expected amount 12.5, got %v", draft.Amount)
	}
	if draft.Description != "早餐" {
		t.Fatalf("expected description 早餐, got %q", draft.Description)
	}
}

func TestFinalizeDraftMatchesCategoryPathAndGlobalConfidence(t *testing.T) {
	t.Parallel()

	handler := &Handler{}
	categoryID := 33
	draft, err := handler.finalizeDraft(
		modelTransactionDraft{
			Kind:         "expense",
			OccurredOn:   "2026-05-06",
			AccountName:  "平安 [cash]",
			CategoryName: "食品餐饮 / 外卖 / 工作日外卖",
			Amount:       25.7,
			Description:  "外卖",
			Confidence:   0.99,
		},
		[]accountOption{
			{ID: 1, Name: "平安", Type: "cash", Label: "平安 [cash]"},
		},
		[]categoryOption{
			{
				ID:    categoryID,
				Name:  "工作日外卖",
				Kind:  "expense",
				Path:  "食品餐饮 / 外卖 / 工作日外卖",
				Label: "食品餐饮 / 外卖 / 工作日外卖 (expense)",
			},
		},
	)
	if err != nil {
		t.Fatalf("finalizeDraft returned error: %v", err)
	}
	if draft.CategoryID == nil || *draft.CategoryID != categoryID {
		t.Fatalf("expected category id %d, got %v", categoryID, draft.CategoryID)
	}
	if len(draft.MissingFields) != 0 {
		t.Fatalf("expected no missing fields, got %v", draft.MissingFields)
	}
	if len(draft.LowConfidenceFields) != 0 {
		t.Fatalf("expected global confidence to satisfy confidence fields, got %v", draft.LowConfidenceFields)
	}
}

func TestFinalizeDraftDefaultsMissingAccountToPingAn(t *testing.T) {
	t.Parallel()

	handler := &Handler{}
	categoryID := 72
	draft, err := handler.finalizeDraft(
		modelTransactionDraft{
			Kind:         "expense",
			OccurredOn:   "2026-05-04",
			AccountName:  "",
			CategoryName: "交通出行 / 打车与代驾 (expense)",
			Amount:       12.5,
			Description:  "打车",
		},
		[]accountOption{
			{ID: 1, Name: "平安", Type: "cash", Label: "平安 [cash]"},
			{ID: 2, Name: "中国银行", Type: "cash", Label: "中国银行 [cash]"},
		},
		[]categoryOption{
			{
				ID:    categoryID,
				Name:  "打车与代驾",
				Kind:  "expense",
				Path:  "交通出行 / 打车与代驾",
				Label: "交通出行 / 打车与代驾 (expense)",
			},
		},
	)
	if err != nil {
		t.Fatalf("finalizeDraft returned error: %v", err)
	}
	if draft.AccountID == nil || *draft.AccountID != 1 {
		t.Fatalf("expected default account id 1, got %v", draft.AccountID)
	}
	if draft.AccountName != "平安" {
		t.Fatalf("expected default account name 平安, got %q", draft.AccountName)
	}
	if containsString(draft.MissingFields, "account") {
		t.Fatalf("expected account not to be missing, got %v", draft.MissingFields)
	}
	if containsString(draft.LowConfidenceFields, "account") {
		t.Fatalf("expected default account not to be low confidence, got %v", draft.LowConfidenceFields)
	}
}

func TestFinalizeDraftMatchesPingAnBankAlias(t *testing.T) {
	t.Parallel()

	for _, accountName := range []string{"平安银行", "平安银行 [cash]"} {
		t.Run(accountName, func(t *testing.T) {
			handler := &Handler{}
			categoryID := 72
			draft, err := handler.finalizeDraft(
				modelTransactionDraft{
					Kind:              "expense",
					OccurredOn:        "2026-05-04",
					AccountName:       accountName,
					CategoryName:      "交通出行 / 打车与代驾 (expense)",
					Amount:            12.5,
					Description:       "打车",
					AccountConfidence: 0.95,
				},
				[]accountOption{
					{ID: 1, Name: "平安", Type: "cash", Label: "平安 [cash]"},
				},
				[]categoryOption{
					{
						ID:    categoryID,
						Name:  "打车与代驾",
						Kind:  "expense",
						Path:  "交通出行 / 打车与代驾",
						Label: "交通出行 / 打车与代驾 (expense)",
					},
				},
			)
			if err != nil {
				t.Fatalf("finalizeDraft returned error: %v", err)
			}
			if draft.AccountID == nil || *draft.AccountID != 1 {
				t.Fatalf("expected account id 1, got %v", draft.AccountID)
			}
			if draft.AccountName != "平安" {
				t.Fatalf("expected account name 平安, got %q", draft.AccountName)
			}
			if containsString(draft.MissingFields, "account") {
				t.Fatalf("expected account not to be missing, got %v", draft.MissingFields)
			}
		})
	}
}

func TestNewHTTPClientDisablesProxyWhenConfigured(t *testing.T) {
	t.Parallel()

	client := newHTTPClient(Config{
		RequestTimeoutSeconds: 5,
		UseEnvProxy:           false,
	})

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", client.Transport)
	}
	if transport.Proxy != nil {
		t.Fatalf("expected proxy lookup to be disabled")
	}
}

func buildChatCompletionBody(t *testing.T, draft modelTransactionDraft) string {
	t.Helper()

	contentBytes, err := json.Marshal(draft)
	if err != nil {
		t.Fatalf("marshal draft: %v", err)
	}

	payload := deepSeekChatCompletionResponse{
		Choices: []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}{
			{
				Message: struct {
					Content string `json:"content"`
				}{
					Content: string(contentBytes),
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	return string(bodyBytes)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
