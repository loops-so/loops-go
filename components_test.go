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

const getComponentResponse = `{
	"success": true,
	"id": "cmpt_abc123",
	"name": "Header",
	"lmx": "<H1>Hello</H1>"
}`

const createComponentResponse = `{
	"success": true,
	"id": "cmpt_abc123",
	"name": "Header",
	"lmx": "<H1>Hello</H1>"
}`

const updateComponentResponse = `{
	"success": true,
	"id": "cmpt_abc123",
	"name": "Header",
	"lmx": "<H1>Updated</H1>",
	"affectedEmailCount": 3
}`

const listComponentsResponse = `{
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
			"id": "cmpt_1",
			"name": "Header",
			"lmx": "<H1>Hello</H1>"
		},
		{
			"id": "cmpt_2",
			"name": "Footer",
			"lmx": "<Paragraph>Bye</Paragraph>"
		}
	]
}`

func TestGetComponent(t *testing.T) {
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
			id:         "cmpt_abc123",
			statusCode: http.StatusOK,
			body:       getComponentResponse,
			wantID:     "cmpt_abc123",
		},
		{
			name:       "not found",
			id:         "cmpt_missing",
			statusCode: http.StatusNotFound,
			body:       `{"success":false,"message":"Component not found"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusNotFound, Message: "Component not found"},
		},
		{
			name:       "invalid id",
			id:         "bad",
			statusCode: http.StatusBadRequest,
			body:       `{"success":false,"message":"Invalid componentId"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusBadRequest, Message: "Invalid componentId"},
		},
		{
			name:       "invalid json",
			id:         "cmpt_abc123",
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
			result, err := client.GetComponent(context.Background(), tt.id)

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
			if want := "/components/" + tt.id; gotPath != want {
				t.Errorf("path = %q, want %q", gotPath, want)
			}
			if result.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", result.ID, tt.wantID)
			}
			if result.Name != "Header" {
				t.Errorf("Name = %q, want Header", result.Name)
			}
			if result.LMX != "<H1>Hello</H1>" {
				t.Errorf("LMX = %q, want <H1>Hello</H1>", result.LMX)
			}
		})
	}
}

func TestCreateComponent(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantAPIErr *APIError
		wantErrMsg string
		wantID     string
	}{
		{
			name:       "success 201",
			statusCode: http.StatusCreated,
			body:       createComponentResponse,
			wantID:     "cmpt_abc123",
		},
		{
			name:       "success 200",
			statusCode: http.StatusOK,
			body:       createComponentResponse,
			wantID:     "cmpt_abc123",
		},
		{
			name:       "invalid body",
			statusCode: http.StatusBadRequest,
			body:       `{"success":false,"message":"Invalid request body"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusBadRequest, Message: "Invalid request body"},
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
			var gotPath, gotMethod, gotBody string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotMethod = r.Method
				b, _ := io.ReadAll(r.Body)
				gotBody = string(b)
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			result, err := client.CreateComponent(context.Background(), CreateComponentRequest{Name: "Header", LMX: "<H1>Hello</H1>"})

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
			if gotPath != "/components" {
				t.Errorf("path = %q, want /components", gotPath)
			}
			var sent map[string]string
			if err := json.Unmarshal([]byte(gotBody), &sent); err != nil {
				t.Fatalf("body unmarshal: %v", err)
			}
			if sent["name"] != "Header" || sent["lmx"] != "<H1>Hello</H1>" {
				t.Errorf("body = %v", sent)
			}
			if result.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", result.ID, tt.wantID)
			}
		})
	}
}

func TestUpdateComponent(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		req        UpdateComponentRequest
		statusCode int
		body       string
		wantAPIErr *APIError
		wantErrMsg string
		wantCount  int
	}{
		{
			name:       "lmx change",
			id:         "cmpt_abc123",
			req:        UpdateComponentRequest{LMX: "<H1>Updated</H1>"},
			statusCode: http.StatusOK,
			body:       updateComponentResponse,
			wantCount:  3,
		},
		{
			name:       "name only",
			id:         "cmpt_abc123",
			req:        UpdateComponentRequest{Name: "Header"},
			statusCode: http.StatusOK,
			body:       `{"success":true,"id":"cmpt_abc123","name":"Header","lmx":"<H1>Hello</H1>","affectedEmailCount":0}`,
			wantCount:  0,
		},
		{
			name:       "not found",
			id:         "cmpt_missing",
			req:        UpdateComponentRequest{Name: "Header"},
			statusCode: http.StatusNotFound,
			body:       `{"success":false,"message":"Component not found"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusNotFound, Message: "Component not found"},
		},
		{
			name:       "unprocessable",
			id:         "cmpt_abc123",
			req:        UpdateComponentRequest{},
			statusCode: http.StatusUnprocessableEntity,
			body:       `{"success":false,"message":"At least one of name or lmx must be provided"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusUnprocessableEntity, Message: "At least one of name or lmx must be provided"},
		},
		{
			name:       "invalid json",
			id:         "cmpt_abc123",
			req:        UpdateComponentRequest{Name: "Header"},
			statusCode: http.StatusOK,
			body:       `not json`,
			wantErrMsg: "failed to decode response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath, gotMethod, gotBody string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotMethod = r.Method
				b, _ := io.ReadAll(r.Body)
				gotBody = string(b)
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			result, err := client.UpdateComponent(context.Background(), tt.id, tt.req)

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
			if want := "/components/" + tt.id; gotPath != want {
				t.Errorf("path = %q, want %q", gotPath, want)
			}
			var sent map[string]string
			if err := json.Unmarshal([]byte(gotBody), &sent); err != nil {
				t.Fatalf("body unmarshal: %v", err)
			}
			if tt.req.Name != "" && sent["name"] != tt.req.Name {
				t.Errorf("body name = %q, want %q", sent["name"], tt.req.Name)
			}
			if tt.req.LMX != "" && sent["lmx"] != tt.req.LMX {
				t.Errorf("body lmx = %q, want %q", sent["lmx"], tt.req.LMX)
			}
			if result.AffectedEmailCount != tt.wantCount {
				t.Errorf("AffectedEmailCount = %d, want %d", result.AffectedEmailCount, tt.wantCount)
			}
			if result.ID != "cmpt_abc123" {
				t.Errorf("ID = %q, want cmpt_abc123", result.ID)
			}
		})
	}
}

func TestListComponents(t *testing.T) {
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
			body:       listComponentsResponse,
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
			name:       "invalid perPage",
			statusCode: http.StatusBadRequest,
			body:       `{"success":false,"message":"perPage must be between 10 and 50"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusBadRequest, Message: "perPage must be between 10 and 50"},
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
			components, pagination, err := client.ListComponents(context.Background(), PaginationParams{})

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
			if len(components) != tt.wantCount {
				t.Errorf("len(components) = %d, want %d", len(components), tt.wantCount)
			}
			if pagination == nil {
				t.Fatal("expected pagination, got nil")
			}
		})
	}
}

func TestListComponents_ResponseData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(listComponentsResponse))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	components, _, err := client.ListComponents(context.Background(), PaginationParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if components[0].ID != "cmpt_1" {
		t.Errorf("ID = %q, want cmpt_1", components[0].ID)
	}
	if components[0].Name != "Header" {
		t.Errorf("Name = %q, want Header", components[0].Name)
	}
	if components[1].LMX != "<Paragraph>Bye</Paragraph>" {
		t.Errorf("LMX = %q, want <Paragraph>Bye</Paragraph>", components[1].LMX)
	}
}

func TestListComponents_QueryParams(t *testing.T) {
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
			client.ListComponents(context.Background(), tt.params)

			if gotPerPage != tt.wantPerPage {
				t.Errorf("perPage = %q, want %q", gotPerPage, tt.wantPerPage)
			}
			if gotCursor != tt.wantCursor {
				t.Errorf("cursor = %q, want %q", gotCursor, tt.wantCursor)
			}
		})
	}
}
