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
			OpenAIAPIKey:  "test-key",
			OpenAIBaseURL: "https://api.linkapi.ai/v1",
			OpenAIModel:   "gpt-4.1-mini",
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

	payload := openAIChatCompletionResponse{
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
