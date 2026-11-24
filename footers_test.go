package loops

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const footerVariantResponse = `{
	"id": "var_1",
	"footerId": "ftr_1",
	"contentRevisionId": "rev_1",
	"draftLmx": "<Paragraph>Draft</Paragraph>",
	"publishedLmx": "<Paragraph>Published</Paragraph>",
	"publishedAt": "2026-11-02T08:30:00.000Z",
	"createdAt": "2026-10-27T10:00:00.000Z",
	"updatedAt": "2026-11-02T08:30:00.000Z"
}`

const unpublishedFooterVariantResponse = `{
	"id": "var_1",
	"footerId": "ftr_1",
	"contentRevisionId": "rev_1",
	"draftLmx": "<Paragraph>Draft</Paragraph>",
	"publishedLmx": null,
	"publishedAt": null,
	"createdAt": "2026-10-27T10:00:00.000Z",
	"updatedAt": "2026-10-27T10:00:00.000Z"
}`

const footerWithVariantsResponse = `{
	"id": "ftr_1",
	"name": "Main footer",
	"isDefault": true,
	"placement": "inside",
	"createdAt": "2026-10-27T10:00:00.000Z",
	"updatedAt": "2026-11-02T08:30:00.000Z",
	"variants": [` + footerVariantResponse + `]
}`

const setDefaultFooterResponse = `{
	"id": "ftr_1",
	"name": "Main footer",
	"isDefault": true,
	"placement": "inside",
	"createdAt": "2026-10-27T10:00:00.000Z",
	"updatedAt": "2026-11-02T08:30:00.000Z",
	"updatedEmailCount": 12,
	"variants": [` + footerVariantResponse + `]
}`

const listFootersResponse = `{
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
			"id": "ftr_1",
			"name": "Main footer",
			"isDefault": true,
			"placement": "inside",
			"createdAt": "2026-10-27T10:00:00.000Z",
			"updatedAt": "2026-11-02T08:30:00.000Z"
		},
		{
			"id": "ftr_2",
			"name": "Marketing footer",
			"isDefault": false,
			"placement": "below",
			"createdAt": "2026-10-20T10:00:00.000Z",
			"updatedAt": "2026-10-21T10:00:00.000Z"
		}
	]
}`

func TestListFooters(t *testing.T) {
	tests := []struct {
		name       string
		params     PaginationParams
		statusCode int
		body       string
		wantAPIErr *APIError
		wantErrMsg string
		wantQuery  string
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
			body:       listFootersResponse,
		},
		{
			name:       "with pagination params",
			params:     PaginationParams{PerPage: 20, Cursor: "cur_1"},
			statusCode: http.StatusOK,
			body:       listFootersResponse,
			wantQuery:  "cursor=cur_1&perPage=20",
		},
		{
			name:       "invalid perPage",
			params:     PaginationParams{PerPage: 5},
			statusCode: http.StatusBadRequest,
			body:       `{"message":"perPage must be between 10 and 50."}`,
			wantAPIErr: &APIError{StatusCode: http.StatusBadRequest, Message: "perPage must be between 10 and 50."},
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
			var gotPath, gotMethod, gotQuery string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotMethod = r.Method
				gotQuery = r.URL.RawQuery
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			footers, page, err := client.ListFooters(context.Background(), tt.params)

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
			if gotPath != "/footers" {
				t.Errorf("path = %q, want /footers", gotPath)
			}
			if gotQuery != tt.wantQuery {
				t.Errorf("query = %q, want %q", gotQuery, tt.wantQuery)
			}
			if len(footers) != 2 {
				t.Fatalf("len(footers) = %d, want 2", len(footers))
			}
			want := Footer{
				ID:        "ftr_1",
				Name:      "Main footer",
				IsDefault: true,
				Placement: FooterPlacementInside,
				CreatedAt: "2026-10-27T10:00:00.000Z",
				UpdatedAt: "2026-11-02T08:30:00.000Z",
			}
			if footers[0] != want {
				t.Errorf("footers[0] = %+v, want %+v", footers[0], want)
			}
			if footers[1].ID != "ftr_2" {
				t.Errorf("footers[1].ID = %q, want ftr_2", footers[1].ID)
			}
			if footers[1].IsDefault {
				t.Error("footers[1].IsDefault = true, want false")
			}
			if footers[1].Placement != FooterPlacementBelow {
				t.Errorf("footers[1].Placement = %q, want %q", footers[1].Placement, FooterPlacementBelow)
			}
			if page.TotalResults != 2 {
				t.Errorf("TotalResults = %d, want 2", page.TotalResults)
			}
			if page.NextCursor != "" {
				t.Errorf("NextCursor = %q, want empty", page.NextCursor)
			}
		})
	}
}

func TestCreateFooter(t *testing.T) {
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
			body:       footerWithVariantsResponse,
		},
		{
			name:       "status ok is not success",
			statusCode: http.StatusOK,
			body:       footerWithVariantsResponse,
			wantAPIErr: &APIError{StatusCode: http.StatusOK, Message: "unexpected status: 200"},
		},
		{
			name:       "invalid body",
			statusCode: http.StatusBadRequest,
			body:       `{"message":"name must be a string."}`,
			wantAPIErr: &APIError{StatusCode: http.StatusBadRequest, Message: "name must be a string."},
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
			var gotPath, gotMethod string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotMethod = r.Method
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			result, err := client.CreateFooter(context.Background(), CreateFooterRequest{Name: "Main footer"})

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
			if gotMethod != http.MethodPost {
				t.Errorf("method = %q, want POST", gotMethod)
			}
			if gotPath != "/footers" {
				t.Errorf("path = %q, want /footers", gotPath)
			}
			if result.ID != "ftr_1" {
				t.Errorf("ID = %q, want ftr_1", result.ID)
			}
			if result.Name != "Main footer" {
				t.Errorf("Name = %q, want Main footer", result.Name)
			}
			if len(result.Variants) != 1 || result.Variants[0].ID != "var_1" {
				t.Errorf("Variants = %+v, want one variant var_1", result.Variants)
			}
		})
	}
}

func TestCreateFooter_RequestBody(t *testing.T) {
	tests := []struct {
		name       string
		req        CreateFooterRequest
		wantFields map[string]any
		absent     []string
	}{
		{
			name:       "name sent when set",
			req:        CreateFooterRequest{Name: "Main footer"},
			wantFields: map[string]any{"name": "Main footer"},
		},
		{
			name:   "name omitted when empty",
			req:    CreateFooterRequest{},
			absent: []string{"name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				b, _ := io.ReadAll(r.Body)
				json.Unmarshal(b, &body)
				w.WriteHeader(http.StatusCreated)
				w.Write([]byte(footerWithVariantsResponse))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			if _, err := client.CreateFooter(context.Background(), tt.req); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if body == nil {
				t.Fatal("request body is not a JSON object")
			}
			for k, want := range tt.wantFields {
				if body[k] != want {
					t.Errorf("body[%q] = %v, want %v", k, body[k], want)
				}
			}
			for _, k := range tt.absent {
				if _, present := body[k]; present {
					t.Errorf("body[%q] should not be present, got %v", k, body[k])
				}
			}
		})
	}
}

func TestGetFooter(t *testing.T) {
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
			id:         "ftr_1",
			statusCode: http.StatusOK,
			body:       footerWithVariantsResponse,
		},
		{
			name:       "not found",
			id:         "ftr_missing",
			statusCode: http.StatusNotFound,
			body:       `{"message":"Footer not found."}`,
			wantAPIErr: &APIError{StatusCode: http.StatusNotFound, Message: "Footer not found."},
		},
		{
			name:       "invalid id",
			id:         "bad",
			statusCode: http.StatusBadRequest,
			body:       `{"message":"Invalid footerId."}`,
			wantAPIErr: &APIError{StatusCode: http.StatusBadRequest, Message: "Invalid footerId."},
		},
		{
			name:       "invalid json",
			id:         "ftr_1",
			statusCode: http.StatusOK,
			body:       `not json`,
			wantErrMsg: "failed to decode response",
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
			result, err := client.GetFooter(context.Background(), tt.id)

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
			if want := "/footers/" + tt.id; gotPath != want {
				t.Errorf("path = %q, want %q", gotPath, want)
			}
			want := Footer{
				ID:        "ftr_1",
				Name:      "Main footer",
				IsDefault: true,
				Placement: FooterPlacementInside,
				CreatedAt: "2026-10-27T10:00:00.000Z",
				UpdatedAt: "2026-11-02T08:30:00.000Z",
			}
			if result.Footer != want {
				t.Errorf("Footer = %+v, want %+v", result.Footer, want)
			}
			if len(result.Variants) != 1 {
				t.Fatalf("len(Variants) = %d, want 1", len(result.Variants))
			}
			if result.Variants[0].ID != "var_1" {
				t.Errorf("Variants[0].ID = %q, want var_1", result.Variants[0].ID)
			}
			if result.Variants[0].DraftLMX != "<Paragraph>Draft</Paragraph>" {
				t.Errorf("Variants[0].DraftLMX = %q, want <Paragraph>Draft</Paragraph>", result.Variants[0].DraftLMX)
			}
		})
	}
}

func TestUpdateFooter(t *testing.T) {
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
			body:       footerWithVariantsResponse,
		},
		{
			name:       "empty body",
			statusCode: http.StatusBadRequest,
			body:       `{"message":"At least one field must be provided."}`,
			wantAPIErr: &APIError{StatusCode: http.StatusBadRequest, Message: "At least one field must be provided."},
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			body:       `{"message":"Footer not found."}`,
			wantAPIErr: &APIError{StatusCode: http.StatusNotFound, Message: "Footer not found."},
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
			var gotPath, gotMethod string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotMethod = r.Method
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			result, err := client.UpdateFooter(context.Background(), "ftr_1", UpdateFooterRequest{Name: "Main footer"})

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
			if gotMethod != http.MethodPost {
				t.Errorf("method = %q, want POST", gotMethod)
			}
			if gotPath != "/footers/ftr_1" {
				t.Errorf("path = %q, want /footers/ftr_1", gotPath)
			}
			if result.ID != "ftr_1" {
				t.Errorf("ID = %q, want ftr_1", result.ID)
			}
			if result.Placement != FooterPlacementInside {
				t.Errorf("Placement = %q, want %q", result.Placement, FooterPlacementInside)
			}
			if len(result.Variants) != 1 || result.Variants[0].ID != "var_1" {
				t.Errorf("Variants = %+v, want one variant var_1", result.Variants)
			}
		})
	}
}

func TestUpdateFooter_RequestBody(t *testing.T) {
	tests := []struct {
		name       string
		req        UpdateFooterRequest
		wantFields map[string]any
		absent     []string
	}{
		{
			name:       "name and placement",
			req:        UpdateFooterRequest{Name: "Marketing footer", Placement: FooterPlacementBelow},
			wantFields: map[string]any{"name": "Marketing footer", "placement": "below"},
		},
		{
			name:       "name only",
			req:        UpdateFooterRequest{Name: "Marketing footer"},
			wantFields: map[string]any{"name": "Marketing footer"},
			absent:     []string{"placement"},
		},
		{
			name:       "placement only",
			req:        UpdateFooterRequest{Placement: FooterPlacementInside},
			wantFields: map[string]any{"placement": "inside"},
			absent:     []string{"name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				b, _ := io.ReadAll(r.Body)
				json.Unmarshal(b, &body)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(footerWithVariantsResponse))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			if _, err := client.UpdateFooter(context.Background(), "ftr_1", tt.req); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for k, want := range tt.wantFields {
				if body[k] != want {
					t.Errorf("body[%q] = %v, want %v", k, body[k], want)
				}
			}
			for _, k := range tt.absent {
				if _, present := body[k]; present {
					t.Errorf("body[%q] should not be present, got %v", k, body[k])
				}
			}
		})
	}
}

func TestDeleteFooter(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantAPIErr *APIError
	}{
		{
			name:       "success",
			statusCode: http.StatusNoContent,
		},
		{
			name:       "emails use the footer",
			statusCode: http.StatusConflict,
			body:       `{"message":"3 emails use the footer. Change their footer before you delete it.","emailCount":3}`,
			wantAPIErr: &APIError{StatusCode: http.StatusConflict, Message: "3 emails use the footer. Change their footer before you delete it."},
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			body:       `{"message":"Footer not found."}`,
			wantAPIErr: &APIError{StatusCode: http.StatusNotFound, Message: "Footer not found."},
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
			err := client.DeleteFooter(context.Background(), "ftr_1")

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
			if gotMethod != http.MethodDelete {
				t.Errorf("method = %q, want DELETE", gotMethod)
			}
			if gotPath != "/footers/ftr_1" {
				t.Errorf("path = %q, want /footers/ftr_1", gotPath)
			}
		})
	}
}

func TestSetDefaultFooter(t *testing.T) {
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
			body:       setDefaultFooterResponse,
		},
		{
			name:       "not published",
			statusCode: http.StatusConflict,
			body:       `{"message":"The footer is not published."}`,
			wantAPIErr: &APIError{StatusCode: http.StatusConflict, Message: "The footer is not published."},
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			body:       `{"message":"Footer not found."}`,
			wantAPIErr: &APIError{StatusCode: http.StatusNotFound, Message: "Footer not found."},
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
			var gotPath, gotMethod string
			var gotBody map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotMethod = r.Method
				b, _ := io.ReadAll(r.Body)
				json.Unmarshal(b, &gotBody)
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			result, err := client.SetDefaultFooter(context.Background(), "ftr_1", SetDefaultFooterRequest{ExistingEmails: FooterExistingEmailsPreviousDefault})

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
			if gotMethod != http.MethodPost {
				t.Errorf("method = %q, want POST", gotMethod)
			}
			if gotPath != "/footers/ftr_1/set-default" {
				t.Errorf("path = %q, want /footers/ftr_1/set-default", gotPath)
			}
			if gotBody["existingEmails"] != "previousDefault" {
				t.Errorf("body[existingEmails] = %v, want previousDefault", gotBody["existingEmails"])
			}
			if result.ID != "ftr_1" {
				t.Errorf("ID = %q, want ftr_1", result.ID)
			}
			if !result.IsDefault {
				t.Error("IsDefault = false, want true")
			}
			if len(result.Variants) != 1 || result.Variants[0].ID != "var_1" {
				t.Errorf("Variants = %+v, want one variant var_1", result.Variants)
			}
			if result.UpdatedEmailCount != 12 {
				t.Errorf("UpdatedEmailCount = %d, want 12", result.UpdatedEmailCount)
			}
		})
	}
}

func TestGetFooterVariant(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		body          string
		wantAPIErr    *APIError
		wantErrMsg    string
		wantPublished bool
	}{
		{
			name:          "success",
			statusCode:    http.StatusOK,
			body:          footerVariantResponse,
			wantPublished: true,
		},
		{
			name:       "never published",
			statusCode: http.StatusOK,
			body:       unpublishedFooterVariantResponse,
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			body:       `{"message":"Footer variant not found."}`,
			wantAPIErr: &APIError{StatusCode: http.StatusNotFound, Message: "Footer variant not found."},
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
			var gotPath, gotMethod string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotMethod = r.Method
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			result, err := client.GetFooterVariant(context.Background(), "ftr_1", "var_1")

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
			if gotPath != "/footers/ftr_1/variants/var_1" {
				t.Errorf("path = %q, want /footers/ftr_1/variants/var_1", gotPath)
			}
			if result.ID != "var_1" {
				t.Errorf("ID = %q, want var_1", result.ID)
			}
			if result.FooterID != "ftr_1" {
				t.Errorf("FooterID = %q, want ftr_1", result.FooterID)
			}
			if result.ContentRevisionID != "rev_1" {
				t.Errorf("ContentRevisionID = %q, want rev_1", result.ContentRevisionID)
			}
			if result.DraftLMX != "<Paragraph>Draft</Paragraph>" {
				t.Errorf("DraftLMX = %q, want <Paragraph>Draft</Paragraph>", result.DraftLMX)
			}
			if result.CreatedAt != "2026-10-27T10:00:00.000Z" {
				t.Errorf("CreatedAt = %q, want 2026-10-27T10:00:00.000Z", result.CreatedAt)
			}
			if !tt.wantPublished {
				if result.PublishedLMX != nil {
					t.Errorf("PublishedLMX = %q, want nil", *result.PublishedLMX)
				}
				if result.PublishedAt != nil {
					t.Errorf("PublishedAt = %q, want nil", *result.PublishedAt)
				}
				return
			}
			if result.PublishedLMX == nil || *result.PublishedLMX != "<Paragraph>Published</Paragraph>" {
				t.Errorf("PublishedLMX = %v, want <Paragraph>Published</Paragraph>", result.PublishedLMX)
			}
			if result.PublishedAt == nil || *result.PublishedAt != "2026-11-02T08:30:00.000Z" {
				t.Errorf("PublishedAt = %v, want 2026-11-02T08:30:00.000Z", result.PublishedAt)
			}
		})
	}
}

func TestUpdateFooterVariant(t *testing.T) {
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
			body:       footerVariantResponse,
		},
		{
			name:       "stale revision",
			statusCode: http.StatusConflict,
			body:       `{"message":"expectedRevisionId is stale."}`,
			wantAPIErr: &APIError{StatusCode: http.StatusConflict, Message: "expectedRevisionId is stale."},
		},
		{
			name:       "lmx too large",
			statusCode: http.StatusRequestEntityTooLarge,
			body:       `{"message":"LMX body exceeds the size limit."}`,
			wantAPIErr: &APIError{StatusCode: http.StatusRequestEntityTooLarge, Message: "LMX body exceeds the size limit."},
		},
		{
			name:       "unsupported lmx",
			statusCode: http.StatusUnprocessableEntity,
			body:       `{"message":"Footers do not support components."}`,
			wantAPIErr: &APIError{StatusCode: http.StatusUnprocessableEntity, Message: "Footers do not support components."},
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
			var gotPath, gotMethod string
			var gotBody map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotMethod = r.Method
				b, _ := io.ReadAll(r.Body)
				json.Unmarshal(b, &gotBody)
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			result, err := client.UpdateFooterVariant(context.Background(), "ftr_1", "var_1", UpdateFooterVariantRequest{
				ExpectedRevisionID: "rev_0",
				LMX:                "<Paragraph>Draft</Paragraph>",
			})

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
			if gotMethod != http.MethodPost {
				t.Errorf("method = %q, want POST", gotMethod)
			}
			if gotPath != "/footers/ftr_1/variants/var_1" {
				t.Errorf("path = %q, want /footers/ftr_1/variants/var_1", gotPath)
			}
			if gotBody["expectedRevisionId"] != "rev_0" {
				t.Errorf("body[expectedRevisionId] = %v, want rev_0", gotBody["expectedRevisionId"])
			}
			if gotBody["lmx"] != "<Paragraph>Draft</Paragraph>" {
				t.Errorf("body[lmx] = %v, want <Paragraph>Draft</Paragraph>", gotBody["lmx"])
			}
			if result.ID != "var_1" {
				t.Errorf("ID = %q, want var_1", result.ID)
			}
			if result.ContentRevisionID != "rev_1" {
				t.Errorf("ContentRevisionID = %q, want rev_1", result.ContentRevisionID)
			}
		})
	}
}

func TestPublishFooterVariant(t *testing.T) {
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
			body:       footerVariantResponse,
		},
		{
			name:       "stale revision",
			statusCode: http.StatusConflict,
			body:       `{"message":"expectedRevisionId is stale."}`,
			wantAPIErr: &APIError{StatusCode: http.StatusConflict, Message: "expectedRevisionId is stale."},
		},
		{
			name:       "missing footer variables",
			statusCode: http.StatusUnprocessableEntity,
			body:       `{"message":"The draft does not have the required footer variables."}`,
			wantAPIErr: &APIError{StatusCode: http.StatusUnprocessableEntity, Message: "The draft does not have the required footer variables."},
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
			var gotPath, gotMethod string
			var gotBody map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotMethod = r.Method
				b, _ := io.ReadAll(r.Body)
				json.Unmarshal(b, &gotBody)
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			result, err := client.PublishFooterVariant(context.Background(), "ftr_1", "var_1", PublishFooterVariantRequest{ExpectedRevisionID: "rev_1"})

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
			if gotMethod != http.MethodPost {
				t.Errorf("method = %q, want POST", gotMethod)
			}
			if gotPath != "/footers/ftr_1/variants/var_1/publish" {
				t.Errorf("path = %q, want /footers/ftr_1/variants/var_1/publish", gotPath)
			}
			if gotBody["expectedRevisionId"] != "rev_1" {
				t.Errorf("body[expectedRevisionId] = %v, want rev_1", gotBody["expectedRevisionId"])
			}
			if result.ID != "var_1" {
				t.Errorf("ID = %q, want var_1", result.ID)
			}
			if result.PublishedLMX == nil || *result.PublishedLMX != "<Paragraph>Published</Paragraph>" {
				t.Errorf("PublishedLMX = %v, want <Paragraph>Published</Paragraph>", result.PublishedLMX)
			}
		})
	}
}
