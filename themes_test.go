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

const getThemeResponse = `{
	"success": true,
	"id": "thm_abc123",
	"name": "Brand",
	"styles": {
		"backgroundColor": "#ffffff",
		"bodyColor": "#fafafa",
		"textBaseColor": "#111111",
		"textBaseFontSize": 16,
		"heading1Color": "#000000",
		"heading1FontSize": 32,
		"buttonBodyColor": "#0066ff",
		"buttonTextColor": "#ffffff",
		"borderRadius": 8
	},
	"isDefault": true,
	"createdAt": "2026-04-01T10:00:00Z",
	"updatedAt": "2026-04-02T10:00:00Z"
}`

const listThemesResponse = `{
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
			"id": "thm_1",
			"name": "Brand",
			"styles": {
				"backgroundColor": "#ffffff",
				"textLinkColor": "#0066ff"
			},
			"isDefault": true,
			"createdAt": "2026-04-01T10:00:00Z",
			"updatedAt": "2026-04-02T10:00:00Z"
		},
		{
			"id": "thm_2",
			"name": "Dark",
			"styles": {
				"backgroundColor": "#000000",
				"textBaseColor": "#ffffff"
			},
			"isDefault": false,
			"createdAt": "2026-03-01T10:00:00Z",
			"updatedAt": "2026-03-05T10:00:00Z"
		}
	]
}`

func TestGetTheme(t *testing.T) {
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
			id:         "thm_abc123",
			statusCode: http.StatusOK,
			body:       getThemeResponse,
			wantID:     "thm_abc123",
		},
		{
			name:       "not found",
			id:         "thm_missing",
			statusCode: http.StatusNotFound,
			body:       `{"success":false,"message":"Theme not found"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusNotFound, Message: "Theme not found"},
		},
		{
			name:       "invalid id",
			id:         "bad",
			statusCode: http.StatusBadRequest,
			body:       `{"success":false,"message":"Invalid themeId"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusBadRequest, Message: "Invalid themeId"},
		},
		{
			name:       "invalid json",
			id:         "thm_abc123",
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
			result, err := client.GetTheme(context.Background(), tt.id)

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
			if want := "/themes/" + tt.id; gotPath != want {
				t.Errorf("path = %q, want %q", gotPath, want)
			}
			if result.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", result.ID, tt.wantID)
			}
			if result.Name != "Brand" {
				t.Errorf("Name = %q, want Brand", result.Name)
			}
			if !result.IsDefault {
				t.Errorf("IsDefault = false, want true")
			}
			if result.Styles.BackgroundColor != "#ffffff" {
				t.Errorf("Styles.BackgroundColor = %q, want #ffffff", result.Styles.BackgroundColor)
			}
			if result.Styles.TextBaseFontSize != 16 {
				t.Errorf("Styles.TextBaseFontSize = %v, want 16", result.Styles.TextBaseFontSize)
			}
			if result.Styles.BorderRadius != 8 {
				t.Errorf("Styles.BorderRadius = %v, want 8", result.Styles.BorderRadius)
			}
		})
	}
}

func TestListThemes(t *testing.T) {
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
			body:       listThemesResponse,
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
			themes, pagination, err := client.ListThemes(context.Background(), PaginationParams{})

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
			if len(themes) != tt.wantCount {
				t.Errorf("len(themes) = %d, want %d", len(themes), tt.wantCount)
			}
			if pagination == nil {
				t.Fatal("expected pagination, got nil")
			}
		})
	}
}

func TestListThemes_ResponseData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(listThemesResponse))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	themes, _, err := client.ListThemes(context.Background(), PaginationParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if themes[0].ID != "thm_1" {
		t.Errorf("ID = %q, want thm_1", themes[0].ID)
	}
	if !themes[0].IsDefault {
		t.Errorf("themes[0].IsDefault = false, want true")
	}
	if themes[0].Styles.TextLinkColor != "#0066ff" {
		t.Errorf("Styles.TextLinkColor = %q, want #0066ff", themes[0].Styles.TextLinkColor)
	}
	if themes[1].IsDefault {
		t.Errorf("themes[1].IsDefault = true, want false")
	}
	if themes[1].Styles.BackgroundColor != "#000000" {
		t.Errorf("Styles.BackgroundColor = %q, want #000000", themes[1].Styles.BackgroundColor)
	}
}

func TestListThemes_QueryParams(t *testing.T) {
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
			client.ListThemes(context.Background(), tt.params)

			if gotPerPage != tt.wantPerPage {
				t.Errorf("perPage = %q, want %q", gotPerPage, tt.wantPerPage)
			}
			if gotCursor != tt.wantCursor {
				t.Errorf("cursor = %q, want %q", gotCursor, tt.wantCursor)
			}
		})
	}
}

func TestCreateTheme(t *testing.T) {
	var gotPath, gotMethod, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(getThemeResponse))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	result, err := client.CreateTheme(context.Background(), CreateThemeRequest{
		Name:   "Brand",
		Styles: &ThemeStyles{BackgroundColor: "#ffffff", TextBaseFontSize: 16},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/themes" {
		t.Errorf("path = %q, want /themes", gotPath)
	}

	var sent struct {
		Name   string      `json:"name"`
		Styles ThemeStyles `json:"styles"`
	}
	if err := json.Unmarshal([]byte(gotBody), &sent); err != nil {
		t.Fatalf("body unmarshal: %v", err)
	}
	if sent.Name != "Brand" {
		t.Errorf("body name = %q, want Brand", sent.Name)
	}
	if sent.Styles.BackgroundColor != "#ffffff" {
		t.Errorf("body styles.backgroundColor = %q, want #ffffff", sent.Styles.BackgroundColor)
	}
	if result.ID != "thm_abc123" {
		t.Errorf("ID = %q, want thm_abc123", result.ID)
	}
	if result.Name != "Brand" {
		t.Errorf("Name = %q, want Brand", result.Name)
	}
}

func TestCreateTheme_OmitsStyles(t *testing.T) {
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(getThemeResponse))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	if _, err := client.CreateTheme(context.Background(), CreateThemeRequest{Name: "Brand"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotBody, `"name":"Brand"`) {
		t.Errorf("body = %q", gotBody)
	}
	if strings.Contains(gotBody, `"styles"`) {
		t.Errorf("body should omit nil styles: %q", gotBody)
	}
}

func TestCreateTheme_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"success":false,"message":"Name is required"}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	_, err := client.CreateTheme(context.Background(), CreateThemeRequest{})

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusBadRequest)
	}
	if apiErr.Message != "Name is required" {
		t.Errorf("Message = %q, want %q", apiErr.Message, "Name is required")
	}
}

func TestUpdateTheme(t *testing.T) {
	tests := []struct {
		name         string
		req          UpdateThemeRequest
		wantContains []string
		wantOmits    []string
	}{
		{
			name:         "name only",
			req:          UpdateThemeRequest{Name: "Renamed"},
			wantContains: []string{`"name":"Renamed"`},
			wantOmits:    []string{`"styles"`},
		},
		{
			name:         "styles only",
			req:          UpdateThemeRequest{Styles: &ThemeStyles{BackgroundColor: "#000000"}},
			wantContains: []string{`"backgroundColor":"#000000"`},
			wantOmits:    []string{`"name"`},
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
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(getThemeResponse))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			result, err := client.UpdateTheme(context.Background(), "thm_abc123", tt.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotMethod != http.MethodPost {
				t.Errorf("method = %q, want POST", gotMethod)
			}
			if gotPath != "/themes/thm_abc123" {
				t.Errorf("path = %q, want /themes/thm_abc123", gotPath)
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(gotBody, want) {
					t.Errorf("body = %q, want it to contain %q", gotBody, want)
				}
			}
			for _, omit := range tt.wantOmits {
				if strings.Contains(gotBody, omit) {
					t.Errorf("body = %q, want it to omit %q", gotBody, omit)
				}
			}
			if result.ID != "thm_abc123" {
				t.Errorf("ID = %q, want thm_abc123", result.ID)
			}
		})
	}
}

func TestUpdateTheme_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"success":false,"message":"Theme not found"}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	_, err := client.UpdateTheme(context.Background(), "thm_missing", UpdateThemeRequest{Name: "Renamed"})

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusNotFound)
	}
	if apiErr.Message != "Theme not found" {
		t.Errorf("Message = %q, want %q", apiErr.Message, "Theme not found")
	}
}
