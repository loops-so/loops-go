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

const listCampaignsResponse = `{
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
			"id": "cmp_1",
			"url": "https://app.loops.so/campaigns/cmp_1",
			"emailMessageId": "em_1",
			"name": "Spring Launch",
			"subject": "New arrivals",
			"status": "Draft",
			"createdAt": "2026-04-01T10:00:00Z",
			"updatedAt": "2026-04-02T10:00:00Z"
		},
		{
			"id": "cmp_2",
			"emailMessageId": null,
			"name": "Summer Sale",
			"subject": "50% off",
			"status": "Sent",
			"createdAt": "2026-03-01T10:00:00Z",
			"updatedAt": "2026-03-05T10:00:00Z"
		}
	]
}`

const createCampaignResponse = `{
	"success": true,
	"id": "cmp_new",
	"url": "https://app.loops.so/campaigns/cmp_new",
	"name": "Spring Launch",
	"status": "Draft",
	"createdAt": "2026-04-20T10:00:00Z",
	"updatedAt": "2026-04-20T10:00:00Z",
	"emailMessageId": "em_new",
	"emailMessageContentRevisionId": "rev_1"
}`

func TestCreateCampaign(t *testing.T) {
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
			body:       createCampaignResponse,
		},
		{
			name:       "bad request",
			statusCode: http.StatusBadRequest,
			body:       `{"success":false,"message":"name is required"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusBadRequest, Message: "name is required"},
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
			resp, err := client.CreateCampaign(CreateCampaignRequest{Name: "Spring Launch"})

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
			if resp.ID != "cmp_new" {
				t.Errorf("ID = %q, want cmp_new", resp.ID)
			}
			if resp.URL != "https://app.loops.so/campaigns/cmp_new" {
				t.Errorf("URL = %q", resp.URL)
			}
			if resp.EmailMessageID == nil || *resp.EmailMessageID != "em_new" {
				t.Errorf("EmailMessageID = %v, want em_new", resp.EmailMessageID)
			}
			if resp.EmailMessageContentRevisionID == nil || *resp.EmailMessageContentRevisionID != "rev_1" {
				t.Errorf("EmailMessageContentRevisionID = %v, want rev_1", resp.EmailMessageContentRevisionID)
			}
		})
	}
}

func TestCreateCampaign_RequestBody(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &body)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(createCampaignResponse))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	if _, err := client.CreateCampaign(CreateCampaignRequest{Name: "Spring"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if body["name"] != "Spring" {
		t.Errorf("name = %v, want Spring", body["name"])
	}
	if _, hasEmail := body["emailMessage"]; hasEmail {
		t.Errorf("emailMessage should not be present in request body, got %v", body["emailMessage"])
	}
}

const updateCampaignResponse = `{
	"success": true,
	"id": "cmp_abc123",
	"emailMessageId": "em_abc123",
	"name": "Renamed",
	"status": "Draft",
	"createdAt": "2026-04-01T10:00:00Z",
	"updatedAt": "2026-04-25T10:00:00Z"
}`

func TestUpdateCampaign(t *testing.T) {
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
			body:       updateCampaignResponse,
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			body:       `{"success":false,"message":"Campaign not found"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusNotFound, Message: "Campaign not found"},
		},
		{
			name:       "not in draft",
			statusCode: http.StatusConflict,
			body:       `{"success":false,"message":"Campaign is not in draft status"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusConflict, Message: "Campaign is not in draft status"},
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
			result, err := client.UpdateCampaign("cmp_abc123", UpdateCampaignRequest{
				Name: "Renamed",
				Set:  map[string]bool{"name": true},
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
			if result.ID != "cmp_abc123" {
				t.Errorf("ID = %q, want cmp_abc123", result.ID)
			}
			if result.Name != "Renamed" {
				t.Errorf("Name = %q, want Renamed", result.Name)
			}
		})
	}
}

func TestUpdateCampaign_RequestBodyAndPath(t *testing.T) {
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
		w.Write([]byte(updateCampaignResponse))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	req := UpdateCampaignRequest{
		Name: "Renamed",
		Set:  map[string]bool{"name": true},
	}
	if _, err := client.UpdateCampaign("cmp_abc123", req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/campaigns/cmp_abc123" {
		t.Errorf("path = %q, want /campaigns/cmp_abc123", gotPath)
	}
	if body["name"] != "Renamed" {
		t.Errorf("name = %v, want Renamed", body["name"])
	}
}

func TestGetCampaign(t *testing.T) {
	body := `{
		"success": true,
		"id": "cmp_abc123",
		"url": "https://app.loops.so/campaigns/cmp_abc123",
		"emailMessageId": "em_abc123",
		"name": "Spring Launch",
		"status": "Draft",
		"createdAt": "2026-04-01T10:00:00Z",
		"updatedAt": "2026-04-02T10:00:00Z"
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
			id:         "cmp_abc123",
			statusCode: http.StatusOK,
			body:       body,
			wantID:     "cmp_abc123",
		},
		{
			name:       "not found",
			id:         "cmp_missing",
			statusCode: http.StatusNotFound,
			body:       `{"success":false,"message":"Campaign not found"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusNotFound, Message: "Campaign not found"},
		},
		{
			name:       "invalid id",
			id:         "bad",
			statusCode: http.StatusBadRequest,
			body:       `{"success":false,"message":"Invalid campaignId"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusBadRequest, Message: "Invalid campaignId"},
		},
		{
			name:       "invalid json",
			id:         "cmp_abc123",
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
			result, err := client.GetCampaign(tt.id)

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
			if want := "/campaigns/" + tt.id; gotPath != want {
				t.Errorf("path = %q, want %q", gotPath, want)
			}
			if result.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", result.ID, tt.wantID)
			}
			if result.URL != "https://app.loops.so/campaigns/cmp_abc123" {
				t.Errorf("URL = %q", result.URL)
			}
			if result.EmailMessageID == nil || *result.EmailMessageID != "em_abc123" {
				t.Errorf("EmailMessageID = %v, want em_abc123", result.EmailMessageID)
			}
			if result.Name != "Spring Launch" {
				t.Errorf("Name = %q, want Spring Launch", result.Name)
			}
			if result.Status != "Draft" {
				t.Errorf("Status = %q, want Draft", result.Status)
			}
		})
	}
}

func TestListCampaigns(t *testing.T) {
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
			body:       listCampaignsResponse,
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
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			campaigns, pagination, err := client.ListCampaigns(PaginationParams{})

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
			if len(campaigns) != tt.wantCount {
				t.Errorf("len(campaigns) = %d, want %d", len(campaigns), tt.wantCount)
			}
			if pagination == nil {
				t.Fatal("expected pagination, got nil")
			}
		})
	}
}

func TestListCampaigns_ResponseData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(listCampaignsResponse))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	campaigns, _, err := client.ListCampaigns(PaginationParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if campaigns[0].ID != "cmp_1" {
		t.Errorf("ID = %q, want cmp_1", campaigns[0].ID)
	}
	if campaigns[0].URL != "https://app.loops.so/campaigns/cmp_1" {
		t.Errorf("URL = %q", campaigns[0].URL)
	}
	if campaigns[0].EmailMessageID == nil || *campaigns[0].EmailMessageID != "em_1" {
		t.Errorf("EmailMessageID = %v, want em_1", campaigns[0].EmailMessageID)
	}
	if campaigns[0].Status != "Draft" {
		t.Errorf("Status = %q, want Draft", campaigns[0].Status)
	}
	if campaigns[1].EmailMessageID != nil {
		t.Errorf("expected nil EmailMessageID, got %v", campaigns[1].EmailMessageID)
	}
}

func TestListCampaigns_QueryParams(t *testing.T) {
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
			client.ListCampaigns(tt.params)

			if gotPerPage != tt.wantPerPage {
				t.Errorf("perPage = %q, want %q", gotPerPage, tt.wantPerPage)
			}
			if gotCursor != tt.wantCursor {
				t.Errorf("cursor = %q, want %q", gotCursor, tt.wantCursor)
			}
		})
	}
}

func TestGetCampaign_WithGroupAndScheduling(t *testing.T) {
	body := `{
		"id": "cmp_abc123",
		"emailMessageId": "em_abc123",
		"name": "Spring Launch",
		"status": "Draft",
		"createdAt": "2026-04-01T10:00:00Z",
		"updatedAt": "2026-04-02T10:00:00Z",
		"campaignGroupId": "grp_news",
		"mailingListId": null,
		"audienceSegmentId": "seg_pro",
		"audienceFilter": null,
		"scheduling": { "method": "schedule", "timestamp": "2026-06-01T09:00:00Z" }
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	result, err := client.GetCampaign("cmp_abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.CampaignGroupID == nil || *result.CampaignGroupID != "grp_news" {
		t.Errorf("CampaignGroupID = %v, want grp_news", result.CampaignGroupID)
	}
	if result.MailingListID != nil {
		t.Errorf("MailingListID = %v, want nil", result.MailingListID)
	}
	if result.AudienceSegmentID == nil || *result.AudienceSegmentID != "seg_pro" {
		t.Errorf("AudienceSegmentID = %v, want seg_pro", result.AudienceSegmentID)
	}
	if result.Scheduling.Method != CampaignSchedulingMethodSchedule {
		t.Errorf("Scheduling.Method = %q, want schedule", result.Scheduling.Method)
	}
	if result.Scheduling.Timestamp == nil || *result.Scheduling.Timestamp != "2026-06-01T09:00:00Z" {
		t.Errorf("Scheduling.Timestamp = %v", result.Scheduling.Timestamp)
	}
}

func TestCreateCampaign_AllFields(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &body)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(createCampaignResponse))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	segment := "seg_pro"
	req := CreateCampaignRequest{
		Name:              "Spring",
		CampaignGroupID:   "grp_news",
		AudienceSegmentID: &segment,
		Scheduling: &CampaignSchedulingRequest{
			Method:    CampaignSchedulingMethodSchedule,
			Timestamp: "2026-06-01T09:00:00Z",
		},
	}
	if _, err := client.CreateCampaign(req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if body["name"] != "Spring" {
		t.Errorf("name = %v", body["name"])
	}
	if body["campaignGroupId"] != "grp_news" {
		t.Errorf("campaignGroupId = %v", body["campaignGroupId"])
	}
	if body["audienceSegmentId"] != "seg_pro" {
		t.Errorf("audienceSegmentId = %v", body["audienceSegmentId"])
	}
	sch, _ := body["scheduling"].(map[string]any)
	if sch["method"] != "schedule" || sch["timestamp"] != "2026-06-01T09:00:00Z" {
		t.Errorf("scheduling = %v", sch)
	}
	if _, has := body["mailingListId"]; has {
		t.Errorf("mailingListId should be omitted, got %v", body["mailingListId"])
	}
}

func TestUpdateCampaign_NullableFields(t *testing.T) {
	var body map[string]any
	var rawBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		rawBody = string(b)
		json.Unmarshal(b, &body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(updateCampaignResponse))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	req := UpdateCampaignRequest{
		MailingListID: nil,
		Set: map[string]bool{
			"mailingListId":     true,
			"audienceSegmentId": true,
		},
	}
	segment := "seg_pro"
	req.AudienceSegmentID = &segment

	if _, err := client.UpdateCampaign("cmp_abc123", req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := body["mailingListId"]; !ok {
		t.Errorf("mailingListId missing from body: %s", rawBody)
	}
	if body["mailingListId"] != nil {
		t.Errorf("mailingListId = %v, want null", body["mailingListId"])
	}
	if body["audienceSegmentId"] != "seg_pro" {
		t.Errorf("audienceSegmentId = %v", body["audienceSegmentId"])
	}
	if _, has := body["name"]; has {
		t.Errorf("name should not be sent, got %v", body["name"])
	}
}

func TestGetCampaignMetrics(t *testing.T) {
	want := EmailMetrics{
		Sends:        4210,
		Opens:        1922,
		Clicks:       301,
		Unsubscribes: 12,
		SpamReports:  1,
		HardBounces:  9,
		SoftBounces:  13,
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
			id:         "cmp_abc123",
			statusCode: http.StatusOK,
			body:       `{"sends":4210,"opens":1922,"clicks":301,"unsubscribes":12,"spamReports":1,"hardBounces":9,"softBounces":13}`,
		},
		{
			name:       "not found",
			id:         "cmp_missing",
			statusCode: http.StatusNotFound,
			body:       `{"message":"Campaign not found."}`,
			wantAPIErr: &APIError{StatusCode: http.StatusNotFound, Message: "Campaign not found."},
		},
		{
			name:       "invalid id",
			id:         "bad",
			statusCode: http.StatusBadRequest,
			body:       `{"message":"Invalid campaignId"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusBadRequest, Message: "Invalid campaignId"},
		},
		{
			name:       "invalid json",
			id:         "cmp_abc123",
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
			result, err := client.GetCampaignMetrics(tt.id)

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
			if wantPath := "/campaigns/" + tt.id + "/metrics"; gotPath != wantPath {
				t.Errorf("path = %q, want %q", gotPath, wantPath)
			}
			if *result != want {
				t.Errorf("metrics = %+v, want %+v", *result, want)
			}
		})
	}
}
