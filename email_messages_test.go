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

func TestGetEmailMessage(t *testing.T) {
	body := `{
		"success": true,
		"id": "em_abc123",
		"campaignId": "cmp_xyz789",
		"subject": "Hello",
		"previewText": "Preview",
		"fromName": "Acme",
		"fromEmail": "hello",
		"replyToEmail": "support@acme.com",
		"lmx": "<Paragraph>Hi</Paragraph><Paragraph>Body text.</Paragraph>",
		"contentRevisionId": "rev_1",
		"updatedAt": "2026-04-20T10:00:00Z"
	}`

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
			id:         "em_abc123",
			statusCode: http.StatusOK,
			body:       body,
			wantID:     "em_abc123",
		},
		{
			name:       "not found",
			id:         "em_missing",
			statusCode: http.StatusNotFound,
			body:       `{"success":false,"message":"Email message not found"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusNotFound, Message: "Email message not found"},
		},
		{
			name:       "mjml conflict",
			id:         "em_mjml",
			statusCode: http.StatusConflict,
			body:       `{"success":false,"message":"Email message uses MJML format"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusConflict, Message: "Email message uses MJML format"},
		},
		{
			name:       "invalid json",
			id:         "em_abc123",
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
			result, err := client.GetEmailMessage(tt.id)

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
			if want := "/email-messages/" + tt.id; gotPath != want {
				t.Errorf("path = %q, want %q", gotPath, want)
			}
			if result.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", result.ID, tt.wantID)
			}
			if result.CampaignID == nil || *result.CampaignID != "cmp_xyz789" {
				t.Errorf("CampaignID = %v, want cmp_xyz789", result.CampaignID)
			}
			if result.Subject != "Hello" {
				t.Errorf("Subject = %q, want Hello", result.Subject)
			}
			if result.LMX != "<Paragraph>Hi</Paragraph><Paragraph>Body text.</Paragraph>" {
				t.Errorf("LMX = %q", result.LMX)
			}
			if result.ContentRevisionID == nil || *result.ContentRevisionID != "rev_1" {
				t.Errorf("ContentRevisionID = %v, want rev_1", result.ContentRevisionID)
			}
		})
	}
}

const updateEmailMessageResponse = `{
	"success": true,
	"id": "em_abc123",
	"campaignId": "cmp_xyz789",
	"subject": "Updated",
	"previewText": "new preview",
	"fromName": "Acme",
	"fromEmail": "hello",
	"replyToEmail": "support@acme.com",
	"lmx": "<Paragraph>Hi</Paragraph>",
	"contentRevisionId": "rev_2",
	"updatedAt": "2026-04-20T11:00:00Z",
	"warnings": [
		{"rule":"unknown_attr","severity":"warning","message":"unknown","path":"body.0"}
	]
}`

func TestUpdateEmailMessage(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantAPIErr *APIError
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
			body:       updateEmailMessageResponse,
		},
		{
			name:       "revision conflict",
			statusCode: http.StatusConflict,
			body:       `{"success":false,"message":"Revision mismatch"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusConflict, Message: "Revision mismatch"},
		},
		{
			name:       "workflow sending",
			statusCode: http.StatusConflict,
			body:       `{"message":"Workflow must not be sending. Current status: Sending."}`,
			wantAPIErr: &APIError{StatusCode: http.StatusConflict, Message: "Workflow must not be sending. Current status: Sending."},
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			body:       `{"success":false,"message":"Email message not found"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusNotFound, Message: "Email message not found"},
		},
		{
			name:       "lmx compile failure",
			statusCode: http.StatusUnprocessableEntity,
			body:       `{"success":false,"message":"LMX failed to compile"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusUnprocessableEntity, Message: "LMX failed to compile"},
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
			req := UpdateEmailMessageRequest{
				EmailMessageFields: EmailMessageFields{Subject: "Updated"},
				Set:                map[string]bool{"subject": true},
			}
			result, err := client.UpdateEmailMessage("em_abc123", req)

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
			if want := "/email-messages/em_abc123"; gotPath != want {
				t.Errorf("path = %q, want %q", gotPath, want)
			}
			if result.ID != "em_abc123" {
				t.Errorf("ID = %q, want em_abc123", result.ID)
			}
			if result.ContentRevisionID == nil || *result.ContentRevisionID != "rev_2" {
				t.Errorf("ContentRevisionID = %v, want rev_2", result.ContentRevisionID)
			}
			if len(result.Warnings) != 1 || result.Warnings[0].Rule != "unknown_attr" {
				t.Errorf("Warnings = %v, want [unknown_attr]", result.Warnings)
			}
		})
	}
}

func TestUpdateEmailMessage_RequestBody(t *testing.T) {
	tests := []struct {
		name       string
		req        UpdateEmailMessageRequest
		wantFields map[string]any
		absent     []string
	}{
		{
			name: "only subject set",
			req: UpdateEmailMessageRequest{
				EmailMessageFields: EmailMessageFields{
					Subject:     "Hello",
					PreviewText: "ignored-not-in-set",
				},
				Set: map[string]bool{"subject": true},
			},
			wantFields: map[string]any{"subject": "Hello"},
			absent:     []string{"previewText", "fromName", "fromEmail", "replyToEmail", "lmx", "expectedRevisionId"},
		},
		{
			name: "blank string is sent when set",
			req: UpdateEmailMessageRequest{
				EmailMessageFields: EmailMessageFields{PreviewText: ""},
				Set:                map[string]bool{"previewText": true},
			},
			wantFields: map[string]any{"previewText": ""},
			absent:     []string{"subject"},
		},
		{
			name: "expected revision id included when non-empty",
			req: UpdateEmailMessageRequest{
				EmailMessageFields: EmailMessageFields{Subject: "Hi"},
				Set:                map[string]bool{"subject": true},
				ExpectedRevisionID: "rev_1",
			},
			wantFields: map[string]any{"subject": "Hi", "expectedRevisionId": "rev_1"},
		},
		{
			name: "all six content fields",
			req: UpdateEmailMessageRequest{
				EmailMessageFields: EmailMessageFields{
					Subject: "s", PreviewText: "p", FromName: "n",
					FromEmail: "u", ReplyToEmail: "r@x.com", LMX: "<p/>",
				},
				Set: map[string]bool{
					"subject": true, "previewText": true, "fromName": true,
					"fromEmail": true, "replyToEmail": true, "lmx": true,
				},
			},
			wantFields: map[string]any{
				"subject": "s", "previewText": "p", "fromName": "n",
				"fromEmail": "u", "replyToEmail": "r@x.com", "lmx": "<p/>",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				b, _ := io.ReadAll(r.Body)
				json.Unmarshal(b, &body)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(updateEmailMessageResponse))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			if _, err := client.UpdateEmailMessage("em_abc123", tt.req); err != nil {
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

func TestGetEmailMessage_NewFields(t *testing.T) {
	body := `{
		"id": "em_abc123",
		"transactionalId": "tx_xyz789",
		"subject": "Hello",
		"previewText": "",
		"fromName": "Acme",
		"fromEmail": "hello",
		"replyToEmail": "support@acme.com",
		"ccEmail": "cc@acme.com",
		"bccEmail": "bcc@acme.com",
		"languageCode": "en-US",
		"emailFormat": "plain",
		"lmx": "<Paragraph>Hi</Paragraph>",
		"contentRevisionId": "rev_1",
		"updatedAt": "2026-06-01T10:00:00Z",
		"contactPropertiesFallbacks": { "firstName": "there" },
		"dataVariablesFallbacks": { "url": "https://example.com" }
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	msg, err := client.GetEmailMessage("em_abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.TransactionalID == nil || *msg.TransactionalID != "tx_xyz789" {
		t.Errorf("TransactionalID = %v, want tx_xyz789", msg.TransactionalID)
	}
	if msg.CampaignID != nil {
		t.Errorf("CampaignID = %v, want nil", msg.CampaignID)
	}
	if msg.CCEmail != "cc@acme.com" || msg.BCCEmail != "bcc@acme.com" {
		t.Errorf("CC/BCC = %q/%q", msg.CCEmail, msg.BCCEmail)
	}
	if msg.LanguageCode != "en-US" {
		t.Errorf("LanguageCode = %q", msg.LanguageCode)
	}
	if msg.EmailFormat != EmailFormatPlain {
		t.Errorf("EmailFormat = %q, want plain", msg.EmailFormat)
	}
	if msg.ContactPropertiesFallbacks["firstName"] != "there" {
		t.Errorf("ContactPropertiesFallbacks = %v", msg.ContactPropertiesFallbacks)
	}
}

func TestUpdateEmailMessage_NewFields(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(updateEmailMessageResponse))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	fallback := "there"
	req := UpdateEmailMessageRequest{
		EmailMessageFields: EmailMessageFields{
			CCEmail:      "cc@acme.com",
			BCCEmail:     "bcc@acme.com",
			LanguageCode: "en-US",
			EmailFormat:  EmailFormatPlain,
			ContactPropertiesFallbacks: map[string]*string{
				"firstName": &fallback,
				"lastName":  nil,
			},
		},
		Set: map[string]bool{
			"ccEmail":                    true,
			"bccEmail":                   true,
			"languageCode":               true,
			"emailFormat":                true,
			"contactPropertiesFallbacks": true,
		},
	}
	if _, err := client.UpdateEmailMessage("em_abc123", req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if body["ccEmail"] != "cc@acme.com" {
		t.Errorf("ccEmail = %v", body["ccEmail"])
	}
	if body["bccEmail"] != "bcc@acme.com" {
		t.Errorf("bccEmail = %v", body["bccEmail"])
	}
	if body["languageCode"] != "en-US" {
		t.Errorf("languageCode = %v", body["languageCode"])
	}
	if body["emailFormat"] != "plain" {
		t.Errorf("emailFormat = %v", body["emailFormat"])
	}
	fb, _ := body["contactPropertiesFallbacks"].(map[string]any)
	if fb["firstName"] != "there" {
		t.Errorf("contactPropertiesFallbacks[firstName] = %v", fb["firstName"])
	}
	if v, ok := fb["lastName"]; !ok || v != nil {
		t.Errorf("contactPropertiesFallbacks[lastName] = %v, want explicit null", v)
	}
}

const previewResponse = `{ "id": "em_abc123" }`

func TestPreviewEmailMessage(t *testing.T) {
	tests := []struct {
		name       string
		req        EmailMessagePreviewRequest
		statusCode int
		body       string
		wantAPIErr *APIError
		wantField  func(t *testing.T, sent map[string]any)
	}{
		{
			name:       "transactional preview",
			req:        EmailMessagePreviewRequest{Emails: []string{"test@example.com"}, DataVariables: map[string]string{"url": "https://example.com"}},
			statusCode: http.StatusOK,
			body:       previewResponse,
			wantField: func(t *testing.T, sent map[string]any) {
				emails, _ := sent["emails"].([]any)
				if len(emails) != 1 || emails[0] != "test@example.com" {
					t.Errorf("emails = %v", emails)
				}
				dv, _ := sent["dataVariables"].(map[string]any)
				if dv["url"] != "https://example.com" {
					t.Errorf("dataVariables = %v", dv)
				}
				if _, has := sent["contactProperties"]; has {
					t.Errorf("contactProperties should be omitted")
				}
			},
		},
		{
			name:       "campaign preview with contact properties",
			req:        EmailMessagePreviewRequest{Emails: []string{"a@x.com", "b@x.com"}, ContactProperties: map[string]string{"firstName": "Alice"}},
			statusCode: http.StatusOK,
			body:       previewResponse,
			wantField: func(t *testing.T, sent map[string]any) {
				cp, _ := sent["contactProperties"].(map[string]any)
				if cp["firstName"] != "Alice" {
					t.Errorf("contactProperties = %v", cp)
				}
			},
		},
		{
			name:       "rejected variable field",
			req:        EmailMessagePreviewRequest{Emails: []string{"test@example.com"}, EventProperties: map[string]string{"k": "v"}},
			statusCode: http.StatusBadRequest,
			body:       `{"message":"eventProperties are not allowed for campaign previews"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusBadRequest, Message: "eventProperties are not allowed for campaign previews"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sent map[string]any
			var gotPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				b, _ := io.ReadAll(r.Body)
				json.Unmarshal(b, &sent)
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			resp, err := client.PreviewEmailMessage("em_abc123", tt.req)

			if tt.wantAPIErr != nil {
				var apiErr *APIError
				if !errors.As(err, &apiErr) {
					t.Fatalf("expected *APIError, got %T: %v", err, err)
				}
				if apiErr.StatusCode != tt.wantAPIErr.StatusCode {
					t.Errorf("StatusCode = %d", apiErr.StatusCode)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotPath != "/email-messages/em_abc123/preview" {
				t.Errorf("path = %q", gotPath)
			}
			if resp.ID != "em_abc123" {
				t.Errorf("ID = %q", resp.ID)
			}
			if tt.wantField != nil {
				tt.wantField(t, sent)
			}
		})
	}
}

func TestRenderEmailMessage(t *testing.T) {
	tests := []struct {
		name       string
		req        EmailMessageRenderRequest
		statusCode int
		body       string
		wantAPIErr *APIError
		wantBody   string
		wantField  func(t *testing.T, sent map[string]any)
	}{
		{
			name:       "empty request",
			req:        EmailMessageRenderRequest{},
			statusCode: http.StatusOK,
			body:       `{"html":"<!doctype html><html>...</html>"}`,
			wantBody:   "{}",
		},
		{
			name:       "transactional render",
			req:        EmailMessageRenderRequest{DataVariables: map[string]string{"loginUrl": "https://app.company.com/login"}},
			statusCode: http.StatusOK,
			body:       `{"html":"<!doctype html><html>...</html>"}`,
			wantField: func(t *testing.T, sent map[string]any) {
				dv, _ := sent["dataVariables"].(map[string]any)
				if dv["loginUrl"] != "https://app.company.com/login" {
					t.Errorf("dataVariables = %v", dv)
				}
				if _, has := sent["contactProperties"]; has {
					t.Errorf("contactProperties should be omitted")
				}
				if _, has := sent["eventProperties"]; has {
					t.Errorf("eventProperties should be omitted")
				}
			},
		},
		{
			name:       "workflow render",
			req:        EmailMessageRenderRequest{ContactProperties: map[string]string{"firstName": "Alex"}, EventProperties: map[string]string{"planName": "Pro"}},
			statusCode: http.StatusOK,
			body:       `{"html":"<!doctype html><html>...</html>"}`,
			wantField: func(t *testing.T, sent map[string]any) {
				cp, _ := sent["contactProperties"].(map[string]any)
				if cp["firstName"] != "Alex" {
					t.Errorf("contactProperties = %v", cp)
				}
				ep, _ := sent["eventProperties"].(map[string]any)
				if ep["planName"] != "Pro" {
					t.Errorf("eventProperties = %v", ep)
				}
				if _, has := sent["dataVariables"]; has {
					t.Errorf("dataVariables should be omitted")
				}
			},
		},
		{
			name:       "not found",
			req:        EmailMessageRenderRequest{},
			statusCode: http.StatusNotFound,
			body:       `{"message":"Email message not found."}`,
			wantAPIErr: &APIError{StatusCode: http.StatusNotFound, Message: "Email message not found."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sent map[string]any
			var gotPath, gotMethod, gotBody string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotMethod = r.Method
				b, _ := io.ReadAll(r.Body)
				gotBody = string(b)
				json.Unmarshal(b, &sent)
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			resp, err := client.RenderEmailMessage("em_abc123", tt.req)

			if tt.wantAPIErr != nil {
				var apiErr *APIError
				if !errors.As(err, &apiErr) {
					t.Fatalf("expected *APIError, got %T: %v", err, err)
				}
				if apiErr.StatusCode != tt.wantAPIErr.StatusCode {
					t.Errorf("StatusCode = %d", apiErr.StatusCode)
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
			if gotPath != "/email-messages/em_abc123/render" {
				t.Errorf("path = %q", gotPath)
			}
			if resp.HTML != "<!doctype html><html>...</html>" {
				t.Errorf("HTML = %q", resp.HTML)
			}
			if tt.wantBody != "" && gotBody != tt.wantBody {
				t.Errorf("body = %q, want %q", gotBody, tt.wantBody)
			}
			if tt.wantField != nil {
				tt.wantField(t, sent)
			}
		})
	}
}

func TestGetEmailMessageGuardian(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		statusCode int
		body       string
		wantAPIErr *APIError
		wantErr    int
		wantWarn   int
	}{
		{
			name:       "errors and warnings",
			id:         "em_abc123",
			statusCode: http.StatusOK,
			body: `{
				"errors": [
					{
						"rule": "missingButtonHrefs",
						"title": "Missing button link",
						"description": "Buttons won't work without href value",
						"items": [{"label": "Click here"}]
					}
				],
				"warnings": [
					{
						"rule": "unsupportedContactProperties",
						"title": "Unsupported contact property",
						"description": "This property is not supported",
						"items": [{"label": "First name", "codeName": "firstName"}]
					}
				]
			}`,
			wantErr:  1,
			wantWarn: 1,
		},
		{
			name:       "all clear",
			id:         "em_clean",
			statusCode: http.StatusOK,
			body:       `{"errors":[],"warnings":[]}`,
			wantErr:    0,
			wantWarn:   0,
		},
		{
			name:       "not found",
			id:         "em_missing",
			statusCode: http.StatusNotFound,
			body:       `{"success":false,"message":"Email message not found"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusNotFound, Message: "Email message not found"},
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
			result, err := client.GetEmailMessageGuardian(tt.id)

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
			if want := "/email-messages/" + tt.id + "/guardian"; gotPath != want {
				t.Errorf("path = %q, want %q", gotPath, want)
			}
			if len(result.Errors) != tt.wantErr {
				t.Errorf("len(Errors) = %d, want %d", len(result.Errors), tt.wantErr)
			}
			if len(result.Warnings) != tt.wantWarn {
				t.Errorf("len(Warnings) = %d, want %d", len(result.Warnings), tt.wantWarn)
			}
			if tt.wantErr > 0 {
				e := result.Errors[0]
				if e.Rule != "missingButtonHrefs" {
					t.Errorf("Errors[0].Rule = %q, want missingButtonHrefs", e.Rule)
				}
				if len(e.Items) != 1 || e.Items[0].Label != "Click here" {
					t.Errorf("Errors[0].Items = %v, want [Click here]", e.Items)
				}
				if e.Items[0].CodeName != "" {
					t.Errorf("Errors[0].Items[0].CodeName = %q, want empty", e.Items[0].CodeName)
				}
			}
			if tt.wantWarn > 0 {
				wItems := result.Warnings[0].Items
				if len(wItems) != 1 || wItems[0].Label != "First name" || wItems[0].CodeName != "firstName" {
					t.Errorf("Warnings[0].Items = %v, want label=First name codeName=firstName", wItems)
				}
			}
		})
	}
}
