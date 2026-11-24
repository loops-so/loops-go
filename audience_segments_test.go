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

const getAudienceSegmentResponse = `{
	"id": "seg_abc123",
	"name": "Power users",
	"description": "Users who clicked the welcome email",
	"createdAt": "2026-05-01T10:00:00Z",
	"updatedAt": "2026-05-02T10:00:00Z",
	"filter": {
		"match": "all",
		"conditions": [
			{
				"type": "property",
				"key": "plan",
				"operator": "equals",
				"value": "pro"
			},
			{
				"type": "optIn",
				"status": "accepted"
			},
			{
				"type": "activity",
				"action": "clicked",
				"negate": false,
				"target": "campaign",
				"id": "cmp_welcome"
			}
		]
	}
}`

const listAudienceSegmentsResponse = `{
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
			"id": "seg_1",
			"name": "Power users",
			"description": null,
			"createdAt": "2026-05-01T10:00:00Z",
			"updatedAt": "2026-05-02T10:00:00Z",
			"filter": null
		},
		{
			"id": "seg_2",
			"name": "Recent signups",
			"description": "Last 30 days",
			"createdAt": "2026-04-01T10:00:00Z",
			"updatedAt": "2026-04-05T10:00:00Z",
			"filter": {
				"match": "any",
				"conditions": [
					{
						"type": "property",
						"key": "signedUpAt",
						"operator": "between",
						"value": { "from": "2026-04-01T00:00:00Z", "to": "2026-05-01T00:00:00Z" }
					}
				]
			}
		}
	]
}`

func TestGetAudienceSegment(t *testing.T) {
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
			id:         "seg_abc123",
			statusCode: http.StatusOK,
			body:       getAudienceSegmentResponse,
			wantID:     "seg_abc123",
		},
		{
			name:       "not found",
			id:         "seg_missing",
			statusCode: http.StatusNotFound,
			body:       `{"message":"Audience segment not found"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusNotFound, Message: "Audience segment not found"},
		},
		{
			name:       "invalid json",
			id:         "seg_abc123",
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
			result, err := client.GetAudienceSegment(context.Background(), tt.id)

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
			if want := "/audience-segments/" + tt.id; gotPath != want {
				t.Errorf("path = %q, want %q", gotPath, want)
			}
			if result.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", result.ID, tt.wantID)
			}
			if result.Filter == nil {
				t.Fatal("Filter is nil, want non-nil")
			}
			if got := result.Filter.Match; got != "all" {
				t.Errorf("Filter.Match = %q, want all", got)
			}
			if len(result.Filter.Conditions) != 3 {
				t.Fatalf("len(Filter.Conditions) = %d, want 3", len(result.Filter.Conditions))
			}

			p := result.Filter.Conditions[0]
			if p.Type != AudienceConditionTypeProperty {
				t.Errorf("Conditions[0].Type = %q, want %q", p.Type, AudienceConditionTypeProperty)
			}
			if p.Property == nil {
				t.Fatal("Conditions[0].Property is nil")
			}
			if p.Property.Key != "plan" || p.Property.Operator != "equals" {
				t.Errorf("Conditions[0].Property = %+v", p.Property)
			}
			if p.Property.Value == nil || p.Property.Value.String == nil || *p.Property.Value.String != "pro" {
				t.Errorf("Conditions[0].Property.Value = %+v", p.Property.Value)
			}

			o := result.Filter.Conditions[1]
			if o.OptIn == nil || o.OptIn.Status == nil || *o.OptIn.Status != "accepted" {
				t.Errorf("Conditions[1].OptIn = %+v", o.OptIn)
			}

			a := result.Filter.Conditions[2]
			if a.Activity == nil || a.Activity.Action != "clicked" || a.Activity.Target != "campaign" || a.Activity.ID != "cmp_welcome" {
				t.Errorf("Conditions[2].Activity = %+v", a.Activity)
			}
		})
	}
}

const createAudienceSegmentResponse = `{
	"id": "seg_new",
	"name": "Pro plan",
	"description": "Contacts on the pro plan",
	"createdAt": "2026-06-01T10:00:00Z",
	"updatedAt": "2026-06-01T10:00:00Z",
	"filter": {
		"match": "all",
		"conditions": [
			{
				"type": "property",
				"key": "plan",
				"operator": "equals",
				"value": "pro"
			}
		]
	}
}`

func TestCreateAudienceSegment(t *testing.T) {
	tests := []struct {
		name       string
		req        CreateAudienceSegmentRequest
		statusCode int
		body       string
		wantAPIErr *APIError
		wantErrMsg string
		wantBody   string
		wantID     string
	}{
		{
			name: "success",
			req: CreateAudienceSegmentRequest{
				Name:        "Pro plan",
				Description: "Contacts on the pro plan",
				Filter: AudienceFilter{
					Match: "all",
					Conditions: []AudienceFilterCondition{
						{
							Type: AudienceConditionTypeProperty,
							Property: &PropertyCondition{
								Key:      "plan",
								Operator: "equals",
								Value:    &PropertyConditionValue{String: ptr("pro")},
							},
						},
					},
				},
			},
			statusCode: http.StatusOK,
			body:       createAudienceSegmentResponse,
			wantBody:   `{"name":"Pro plan","description":"Contacts on the pro plan","filter":{"match":"all","conditions":[{"key":"plan","operator":"equals","type":"property","value":"pro"}]}}`,
			wantID:     "seg_new",
		},
		{
			name: "bad request",
			req: CreateAudienceSegmentRequest{
				Name: "Pro plan",
				Filter: AudienceFilter{
					Match: "all",
					Conditions: []AudienceFilterCondition{
						{
							Type:     AudienceConditionTypeProperty,
							Property: &PropertyCondition{Key: "plan", Operator: "equals", Value: &PropertyConditionValue{String: ptr("pro")}},
						},
					},
				},
			},
			statusCode: http.StatusBadRequest,
			body:       `{"message":"A segment with this name already exists"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusBadRequest, Message: "A segment with this name already exists"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotPath, gotBody string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				buf := new(strings.Builder)
				io.Copy(buf, r.Body)
				gotBody = buf.String()
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			result, err := client.CreateAudienceSegment(context.Background(), tt.req)

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
			if gotPath != "/audience-segments" {
				t.Errorf("path = %q, want /audience-segments", gotPath)
			}
			if gotBody != tt.wantBody {
				t.Errorf("body = %s, want %s", gotBody, tt.wantBody)
			}
			if result.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", result.ID, tt.wantID)
			}
			if result.Filter == nil {
				t.Fatal("Filter is nil, want non-nil")
			}
			if result.Filter.Match != "all" {
				t.Errorf("Filter.Match = %q, want all", result.Filter.Match)
			}
		})
	}
}

func TestListAudienceSegments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(listAudienceSegmentsResponse))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	segments, pagination, err := client.ListAudienceSegments(context.Background(), PaginationParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(segments) != 2 {
		t.Fatalf("len(segments) = %d, want 2", len(segments))
	}
	if pagination == nil {
		t.Fatal("pagination is nil")
	}
	if segments[0].Filter != nil {
		t.Errorf("segments[0].Filter = %+v, want nil", segments[0].Filter)
	}
	if segments[0].Description != nil {
		t.Errorf("segments[0].Description = %v, want nil", segments[0].Description)
	}
	if segments[1].Description == nil || *segments[1].Description != "Last 30 days" {
		t.Errorf("segments[1].Description = %v, want \"Last 30 days\"", segments[1].Description)
	}
	if segments[1].Filter == nil {
		t.Fatal("segments[1].Filter is nil")
	}
	cond := segments[1].Filter.Conditions[0]
	if cond.Property == nil || cond.Property.Operator != "between" {
		t.Fatalf("Conditions[0].Property = %+v", cond.Property)
	}
	if cond.Property.Value == nil || cond.Property.Value.Range == nil {
		t.Fatalf("Conditions[0].Property.Value = %+v", cond.Property.Value)
	}
	if cond.Property.Value.Range.From != "2026-04-01T00:00:00Z" || cond.Property.Value.Range.To != "2026-05-01T00:00:00Z" {
		t.Errorf("Range = %+v", cond.Property.Value.Range)
	}
}

func TestListAudienceSegments_QueryParams(t *testing.T) {
	var gotPerPage, gotCursor string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPerPage = r.URL.Query().Get("perPage")
		gotCursor = r.URL.Query().Get("cursor")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"pagination":{},"data":[]}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	if _, _, err := client.ListAudienceSegments(context.Background(), PaginationParams{PerPage: "10", Cursor: "xyz"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPerPage != "10" {
		t.Errorf("perPage = %q, want 10", gotPerPage)
	}
	if gotCursor != "xyz" {
		t.Errorf("cursor = %q, want xyz", gotCursor)
	}
}

func TestAudienceFilterCondition_MarshalRoundTrip(t *testing.T) {
	num := 5.0
	str := "pro"
	tests := []struct {
		name      string
		condition AudienceFilterCondition
		wantJSON  string
	}{
		{
			name: "property string",
			condition: AudienceFilterCondition{
				Type: AudienceConditionTypeProperty,
				Property: &PropertyCondition{
					Key:      "plan",
					Operator: "equals",
					Value:    &PropertyConditionValue{String: &str},
				},
			},
			wantJSON: `{"key":"plan","operator":"equals","type":"property","value":"pro"}`,
		},
		{
			name: "property number",
			condition: AudienceFilterCondition{
				Type: AudienceConditionTypeProperty,
				Property: &PropertyCondition{
					Key:      "score",
					Operator: "greaterThan",
					Value:    &PropertyConditionValue{Number: &num},
				},
			},
			wantJSON: `{"key":"score","operator":"greaterThan","type":"property","value":5}`,
		},
		{
			name: "property no value",
			condition: AudienceFilterCondition{
				Type: AudienceConditionTypeProperty,
				Property: &PropertyCondition{
					Key:      "verified",
					Operator: "isTrue",
				},
			},
			wantJSON: `{"key":"verified","operator":"isTrue","type":"property"}`,
		},
		{
			name: "opt-in",
			condition: AudienceFilterCondition{
				Type:  AudienceConditionTypeOptIn,
				OptIn: &OptInCondition{Status: ptr("pending")},
			},
			wantJSON: `{"status":"pending","type":"optIn"}`,
		},
		{
			name: "opt-in null",
			condition: AudienceFilterCondition{
				Type:  AudienceConditionTypeOptIn,
				OptIn: &OptInCondition{Status: nil},
			},
			wantJSON: `{"status":null,"type":"optIn"}`,
		},
		{
			name: "activity",
			condition: AudienceFilterCondition{
				Type: AudienceConditionTypeActivity,
				Activity: &ActivityCondition{
					Action: "opened",
					Negate: true,
					Target: "workflow",
					ID:     "wf_1",
				},
			},
			wantJSON: `{"action":"opened","id":"wf_1","negate":true,"target":"workflow","type":"activity"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := json.Marshal(tt.condition)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(raw) != tt.wantJSON {
				t.Errorf("Marshal = %s, want %s", raw, tt.wantJSON)
			}
			var round AudienceFilterCondition
			if err := json.Unmarshal(raw, &round); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if round.Type != tt.condition.Type {
				t.Errorf("round-trip Type = %q, want %q", round.Type, tt.condition.Type)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }
