package loops

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const getEventPatternResponse = `{
	"success": true,
	"id": "evp_abc123",
	"eventName": "purchase.completed",
	"eventProperties": [
		{"name": "amount", "type": "number"},
		{"name": "currency", "type": "string"}
	],
	"incomingWebhookPlatform": "stripe"
}`

const getCustomEventPatternResponse = `{
	"success": true,
	"id": "evp_custom",
	"eventName": "signed up",
	"eventProperties": [],
	"incomingWebhookPlatform": null
}`

const listEventPatternsResponse = `{
	"success": true,
	"pagination": {
		"totalResults": 2,
		"returnedResults": 2,
		"perPage": 20,
		"totalPages": 1,
		"nextCursor": "",
		"nextPage": ""
	},
	"data": [
		{
			"id": "evp_1",
			"eventName": "purchase.completed",
			"incomingWebhookPlatform": "stripe"
		},
		{
			"id": "evp_2",
			"eventName": "signed up",
			"incomingWebhookPlatform": null
		}
	]
}`

func TestGetEventPattern(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		statusCode int
		body       string
		wantAPIErr *APIError
		wantErrMsg string
		wantID     string
	}{
		{
			name:       "success",
			id:         "evp_abc123",
			statusCode: http.StatusOK,
			body:       getEventPatternResponse,
			wantID:     "evp_abc123",
		},
		{
			name:       "not found",
			id:         "evp_missing",
			statusCode: http.StatusNotFound,
			body:       `{"success":false,"message":"Event pattern not found"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusNotFound, Message: "Event pattern not found"},
		},
		{
			name:       "invalid json",
			id:         "evp_abc123",
			statusCode: http.StatusOK,
			body:       `not json`,
			wantErrMsg: "failed to decode response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			result, err := client.GetEventPattern(context.Background(), tt.id)

			if tt.wantAPIErr != nil {
				var apiErr *APIError
				if !errors.As(err, &apiErr) {
					t.Fatalf("expected *APIError, got %T: %v", err, err)
				}
				if apiErr.StatusCode != tt.wantAPIErr.StatusCode {
					t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, tt.wantAPIErr.StatusCode)
				}
				if apiErr.Message != tt.wantAPIErr.Message {
					t.Errorf("Message = %q, want %q", apiErr.Message, tt.wantAPIErr.Message)
				}
				return
			}

			if tt.wantErrMsg != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrMsg)
				}
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("error = %q, want it to contain %q", err.Error(), tt.wantErrMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if want := "/event-patterns/" + tt.id; gotPath != want {
				t.Errorf("path = %q, want %q", gotPath, want)
			}
			if result.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", result.ID, tt.wantID)
			}
			if result.EventName != "purchase.completed" {
				t.Errorf("EventName = %q, want purchase.completed", result.EventName)
			}
			if result.IncomingWebhookPlatform == nil || *result.IncomingWebhookPlatform != "stripe" {
				t.Errorf("IncomingWebhookPlatform = %v, want stripe", result.IncomingWebhookPlatform)
			}
			if len(result.EventProperties) != 2 {
				t.Fatalf("len(EventProperties) = %d, want 2", len(result.EventProperties))
			}
			if result.EventProperties[0].Name != "amount" || result.EventProperties[0].Type != "number" {
				t.Errorf("EventProperties[0] = %+v, want {amount number}", result.EventProperties[0])
			}
		})
	}
}

func TestGetEventPattern_CustomEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(getCustomEventPatternResponse))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	result, err := client.GetEventPattern(context.Background(), "evp_custom")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IncomingWebhookPlatform != nil {
		t.Errorf("IncomingWebhookPlatform = %v, want nil", *result.IncomingWebhookPlatform)
	}
	if len(result.EventProperties) != 0 {
		t.Errorf("len(EventProperties) = %d, want 0", len(result.EventProperties))
	}
}

func TestGetEventPatternByName(t *testing.T) {
	tests := []struct {
		name       string
		eventName  string
		wantPath   string
		statusCode int
		body       string
		wantAPIErr *APIError
	}{
		{
			name:       "success",
			eventName:  "purchase.completed",
			wantPath:   "/event-patterns/by-name/purchase.completed",
			statusCode: http.StatusOK,
			body:       getEventPatternResponse,
		},
		{
			name:       "name needing url escaping",
			eventName:  "signed up/v2",
			wantPath:   "/event-patterns/by-name/signed up/v2",
			statusCode: http.StatusOK,
			body:       getEventPatternResponse,
		},
		{
			name:       "not found",
			eventName:  "missing.event",
			statusCode: http.StatusNotFound,
			body:       `{"success":false,"message":"Event pattern not found"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusNotFound, Message: "Event pattern not found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath, gotRawPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotRawPath = r.URL.EscapedPath()
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			result, err := client.GetEventPatternByName(context.Background(), tt.eventName)

			if tt.wantAPIErr != nil {
				var apiErr *APIError
				if !errors.As(err, &apiErr) {
					t.Fatalf("expected *APIError, got %T: %v", err, err)
				}
				if apiErr.StatusCode != tt.wantAPIErr.StatusCode {
					t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, tt.wantAPIErr.StatusCode)
				}
				if apiErr.Message != tt.wantAPIErr.Message {
					t.Errorf("Message = %q, want %q", apiErr.Message, tt.wantAPIErr.Message)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotPath != tt.wantPath {
				t.Errorf("path = %q, want %q", gotPath, tt.wantPath)
			}
			if strings.Contains(tt.eventName, " ") && !strings.Contains(gotRawPath, "%20") {
				t.Errorf("escaped path = %q, want it to contain %%20", gotRawPath)
			}
			if result.ID != "evp_abc123" {
				t.Errorf("ID = %q, want evp_abc123", result.ID)
			}
		})
	}
}

func TestListEventPatterns(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantAPIErr *APIError
		wantErrMsg string
		wantCount  int
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
			body:       listEventPatternsResponse,
			wantCount:  2,
		},
		{
			name:       "empty list",
			statusCode: http.StatusOK,
			body:       `{"success":true,"pagination":{"totalResults":0},"data":[]}`,
			wantCount:  0,
		},
		{
			name:       "unauthorized",
			statusCode: http.StatusUnauthorized,
			body:       `{"success":false,"error":"Invalid API key"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusUnauthorized, Message: "Invalid API key"},
		},
		{
			name:       "invalid json",
			statusCode: http.StatusOK,
			body:       `not json`,
			wantErrMsg: "failed to decode response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			patterns, pagination, err := client.ListEventPatterns(context.Background(), PaginationParams{})

			if tt.wantAPIErr != nil {
				var apiErr *APIError
				if !errors.As(err, &apiErr) {
					t.Fatalf("expected *APIError, got %T: %v", err, err)
				}
				if apiErr.StatusCode != tt.wantAPIErr.StatusCode {
					t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, tt.wantAPIErr.StatusCode)
				}
				if tt.wantAPIErr.Message != "" && apiErr.Message != tt.wantAPIErr.Message {
					t.Errorf("Message = %q, want %q", apiErr.Message, tt.wantAPIErr.Message)
				}
				return
			}

			if tt.wantErrMsg != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrMsg)
				}
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("error = %q, want it to contain %q", err.Error(), tt.wantErrMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotPath != "/event-patterns" {
				t.Errorf("path = %q, want /event-patterns", gotPath)
			}
			if len(patterns) != tt.wantCount {
				t.Errorf("len(patterns) = %d, want %d", len(patterns), tt.wantCount)
			}
			if pagination == nil {
				t.Fatal("expected pagination, got nil")
			}
		})
	}
}

func TestListEventPatterns_ResponseData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(listEventPatternsResponse))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	patterns, _, err := client.ListEventPatterns(context.Background(), PaginationParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if patterns[0].ID != "evp_1" {
		t.Errorf("ID = %q, want evp_1", patterns[0].ID)
	}
	if patterns[0].IncomingWebhookPlatform == nil || *patterns[0].IncomingWebhookPlatform != "stripe" {
		t.Errorf("patterns[0].IncomingWebhookPlatform = %v, want stripe", patterns[0].IncomingWebhookPlatform)
	}
	if patterns[1].IncomingWebhookPlatform != nil {
		t.Errorf("patterns[1].IncomingWebhookPlatform = %v, want nil", *patterns[1].IncomingWebhookPlatform)
	}
	if patterns[1].EventName != "signed up" {
		t.Errorf("patterns[1].EventName = %q, want signed up", patterns[1].EventName)
	}
}

func TestListEventPatterns_QueryParams(t *testing.T) {
	tests := []struct {
		name        string
		params      PaginationParams
		wantPerPage string
		wantCursor  string
	}{
		{
			name:   "no params",
			params: PaginationParams{},
		},
		{
			name:        "both params",
			params:      PaginationParams{PerPage: "10", Cursor: "xyz"},
			wantPerPage: "10",
			wantCursor:  "xyz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPerPage, gotCursor string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPerPage = r.URL.Query().Get("perPage")
				gotCursor = r.URL.Query().Get("cursor")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"pagination":{},"data":[]}`))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			client.ListEventPatterns(context.Background(), tt.params)

			if gotPerPage != tt.wantPerPage {
				t.Errorf("perPage = %q, want %q", gotPerPage, tt.wantPerPage)
			}
			if gotCursor != tt.wantCursor {
				t.Errorf("cursor = %q, want %q", gotCursor, tt.wantCursor)
			}
		})
	}
}
