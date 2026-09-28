package loops

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const createTransactionalResponse = `{
	"id": "tx_new",
	"url": "https://app.loops.so/transactional/tx_new",
	"name": "Welcome email",
	"draftEmailMessageId": "em_draft",
	"draftEmailMessageContentRevisionId": "rev_1",
	"publishedEmailMessageId": null,
	"createdAt": "2026-05-01T10:00:00Z",
	"updatedAt": "2026-05-01T10:00:00Z",
	"dataVariables": []
}`

func TestCreateTransactional(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantAPIErr *APIError
		wantErrMsg string
	}{
		{
			name:       "success",
			statusCode: http.StatusCreated,
			body:       createTransactionalResponse,
		},
		{
			name:       "bad request",
			statusCode: http.StatusBadRequest,
			body:       `{"success":false,"message":"name is required"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusBadRequest, Message: "name is required"},
		},
		{
			name:       "unauthorized",
			statusCode: http.StatusUnauthorized,
			body:       `{"success":false,"error":"Invalid API key"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusUnauthorized, Message: "Invalid API key"},
		},
		{
			name:       "invalid json",
			statusCode: http.StatusCreated,
			body:       `not json`,
			wantErrMsg: "failed to decode response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			resp, err := client.CreateTransactional(CreateTransactionalRequest{Name: "Welcome email"})

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
			if resp.ID != "tx_new" {
				t.Errorf("ID = %q, want tx_new", resp.ID)
			}
			if resp.URL != "https://app.loops.so/transactional/tx_new" {
				t.Errorf("URL = %q", resp.URL)
			}
			if resp.DraftEmailMessageID == nil || *resp.DraftEmailMessageID != "em_draft" {
				t.Errorf("DraftEmailMessageID = %v, want em_draft", resp.DraftEmailMessageID)
			}
			if resp.DraftEmailMessageContentRevisionID == nil || *resp.DraftEmailMessageContentRevisionID != "rev_1" {
				t.Errorf("DraftEmailMessageContentRevisionID = %v, want rev_1", resp.DraftEmailMessageContentRevisionID)
			}
			if resp.PublishedEmailMessageID != nil {
				t.Errorf("expected nil PublishedEmailMessageID, got %v", *resp.PublishedEmailMessageID)
			}
		})
	}
}

func TestCreateTransactional_RequestBody(t *testing.T) {
	var (
		gotPath   string
		gotMethod string
		body      map[string]any
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &body)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(createTransactionalResponse))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	if _, err := client.CreateTransactional(CreateTransactionalRequest{Name: "Welcome email"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/transactional-emails" {
		t.Errorf("path = %q, want /transactional-emails", gotPath)
	}
	if body["name"] != "Welcome email" {
		t.Errorf("name = %v, want Welcome email", body["name"])
	}
}

func TestCreateTransactional_WithGroup(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &body)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(createTransactionalResponse))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	req := CreateTransactionalRequest{Name: "Welcome email", TransactionalGroupID: "grp_welcome"}
	if _, err := client.CreateTransactional(req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body["transactionalGroupId"] != "grp_welcome" {
		t.Errorf("transactionalGroupId = %v, want grp_welcome", body["transactionalGroupId"])
	}
}

const getTransactionalResponse = `{
	"id": "tx_abc123",
	"url": "https://app.loops.so/transactional/tx_abc123",
	"name": "Welcome email",
	"draftEmailMessageId": "em_draft",
	"publishedEmailMessageId": "em_published",
	"createdAt": "2026-05-01T10:00:00Z",
	"updatedAt": "2026-05-02T10:00:00Z",
	"dataVariables": ["firstName", "url"]
}`

func TestGetTransactional(t *testing.T) {
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
			id:         "tx_abc123",
			statusCode: http.StatusOK,
			body:       getTransactionalResponse,
			wantID:     "tx_abc123",
		},
		{
			name:       "not found",
			id:         "tx_missing",
			statusCode: http.StatusNotFound,
			body:       `{"success":false,"message":"Transactional email not found"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusNotFound, Message: "Transactional email not found"},
		},
		{
			name:       "invalid id",
			id:         "bad",
			statusCode: http.StatusBadRequest,
			body:       `{"success":false,"message":"Invalid transactionalId"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusBadRequest, Message: "Invalid transactionalId"},
		},
		{
			name:       "invalid json",
			id:         "tx_abc123",
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
			result, err := client.GetTransactional(tt.id)

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
			if want := "/transactional-emails/" + tt.id; gotPath != want {
				t.Errorf("path = %q, want %q", gotPath, want)
			}
			if result.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", result.ID, tt.wantID)
			}
			if result.URL != "https://app.loops.so/transactional/tx_abc123" {
				t.Errorf("URL = %q", result.URL)
			}
			if result.PublishedEmailMessageID == nil || *result.PublishedEmailMessageID != "em_published" {
				t.Errorf("PublishedEmailMessageID = %v, want em_published", result.PublishedEmailMessageID)
			}
			if len(result.DataVariables) != 2 {
				t.Errorf("len(DataVariables) = %d, want 2", len(result.DataVariables))
			}
		})
	}
}

const updateTransactionalResponse = `{
	"id": "tx_abc123",
	"name": "Renamed",
	"draftEmailMessageId": "em_draft",
	"publishedEmailMessageId": null,
	"createdAt": "2026-05-01T10:00:00Z",
	"updatedAt": "2026-05-03T10:00:00Z",
	"dataVariables": []
}`

func TestUpdateTransactional(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantAPIErr *APIError
		wantErrMsg string
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
			body:       updateTransactionalResponse,
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			body:       `{"success":false,"message":"Transactional email not found"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusNotFound, Message: "Transactional email not found"},
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
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			result, err := client.UpdateTransactional("tx_abc123", UpdateTransactionalRequest{Name: "Renamed"})

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
			if result.ID != "tx_abc123" {
				t.Errorf("ID = %q, want tx_abc123", result.ID)
			}
			if result.Name != "Renamed" {
				t.Errorf("Name = %q, want Renamed", result.Name)
			}
		})
	}
}

func TestUpdateTransactional_RequestBodyAndPath(t *testing.T) {
	var (
		gotPath   string
		gotMethod string
		body      map[string]any
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(updateTransactionalResponse))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	if _, err := client.UpdateTransactional("tx_abc123", UpdateTransactionalRequest{Name: "Renamed"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/transactional-emails/tx_abc123" {
		t.Errorf("path = %q, want /transactional-emails/tx_abc123", gotPath)
	}
	if body["name"] != "Renamed" {
		t.Errorf("name = %v, want Renamed", body["name"])
	}
}

func TestEnsureTransactionalDraft(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantAPIErr *APIError
		wantErrMsg string
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
			body:       createTransactionalResponse,
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			body:       `{"success":false,"message":"Transactional email not found"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusNotFound, Message: "Transactional email not found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath, gotMethod string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotMethod = r.Method
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			result, err := client.EnsureTransactionalDraft("tx_abc123")

			if tt.wantAPIErr != nil {
				var apiErr *APIError
				if !errors.As(err, &apiErr) {
					t.Fatalf("expected *APIError, got %T: %v", err, err)
				}
				if apiErr.StatusCode != tt.wantAPIErr.StatusCode {
					t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, tt.wantAPIErr.StatusCode)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotMethod != http.MethodPost {
				t.Errorf("method = %q, want POST", gotMethod)
			}
			if gotPath != "/transactional-emails/tx_abc123/draft" {
				t.Errorf("path = %q, want /transactional-emails/tx_abc123/draft", gotPath)
			}
			if result.DraftEmailMessageContentRevisionID == nil {
				t.Errorf("expected DraftEmailMessageContentRevisionID, got nil")
			}
		})
	}
}

func TestPublishTransactional(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantAPIErr *APIError
		wantErrMsg string
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
			body:       getTransactionalResponse,
		},
		{
			name:       "no draft",
			statusCode: http.StatusConflict,
			body:       `{"success":false,"message":"No draft to publish"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusConflict, Message: "No draft to publish"},
		},
		{
			name:       "validation failed",
			statusCode: http.StatusUnprocessableEntity,
			body:       `{"success":false,"message":"Draft failed validation"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusUnprocessableEntity, Message: "Draft failed validation"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath, gotMethod string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotMethod = r.Method
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			result, err := client.PublishTransactional("tx_abc123")

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
			if gotMethod != http.MethodPost {
				t.Errorf("method = %q, want POST", gotMethod)
			}
			if gotPath != "/transactional-emails/tx_abc123/publish" {
				t.Errorf("path = %q, want /transactional-emails/tx_abc123/publish", gotPath)
			}
			if result.ID != "tx_abc123" {
				t.Errorf("ID = %q, want tx_abc123", result.ID)
			}
		})
	}
}

const listTransactionalsResponse = `{
	"pagination": {
		"totalResults": 2,
		"returnedResults": 2,
		"perPage": 20,
		"totalPages": 1,
		"nextCursor": null,
		"nextPage": null
	},
	"data": [
		{
			"id": "tx_1",
			"url": "https://app.loops.so/transactional/tx_1",
			"name": "Welcome",
			"draftEmailMessageId": "em_d1",
			"publishedEmailMessageId": "em_p1",
			"createdAt": "2026-05-01T10:00:00Z",
			"updatedAt": "2026-05-02T10:00:00Z",
			"dataVariables": ["firstName"]
		},
		{
			"id": "tx_2",
			"name": "Reset password",
			"draftEmailMessageId": null,
			"publishedEmailMessageId": "em_p2",
			"createdAt": "2026-04-01T10:00:00Z",
			"updatedAt": "2026-04-15T10:00:00Z",
			"dataVariables": ["url"]
		}
	]
}`

func TestListTransactionals(t *testing.T) {
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
			body:       listTransactionalsResponse,
			wantCount:  2,
		},
		{
			name:       "empty list",
			statusCode: http.StatusOK,
			body:       `{"pagination":{"totalResults":0},"data":[]}`,
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
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			items, pagination, err := client.ListTransactionals(PaginationParams{})

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
			if len(items) != tt.wantCount {
				t.Errorf("len(items) = %d, want %d", len(items), tt.wantCount)
			}
			if pagination == nil {
				t.Fatal("expected pagination, got nil")
			}
		})
	}
}

func TestListTransactionals_ResponseData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(listTransactionalsResponse))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	items, _, err := client.ListTransactionals(PaginationParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if items[0].ID != "tx_1" {
		t.Errorf("ID = %q, want tx_1", items[0].ID)
	}
	if items[0].URL != "https://app.loops.so/transactional/tx_1" {
		t.Errorf("URL = %q", items[0].URL)
	}
	if items[0].DraftEmailMessageID == nil || *items[0].DraftEmailMessageID != "em_d1" {
		t.Errorf("DraftEmailMessageID = %v, want em_d1", items[0].DraftEmailMessageID)
	}
	if items[1].DraftEmailMessageID != nil {
		t.Errorf("expected nil DraftEmailMessageID, got %v", items[1].DraftEmailMessageID)
	}
	if items[1].PublishedEmailMessageID == nil || *items[1].PublishedEmailMessageID != "em_p2" {
		t.Errorf("PublishedEmailMessageID = %v, want em_p2", items[1].PublishedEmailMessageID)
	}
}

func TestListTransactionals_QueryParams(t *testing.T) {
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
			var gotPath, gotPerPage, gotCursor string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotPerPage = r.URL.Query().Get("perPage")
				gotCursor = r.URL.Query().Get("cursor")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"pagination":{},"data":[]}`))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			client.ListTransactionals(tt.params)

			if gotPath != "/transactional-emails" {
				t.Errorf("path = %q, want /transactional-emails", gotPath)
			}
			if gotPerPage != tt.wantPerPage {
				t.Errorf("perPage = %q, want %q", gotPerPage, tt.wantPerPage)
			}
			if gotCursor != tt.wantCursor {
				t.Errorf("cursor = %q, want %q", gotCursor, tt.wantCursor)
			}
		})
	}
}

func TestGetTransactionalMetrics(t *testing.T) {
	want := TransactionalMetrics{
		Sends:       4210,
		Deliveries:  4175,
		SpamReports: 1,
		HardBounces: 9,
		SoftBounces: 13,
	}

	tests := []struct {
		name       string
		id         string
		statusCode int
		body       string
		wantAPIErr *APIError
		wantErrMsg string
	}{
		{
			name:       "success",
			id:         "tx_abc123",
			statusCode: http.StatusOK,
			body:       `{"sends":4210,"deliveries":4175,"spamReports":1,"hardBounces":9,"softBounces":13}`,
		},
		{
			name:       "not found",
			id:         "tx_missing",
			statusCode: http.StatusNotFound,
			body:       `{"message":"Transactional email not found."}`,
			wantAPIErr: &APIError{StatusCode: http.StatusNotFound, Message: "Transactional email not found."},
		},
		{
			name:       "invalid id",
			id:         "bad",
			statusCode: http.StatusBadRequest,
			body:       `{"message":"Invalid transactionalId"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusBadRequest, Message: "Invalid transactionalId"},
		},
		{
			name:       "invalid json",
			id:         "tx_abc123",
			statusCode: http.StatusOK,
			body:       `not json`,
			wantErrMsg: "failed to decode response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			result, err := client.GetTransactionalMetrics(tt.id)

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
			if gotMethod != http.MethodGet {
				t.Errorf("method = %q, want GET", gotMethod)
			}
			if wantPath := "/transactional-emails/" + tt.id + "/metrics"; gotPath != wantPath {
				t.Errorf("path = %q, want %q", gotPath, wantPath)
			}
			if *result != want {
				t.Errorf("metrics = %+v, want %+v", *result, want)
			}
		})
	}
}
