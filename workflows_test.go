package loops

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

const listWorkflowsResponse = `{
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
			"id": "wf_1",
			"url": "https://app.loops.so/workflows/wf_1",
			"name": "Onboarding",
			"createdAt": "2026-05-01T10:00:00Z",
			"updatedAt": "2026-05-02T10:00:00Z"
		},
		{
			"id": "wf_2",
			"url": "https://app.loops.so/workflows/wf_2",
			"name": "Win-back",
			"createdAt": "2026-04-01T10:00:00Z",
			"updatedAt": "2026-04-05T10:00:00Z"
		}
	]
}`

const getSimplifiedWorkflowResponse = `{
	"id": "wf_1",
	"url": "https://app.loops.so/workflows/wf_1",
	"status": "Draft",
	"workflowRevisionId": "rev_1",
	"name": "Onboarding",
	"description": "Welcome new signups",
	"emoji": "👋",
	"mailingListId": null,
	"rootNodeId": "node_root",
	"nodes": {
		"node_root": {
			"typeName": "SignupTrigger",
			"nextNodeIds": ["node_send"]
		},
		"node_send": {
			"typeName": "SendEmailAction",
			"nextNodeIds": ["node_branch"],
			"emailMessageId": "em_welcome",
			"subject": "Welcome to Acme"
		},
		"node_branch": {
			"typeName": "BranchNode",
			"nextNodeIds": ["node_exit"]
		},
		"node_exit": {
			"typeName": "ExitAction",
			"nextNodeIds": []
		}
	}
}`

func TestListWorkflows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(listWorkflowsResponse))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	workflows, pagination, err := client.ListWorkflows(PaginationParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(workflows) != 2 {
		t.Fatalf("len(workflows) = %d, want 2", len(workflows))
	}
	if pagination == nil {
		t.Fatal("pagination is nil")
	}
	if workflows[0].ID != "wf_1" || workflows[0].Name != "Onboarding" {
		t.Errorf("workflows[0] = %+v", workflows[0])
	}
	if workflows[0].URL != "https://app.loops.so/workflows/wf_1" {
		t.Errorf("workflows[0].URL = %q", workflows[0].URL)
	}
}

func TestListWorkflows_QueryParams(t *testing.T) {
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
	if _, _, err := client.ListWorkflows(PaginationParams{PerPage: "25", Cursor: "abc"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/workflows" {
		t.Errorf("path = %q, want /workflows", gotPath)
	}
	if gotPerPage != "25" {
		t.Errorf("perPage = %q, want 25", gotPerPage)
	}
	if gotCursor != "abc" {
		t.Errorf("cursor = %q, want abc", gotCursor)
	}
}

func TestGetWorkflow(t *testing.T) {
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
			id:         "wf_1",
			statusCode: http.StatusOK,
			body:       getSimplifiedWorkflowResponse,
		},
		{
			name:       "not found",
			id:         "wf_missing",
			statusCode: http.StatusNotFound,
			body:       `{"message":"Workflow not found"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusNotFound, Message: "Workflow not found"},
		},
		{
			name:       "invalid json",
			id:         "wf_1",
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
			result, err := client.GetWorkflow(tt.id)

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
			if want := "/workflows/" + tt.id; gotPath != want {
				t.Errorf("path = %q, want %q", gotPath, want)
			}
			if result.URL != "https://app.loops.so/workflows/wf_1" {
				t.Errorf("URL = %q", result.URL)
			}
			if result.ID != "wf_1" {
				t.Errorf("ID = %q, want wf_1", result.ID)
			}
			if result.Status != WorkflowStatusDraft {
				t.Errorf("Status = %q, want Draft", result.Status)
			}
			if result.WorkflowRevisionID == nil || *result.WorkflowRevisionID != "rev_1" {
				t.Errorf("WorkflowRevisionID = %v, want rev_1", result.WorkflowRevisionID)
			}
			if result.RootNodeID == nil || *result.RootNodeID != "node_root" {
				t.Errorf("RootNodeID = %v, want node_root", result.RootNodeID)
			}
			if result.MailingListID != nil {
				t.Errorf("MailingListID = %v, want nil", result.MailingListID)
			}
			if len(result.Nodes) != 4 {
				t.Fatalf("len(Nodes) = %d, want 4", len(result.Nodes))
			}

			root := result.Nodes["node_root"]
			if root.TypeName != WorkflowNodeTypeSignupTrigger {
				t.Errorf("root.TypeName = %q, want SignupTrigger", root.TypeName)
			}
			if root.SignupTrigger == nil {
				t.Fatal("root.SignupTrigger is nil")
			}
			if len(root.SignupTrigger.NextNodeIDs) != 1 || root.SignupTrigger.NextNodeIDs[0] != "node_send" {
				t.Errorf("root.SignupTrigger.NextNodeIDs = %v", root.SignupTrigger.NextNodeIDs)
			}

			send := result.Nodes["node_send"]
			if send.TypeName != WorkflowNodeTypeSendEmailAction {
				t.Errorf("send.TypeName = %q, want SendEmailAction", send.TypeName)
			}
			if send.SendEmailAction == nil {
				t.Fatal("send.SendEmailAction is nil")
			}
			if send.SendEmailAction.EmailMessageID != "em_welcome" {
				t.Errorf("send.SendEmailAction.EmailMessageID = %q", send.SendEmailAction.EmailMessageID)
			}
			if send.SendEmailAction.Subject != "Welcome to Acme" {
				t.Errorf("send.SendEmailAction.Subject = %q", send.SendEmailAction.Subject)
			}

			branch := result.Nodes["node_branch"]
			if branch.TypeName != WorkflowNodeTypeBranchNode || branch.BranchNode == nil {
				t.Errorf("branch = %+v", branch)
			}

			exit := result.Nodes["node_exit"]
			if exit.TypeName != WorkflowNodeTypeExitAction || exit.ExitAction == nil {
				t.Errorf("exit = %+v", exit)
			}
		})
	}
}

func TestGetWorkflowNode_EventTrigger(t *testing.T) {
	body := `{
		"id": "node_evt",
		"workflowId": "wf_1",
		"typeName": "EventTrigger",
		"nextNodeIds": ["node_next"],
		"eventName": "signed_up",
		"eventProperties": [
			{"name": "plan", "type": "string"},
			{"name": "seats", "type": "number"}
		],
		"reEligible": true,
		"workflowRevisionId": "rev_1"
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if want := "/workflows/wf_1/nodes/node_evt"; r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	node, err := client.GetWorkflowNode("wf_1", "node_evt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.WorkflowRevisionID == nil || *node.WorkflowRevisionID != "rev_1" {
		t.Errorf("WorkflowRevisionID = %v, want rev_1", node.WorkflowRevisionID)
	}
	if node.TypeName != WorkflowNodeTypeEventTrigger {
		t.Fatalf("TypeName = %q, want EventTrigger", node.TypeName)
	}
	if node.EventTrigger == nil {
		t.Fatal("EventTrigger is nil")
	}
	if node.EventTrigger.ID != "node_evt" || node.EventTrigger.WorkflowID != "wf_1" {
		t.Errorf("EventTrigger ids = %+v", node.EventTrigger)
	}
	if node.EventTrigger.EventName == nil || *node.EventTrigger.EventName != "signed_up" {
		t.Errorf("EventName = %v", node.EventTrigger.EventName)
	}
	if len(node.EventTrigger.EventProperties) != 2 {
		t.Fatalf("len(EventProperties) = %d", len(node.EventTrigger.EventProperties))
	}
	if node.EventTrigger.EventProperties[1].Name != "seats" || node.EventTrigger.EventProperties[1].Type != "number" {
		t.Errorf("EventProperties[1] = %+v", node.EventTrigger.EventProperties[1])
	}
	if !node.EventTrigger.ReEligible {
		t.Error("ReEligible = false, want true")
	}
}

func TestGetWorkflowNode_ContactPropertyTrigger_NullQuery(t *testing.T) {
	body := `{
		"id": "node_cp",
		"workflowId": "wf_1",
		"typeName": "ContactPropertyTrigger",
		"nextNodeIds": [],
		"contactPropertyQuery": null,
		"reEligible": false
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	node, err := client.GetWorkflowNode("wf_1", "node_cp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.ContactPropertyTrigger == nil {
		t.Fatal("ContactPropertyTrigger is nil")
	}
	if node.ContactPropertyTrigger.ContactPropertyQuery != nil {
		t.Errorf("ContactPropertyQuery = %+v, want nil", node.ContactPropertyTrigger.ContactPropertyQuery)
	}
}

func TestGetWorkflowNode_ContactPropertyTrigger_WithQuery(t *testing.T) {
	body := `{
		"id": "node_cp",
		"workflowId": "wf_1",
		"typeName": "ContactPropertyTrigger",
		"nextNodeIds": ["node_next"],
		"contactPropertyQuery": {
			"key": "plan",
			"is": {"value": "pro", "operator": "equal"},
			"was": {"value": "free", "operator": "equal"}
		},
		"reEligible": true
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	node, err := client.GetWorkflowNode("wf_1", "node_cp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	q := node.ContactPropertyTrigger.ContactPropertyQuery
	if q == nil {
		t.Fatal("ContactPropertyQuery is nil")
	}
	if q.Key != "plan" {
		t.Errorf("Key = %q, want plan", q.Key)
	}
	if q.Is.Operator != "equal" {
		t.Errorf("Is.Operator = %q, want equal", q.Is.Operator)
	}
	if q.Is.Value.String == nil || *q.Is.Value.String != "pro" {
		t.Errorf("Is.Value.String = %v", q.Is.Value.String)
	}
	if q.Was.Value.String == nil || *q.Was.Value.String != "free" {
		t.Errorf("Was.Value.String = %v", q.Was.Value.String)
	}
}

func TestGetWorkflowNode_TimerAction(t *testing.T) {
	body := `{
		"id": "node_t",
		"workflowId": "wf_1",
		"typeName": "TimerAction",
		"nextNodeIds": ["node_next"],
		"amount": 3,
		"unit": "d"
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	node, err := client.GetWorkflowNode("wf_1", "node_t")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.TimerAction == nil {
		t.Fatal("TimerAction is nil")
	}
	if node.TimerAction.Amount != 3 {
		t.Errorf("Amount = %v, want 3", node.TimerAction.Amount)
	}
	if node.TimerAction.Unit != WorkflowTimerUnitDays {
		t.Errorf("Unit = %q, want d", node.TimerAction.Unit)
	}
}

func TestGetWorkflowNode_AudienceFilter(t *testing.T) {
	body := `{
		"id": "node_af",
		"workflowId": "wf_1",
		"typeName": "AudienceFilter",
		"nextNodeIds": ["node_next"],
		"audienceFilter": {
			"match": "all",
			"conditions": [
				{"type": "property", "key": "plan", "operator": "equals", "value": "pro"}
			]
		},
		"audienceSegmentId": "seg_abc",
		"appliesDownstream": true
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	node, err := client.GetWorkflowNode("wf_1", "node_af")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.AudienceFilter == nil {
		t.Fatal("AudienceFilter (node) is nil")
	}
	if node.AudienceFilter.AudienceSegmentID != "seg_abc" {
		t.Errorf("AudienceSegmentID = %q, want seg_abc", node.AudienceFilter.AudienceSegmentID)
	}
	af := node.AudienceFilter.AudienceFilter
	if af == nil {
		t.Fatal("AudienceFilter.AudienceFilter is nil")
	}
	if af.Match != "all" || len(af.Conditions) != 1 {
		t.Errorf("audienceFilter = %+v", af)
	}
	if af.Conditions[0].Property == nil || af.Conditions[0].Property.Key != "plan" {
		t.Errorf("audienceFilter.Conditions[0] = %+v", af.Conditions[0])
	}
	if !node.AudienceFilter.AppliesDownstream {
		t.Error("AppliesDownstream = false, want true")
	}
	raw, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"appliesDownstream":true`) {
		t.Errorf("marshal dropped appliesDownstream: %s", raw)
	}
}

func TestAudienceFilterWorkflowNode_MarshalKeepsFalseAppliesDownstream(t *testing.T) {
	n := WorkflowNode{
		TypeName:       WorkflowNodeTypeAudienceFilter,
		AudienceFilter: &AudienceFilterWorkflowNode{ID: "n7", WorkflowID: "wf_1", NextNodeIDs: []string{"n9"}},
	}
	raw, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"appliesDownstream":false`) {
		t.Errorf("marshal dropped appliesDownstream: %s", raw)
	}
}

func TestWorkflowNode_MarshalRoundTrip(t *testing.T) {
	original := WorkflowNode{
		TypeName: WorkflowNodeTypeTimerAction,
		TimerAction: &TimerActionWorkflowNode{
			ID:          "n1",
			WorkflowID:  "wf_1",
			NextNodeIDs: []string{"n2"},
			Amount:      15,
			Unit:        WorkflowTimerUnitMinutes,
		},
	}
	raw, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"typeName":"TimerAction"`) {
		t.Errorf("Marshal missing typeName: %s", raw)
	}

	var round WorkflowNode
	if err := json.Unmarshal(raw, &round); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if round.TypeName != original.TypeName {
		t.Errorf("round-trip TypeName = %q, want %q", round.TypeName, original.TypeName)
	}
	if round.TimerAction == nil || round.TimerAction.Unit != WorkflowTimerUnitMinutes {
		t.Errorf("round-trip TimerAction = %+v", round.TimerAction)
	}
}

func TestSimplifiedWorkflowNode_MarshalRoundTrip(t *testing.T) {
	original := SimplifiedWorkflowNode{
		TypeName: WorkflowNodeTypeEventTrigger,
		EventTrigger: &SimplifiedEventTriggerWorkflowNode{
			NextNodeIDs: []string{"n2"},
			EventName:   "signed_up",
			ReEligible:  ptr(true),
		},
	}
	raw, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"typeName":"EventTrigger"`) {
		t.Errorf("Marshal missing typeName: %s", raw)
	}

	var round SimplifiedWorkflowNode
	if err := json.Unmarshal(raw, &round); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if round.EventTrigger == nil || round.EventTrigger.EventName != "signed_up" {
		t.Errorf("round-trip EventTrigger = %+v", round.EventTrigger)
	}
}

// decodeBody reads and JSON-decodes the request body of a test server handler.
func decodeBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	var got map[string]any
	if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	return got
}

func TestCreateWorkflow(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody map[string]any
	resp := `{
		"id": "wf_new",
		"status": "Draft",
		"workflowRevisionId": null,
		"name": "New flow",
		"mailingListId": "ml_1",
		"rootNodeId": "node_root",
		"nodes": {}
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotBody = decodeBody(t, r)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(resp))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	wf, err := client.CreateWorkflow(CreateWorkflowRequest{
		Name:          "New flow",
		Description:   "desc",
		MailingListID: ptr("ml_1"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/workflows" {
		t.Errorf("path = %q, want /workflows", gotPath)
	}
	if gotBody["name"] != "New flow" || gotBody["description"] != "desc" || gotBody["mailingListId"] != "ml_1" {
		t.Errorf("body = %+v", gotBody)
	}
	if wf.ID != "wf_new" || wf.Status != WorkflowStatusDraft {
		t.Errorf("wf = %+v", wf)
	}
	if wf.WorkflowRevisionID != nil {
		t.Errorf("WorkflowRevisionID = %v, want nil", wf.WorkflowRevisionID)
	}
}

func TestUpdateWorkflow_NullRevision(t *testing.T) {
	var gotBody map[string]any
	var rawBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(strings.Builder)
		tee := io.TeeReader(r.Body, buf)
		json.NewDecoder(tee).Decode(&gotBody)
		rawBody = buf.String()
		if want := "/workflows/wf_1"; r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"wf_1","status":"Draft","workflowRevisionId":"rev_2","name":"Renamed","mailingListId":null,"rootNodeId":"n","nodes":{}}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	wf, err := client.UpdateWorkflow("wf_1", UpdateWorkflowPropertiesRequest{
		ExpectedRevisionID: nil,
		Name:               "Renamed",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(rawBody, `"expectedRevisionId":null`) {
		t.Errorf("body must send explicit null revision: %s", rawBody)
	}
	if _, ok := gotBody["expectedRevisionId"]; !ok {
		t.Error("expectedRevisionId key missing from body")
	}
	if gotBody["name"] != "Renamed" {
		t.Errorf("name = %v, want Renamed", gotBody["name"])
	}
	if _, ok := gotBody["description"]; ok {
		t.Errorf("description should be omitted, body = %+v", gotBody)
	}
	if wf.Name != "Renamed" || wf.WorkflowRevisionID == nil || *wf.WorkflowRevisionID != "rev_2" {
		t.Errorf("wf = %+v", wf)
	}
}

func TestUpdateWorkflow_StaleRevision409(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"message":"Workflow revision is stale."}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	_, err := client.UpdateWorkflow("wf_1", UpdateWorkflowPropertiesRequest{
		ExpectedRevisionID: ptr("old_rev"),
		Name:               "x",
	})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusConflict {
		t.Errorf("StatusCode = %d, want 409", apiErr.StatusCode)
	}
	if apiErr.Message != "Workflow revision is stale." {
		t.Errorf("Message = %q", apiErr.Message)
	}
}

func TestChangeWorkflowMailingList_Preview(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody = decodeBody(t, r)
		if want := "/workflows/wf_1/mailing-list"; r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"queuedContactsFound","mailingListId":"ml_2","queuedContactCount":7}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	res, err := client.ChangeWorkflowMailingList("wf_1", ChangeWorkflowMailingListRequest{
		ExpectedRevisionID: ptr("rev_1"),
		MailingListID:      ptr("ml_2"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["expectedRevisionId"] != "rev_1" || gotBody["mailingListId"] != "ml_2" {
		t.Errorf("body = %+v", gotBody)
	}
	if _, ok := gotBody["dryRun"]; ok {
		t.Errorf("dryRun should be omitted, body = %+v", gotBody)
	}
	if res.Status != WorkflowMutationStatusQueuedContactsFound {
		t.Errorf("Status = %q, want queuedContactsFound", res.Status)
	}
	if res.QueuedContactCount != 7 {
		t.Errorf("QueuedContactCount = %v, want 7", res.QueuedContactCount)
	}
	if res.WorkflowRevisionID != nil {
		t.Errorf("WorkflowRevisionID = %v, want nil", res.WorkflowRevisionID)
	}
	if res.Workflow != nil {
		t.Errorf("Workflow = %+v, want nil on queuedContactsFound", res.Workflow)
	}
}

func TestChangeWorkflowMailingList_NullClear(t *testing.T) {
	var rawBody string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(strings.Builder)
		tee := io.TeeReader(r.Body, buf)
		json.NewDecoder(tee).Decode(&gotBody)
		rawBody = buf.String()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"updated","mailingListId":null,"workflowRevisionId":"rev_3","queuedContactCount":0,"workflow":{"id":"wf_1","status":"Draft","workflowRevisionId":"rev_3","mailingListId":null,"rootNodeId":"r","nodes":{}}}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	res, err := client.ChangeWorkflowMailingList("wf_1", ChangeWorkflowMailingListRequest{
		ExpectedRevisionID:  ptr("rev_2"),
		MailingListID:       nil,
		DryRun:              true,
		QueuedContactPolicy: WorkflowQueuedContactPolicyDiscard,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(rawBody, `"mailingListId":null`) {
		t.Errorf("body must send explicit null mailingListId: %s", rawBody)
	}
	if gotBody["dryRun"] != true || gotBody["queuedContactPolicy"] != "discard" {
		t.Errorf("body = %+v", gotBody)
	}
	if res.Status != WorkflowMutationStatusUpdated {
		t.Errorf("Status = %q, want updated", res.Status)
	}
	if res.WorkflowRevisionID == nil || *res.WorkflowRevisionID != "rev_3" {
		t.Errorf("WorkflowRevisionID = %v, want rev_3", res.WorkflowRevisionID)
	}
	if res.MailingListID != nil {
		t.Errorf("MailingListID = %v, want nil", res.MailingListID)
	}
	if res.Workflow == nil || res.Workflow.ID != "wf_1" {
		t.Errorf("Workflow = %+v, want id wf_1", res.Workflow)
	}
}

func TestCreateWorkflowNode_Between(t *testing.T) {
	var gotBody map[string]any
	resp := `{
		"node": {
			"id": "node_new",
			"typeName": "BranchNode",
			"nextNodeIds": ["c1", "c2"],
			"workflowRevisionId": "rev_5",
			"createdChildNodes": [
				{"id": "c1", "typeName": "AudienceFilter", "nextNodeIds": [], "appliesDownstream": false},
				{"id": "c2", "typeName": "AudienceFilter", "nextNodeIds": [], "appliesDownstream": false}
			]
		},
		"workflow": {"id":"wf_1","status":"Draft","workflowRevisionId":"rev_5","mailingListId":null,"rootNodeId":"r","nodes":{}}
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody = decodeBody(t, r)
		if want := "/workflows/wf_1/nodes"; r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(resp))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	res, err := client.CreateWorkflowNode("wf_1", CreateWorkflowNodeRequest{
		ExpectedRevisionID: ptr("rev_4"),
		InsertMode:         WorkflowInsertModeBetween,
		NodeTypeName:       CreateWorkflowNodeTypeBranchNode,
		FromNodeID:         "node_a",
		ToNodeID:           "node_b",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["insertMode"] != "between" || gotBody["fromNodeId"] != "node_a" || gotBody["toNodeId"] != "node_b" {
		t.Errorf("body = %+v", gotBody)
	}
	if gotBody["nodeTypeName"] != "BranchNode" {
		t.Errorf("nodeTypeName = %v", gotBody["nodeTypeName"])
	}
	if _, ok := gotBody["beforeNodeId"]; ok {
		t.Errorf("beforeNodeId should be absent for between: %+v", gotBody)
	}
	if res.Node.TypeName != WorkflowNodeTypeBranchNode || res.Node.BranchNode == nil {
		t.Errorf("node = %+v", res.Node)
	}
	if res.Node.WorkflowRevisionID != "rev_5" {
		t.Errorf("node revision = %q, want rev_5", res.Node.WorkflowRevisionID)
	}
	if len(res.Node.CreatedChildNodes) != 2 {
		t.Fatalf("len(CreatedChildNodes) = %d, want 2", len(res.Node.CreatedChildNodes))
	}
	if res.Node.CreatedChildNodes[0].TypeName != WorkflowNodeTypeAudienceFilter || res.Node.CreatedChildNodes[0].AudienceFilter == nil {
		t.Errorf("child[0] = %+v", res.Node.CreatedChildNodes[0])
	}
	if res.Workflow.ID != "wf_1" {
		t.Errorf("workflow.ID = %q", res.Workflow.ID)
	}
}

func TestCreateWorkflowNode_Before_NullRevision(t *testing.T) {
	var rawBody string
	var gotBody map[string]any
	resp := `{
		"node": {"id":"node_new","typeName":"TimerAction","nextNodeIds":[],"amount":0,"unit":"m","workflowRevisionId":"rev_1"},
		"workflow": {"id":"wf_1","status":"Draft","workflowRevisionId":"rev_1","mailingListId":null,"rootNodeId":"r","nodes":{}}
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(strings.Builder)
		tee := io.TeeReader(r.Body, buf)
		json.NewDecoder(tee).Decode(&gotBody)
		rawBody = buf.String()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(resp))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	res, err := client.CreateWorkflowNode("wf_1", CreateWorkflowNodeRequest{
		ExpectedRevisionID: nil,
		InsertMode:         WorkflowInsertModeBefore,
		NodeTypeName:       CreateWorkflowNodeTypeTimerAction,
		BeforeNodeID:       "node_target",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(rawBody, `"expectedRevisionId":null`) {
		t.Errorf("body must send explicit null revision: %s", rawBody)
	}
	if gotBody["insertMode"] != "before" || gotBody["beforeNodeId"] != "node_target" {
		t.Errorf("body = %+v", gotBody)
	}
	if _, ok := gotBody["fromNodeId"]; ok {
		t.Errorf("fromNodeId should be absent for before: %+v", gotBody)
	}
	if res.Node.TypeName != WorkflowNodeTypeTimerAction || res.Node.TimerAction == nil {
		t.Errorf("node = %+v", res.Node)
	}
	if res.Node.TimerAction.Unit != WorkflowTimerUnitMinutes {
		t.Errorf("unit = %q, want m", res.Node.TimerAction.Unit)
	}
}

func TestCreateWorkflowNode_Before_ToNodeID(t *testing.T) {
	var gotBody map[string]any
	resp := `{
		"node": {"id":"node_new","typeName":"TimerAction","nextNodeIds":[],"amount":0,"unit":"m","workflowRevisionId":"rev_1"},
		"workflow": {"id":"wf_1","status":"Draft","workflowRevisionId":"rev_1","mailingListId":null,"rootNodeId":"r","nodes":{}}
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody = decodeBody(t, r)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(resp))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	_, err := client.CreateWorkflowNode("wf_1", CreateWorkflowNodeRequest{
		ExpectedRevisionID: ptr("rev_0"),
		InsertMode:         WorkflowInsertModeBefore,
		NodeTypeName:       CreateWorkflowNodeTypeTimerAction,
		ToNodeID:           "node_target",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["insertMode"] != "before" || gotBody["toNodeId"] != "node_target" {
		t.Errorf("body = %+v", gotBody)
	}
	if _, ok := gotBody["beforeNodeId"]; ok {
		t.Errorf("beforeNodeId should be absent when ToNodeID is set: %+v", gotBody)
	}
	if _, ok := gotBody["fromNodeId"]; ok {
		t.Errorf("fromNodeId should be absent for before: %+v", gotBody)
	}
}

func TestCreateWorkflowNode_After(t *testing.T) {
	var gotBody map[string]any
	resp := `{
		"node": {"id":"node_new","typeName":"TimerAction","nextNodeIds":[],"amount":0,"unit":"m","workflowRevisionId":"rev_1"},
		"workflow": {"id":"wf_1","status":"Draft","workflowRevisionId":"rev_1","mailingListId":null,"rootNodeId":"r","nodes":{}}
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody = decodeBody(t, r)
		if want := "/workflows/wf_1/nodes"; r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(resp))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	res, err := client.CreateWorkflowNode("wf_1", CreateWorkflowNodeRequest{
		ExpectedRevisionID: ptr("rev_0"),
		InsertMode:         WorkflowInsertModeAfter,
		NodeTypeName:       CreateWorkflowNodeTypeTimerAction,
		FromNodeID:         "node_a",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["insertMode"] != "after" || gotBody["fromNodeId"] != "node_a" {
		t.Errorf("body = %+v", gotBody)
	}
	if _, ok := gotBody["toNodeId"]; ok {
		t.Errorf("toNodeId should be absent for after: %+v", gotBody)
	}
	if _, ok := gotBody["beforeNodeId"]; ok {
		t.Errorf("beforeNodeId should be absent for after: %+v", gotBody)
	}
	if res.Node.TypeName != WorkflowNodeTypeTimerAction {
		t.Errorf("node = %+v", res.Node)
	}
}

func TestRerouteNodeConnection(t *testing.T) {
	var gotBody map[string]any
	resp := `{
		"id": "node_a",
		"typeName": "SignupTrigger",
		"nextNodeIds": ["node_c"],
		"workflowRevisionId": "rev_31",
		"workflow": {"id":"wf_1","status":"Draft","workflowRevisionId":"rev_31","mailingListId":null,"rootNodeId":"node_a","nodes":{}}
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody = decodeBody(t, r)
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if want := "/workflows/wf_1/nodes/node_a/reroute"; r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(resp))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	res, err := client.RerouteNodeConnection("wf_1", "node_a", RerouteNodeConnectionRequest{
		ExpectedRevisionID: ptr("rev_30"),
		NewTargetNodeID:    "node_c",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["expectedRevisionId"] != "rev_30" || gotBody["newTargetNodeId"] != "node_c" {
		t.Errorf("body = %+v", gotBody)
	}
	if res.TypeName != WorkflowNodeTypeSignupTrigger {
		t.Errorf("node typeName = %q", res.TypeName)
	}
	if res.WorkflowRevisionID != "rev_31" {
		t.Errorf("node revision = %q, want rev_31", res.WorkflowRevisionID)
	}
	if res.Workflow.ID != "wf_1" {
		t.Errorf("workflow = %+v", res.Workflow)
	}
}

func TestUpdateWorkflowNode_Payloads(t *testing.T) {
	tests := []struct {
		name    string
		payload UpdateWorkflowNodePayload
		// wantSubstr are JSON fragments that must appear in the marshaled payload.
		wantSubstr []string
		// notSubstr are fragments that must NOT appear.
		notSubstr []string
	}{
		{
			name: "SignupTrigger has typeName",
			payload: UpdateWorkflowNodePayload{
				TypeName:      WorkflowNodeTypeSignupTrigger,
				SignupTrigger: &WorkflowSignupTriggerPayload{},
			},
			wantSubstr: []string{`"typeName":"SignupTrigger"`},
		},
		{
			name: "EventTrigger with null eventName",
			payload: UpdateWorkflowNodePayload{
				TypeName:     WorkflowNodeTypeEventTrigger,
				EventTrigger: &WorkflowEventTriggerPayload{EventName: nil, EventPatternID: ptr("ep_1"), ReEligible: ptr(true)},
			},
			wantSubstr: []string{`"typeName":"EventTrigger"`, `"eventPatternId":"ep_1"`, `"reEligible":true`},
		},
		{
			name: "ContactPropertyTrigger with query",
			payload: UpdateWorkflowNodePayload{
				TypeName: WorkflowNodeTypeContactPropertyTrigger,
				ContactPropertyTrigger: &WorkflowContactPropertyTriggerPayload{
					ContactPropertyQuery: &WorkflowContactPropertyQuery{
						Key: "plan",
						Is:  WorkflowContactPropertyComparison{Value: WorkflowContactPropertyValue{String: ptr("pro")}, Operator: "equal"},
					},
				},
			},
			wantSubstr: []string{`"typeName":"ContactPropertyTrigger"`, `"key":"plan"`},
		},
		{
			name: "AddToListTrigger",
			payload: UpdateWorkflowNodePayload{
				TypeName:         WorkflowNodeTypeAddToListTrigger,
				AddToListTrigger: &WorkflowAddToListTriggerPayload{ReEligible: ptr(false)},
			},
			wantSubstr: []string{`"typeName":"AddToListTrigger"`, `"reEligible":false`},
		},
		{
			name: "AudienceFilter has no typeName",
			payload: UpdateWorkflowNodePayload{
				TypeName: WorkflowNodeTypeAudienceFilter,
				AudienceFilter: &WorkflowAudienceFilterPayload{
					AudienceSegmentID: ptr("seg_1"),
					AppliesDownstream: ptr(true),
				},
			},
			wantSubstr: []string{`"audienceSegmentId":"seg_1"`, `"appliesDownstream":true`},
			notSubstr:  []string{`typeName`},
		},
		{
			name: "TimerAction has no typeName",
			payload: UpdateWorkflowNodePayload{
				TypeName:    WorkflowNodeTypeTimerAction,
				TimerAction: &WorkflowTimerActionPayload{Amount: ptr(3.0), Unit: WorkflowTimerUnitDays},
			},
			wantSubstr: []string{`"amount":3`, `"unit":"d"`},
			notSubstr:  []string{`typeName`},
		},
		{
			name: "ExperimentBranch has no typeName",
			payload: UpdateWorkflowNodePayload{
				TypeName:         WorkflowNodeTypeExperimentBranchNode,
				ExperimentBranch: &WorkflowExperimentBranchPayload{SamplingRate: ptr(50.0)},
			},
			wantSubstr: []string{`"samplingRate":50`},
			notSubstr:  []string{`typeName`},
		},
		{
			name: "Variant has no typeName",
			payload: UpdateWorkflowNodePayload{
				TypeName: WorkflowNodeTypeVariantNode,
				Variant:  &WorkflowVariantPayload{IsControl: ptr(true)},
			},
			wantSubstr: []string{`"isControl":true`},
			notSubstr:  []string{`typeName`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := json.Marshal(tt.payload)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			for _, want := range tt.wantSubstr {
				if !strings.Contains(string(raw), want) {
					t.Errorf("payload %s missing %q", raw, want)
				}
			}
			for _, no := range tt.notSubstr {
				if strings.Contains(string(raw), no) {
					t.Errorf("payload %s must not contain %q", raw, no)
				}
			}
		})
	}
}

func TestUpdateWorkflowNode_RequestAndResponse(t *testing.T) {
	var rawBody string
	var gotBody map[string]any
	resp := `{
		"id": "node_t",
		"typeName": "TimerAction",
		"nextNodeIds": ["n2"],
		"amount": 2,
		"unit": "h",
		"workflowRevisionId": "rev_9",
		"workflow": {"id":"wf_1","status":"Draft","workflowRevisionId":"rev_9","mailingListId":null,"rootNodeId":"r","nodes":{}}
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(strings.Builder)
		tee := io.TeeReader(r.Body, buf)
		json.NewDecoder(tee).Decode(&gotBody)
		rawBody = buf.String()
		if want := "/workflows/wf_1/nodes/node_t"; r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(resp))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	node, err := client.UpdateWorkflowNode("wf_1", "node_t", UpdateWorkflowNodeRequest{
		ExpectedRevisionID: ptr("rev_8"),
		Payload: UpdateWorkflowNodePayload{
			TypeName:    WorkflowNodeTypeTimerAction,
			TimerAction: &WorkflowTimerActionPayload{Amount: ptr(2.0), Unit: WorkflowTimerUnitHours},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["expectedRevisionId"] != "rev_8" {
		t.Errorf("expectedRevisionId = %v", gotBody["expectedRevisionId"])
	}
	if !strings.Contains(rawBody, `"payload":{`) {
		t.Errorf("payload missing from body: %s", rawBody)
	}
	if node.TypeName != WorkflowNodeTypeTimerAction || node.TimerAction == nil {
		t.Errorf("node = %+v", node)
	}
	if node.TimerAction.Amount != 2 || node.TimerAction.Unit != WorkflowTimerUnitHours {
		t.Errorf("timer = %+v", node.TimerAction)
	}
	if node.WorkflowRevisionID != "rev_9" {
		t.Errorf("revision = %q, want rev_9", node.WorkflowRevisionID)
	}
	if node.Workflow.ID != "wf_1" {
		t.Errorf("workflow = %+v", node.Workflow)
	}
	if node.Workflow.WorkflowRevisionID == nil || *node.Workflow.WorkflowRevisionID != "rev_9" {
		t.Errorf("workflow revision = %v, want rev_9", node.Workflow.WorkflowRevisionID)
	}
}

func TestAddWorkflowBranch(t *testing.T) {
	var gotBody map[string]any
	resp := `{
		"node": {"id":"node_b","typeName":"BranchNode","nextNodeIds":["c1","c2","c3"],"workflowRevisionId":"rev_11"},
		"workflow": {"id":"wf_1","status":"Draft","workflowRevisionId":"rev_11","mailingListId":null,"rootNodeId":"r","nodes":{}}
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody = decodeBody(t, r)
		if want := "/workflows/wf_1/nodes/node_b/add-branch"; r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(resp))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	res, err := client.AddWorkflowBranch("wf_1", "node_b", AddWorkflowBranchRequest{
		ExpectedRevisionID: ptr("rev_10"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["expectedRevisionId"] != "rev_10" {
		t.Errorf("body = %+v", gotBody)
	}
	if res.Node.TypeName != WorkflowNodeTypeBranchNode || res.Node.BranchNode == nil {
		t.Errorf("node = %+v", res.Node)
	}
	if len(res.Node.BranchNode.NextNodeIDs) != 3 {
		t.Errorf("NextNodeIDs = %v", res.Node.BranchNode.NextNodeIDs)
	}
	if res.Node.WorkflowRevisionID != "rev_11" {
		t.Errorf("revision = %q", res.Node.WorkflowRevisionID)
	}
	if res.Workflow.ID != "wf_1" {
		t.Errorf("workflow = %+v", res.Workflow)
	}
}

func TestDeleteWorkflowNode(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotBody = decodeBody(t, r)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"deleted","nodeIds":["node_x"],"workflowRevisionId":"rev_20","queuedContactCount":0,"workflow":{"id":"wf_1","status":"Draft","workflowRevisionId":"rev_20","mailingListId":null,"rootNodeId":"r","nodes":{}}}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	res, err := client.DeleteWorkflowNode("wf_1", "node_x", DeleteWorkflowNodeRequest{
		ExpectedRevisionID: ptr("rev_19"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/workflows/wf_1/nodes/node_x" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody["expectedRevisionId"] != "rev_19" {
		t.Errorf("body = %+v", gotBody)
	}
	if res.Status != WorkflowMutationStatusDeleted {
		t.Errorf("Status = %q, want deleted", res.Status)
	}
	if res.WorkflowRevisionID == nil || *res.WorkflowRevisionID != "rev_20" {
		t.Errorf("WorkflowRevisionID = %v, want rev_20", res.WorkflowRevisionID)
	}
	if len(res.NodeIDs) != 1 || res.NodeIDs[0] != "node_x" {
		t.Errorf("NodeIDs = %v", res.NodeIDs)
	}
	if res.Workflow == nil || res.Workflow.ID != "wf_1" {
		t.Errorf("Workflow = %+v, want id wf_1", res.Workflow)
	}
}

func TestDeleteWorkflowNodeRecursive_DryRun(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotBody = decodeBody(t, r)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"dryRun","nodeIds":["node_x","node_y"],"queuedContactCount":4}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	res, err := client.DeleteWorkflowNodeRecursive("wf_1", "node_x", DeleteWorkflowNodeRequest{
		ExpectedRevisionID:  ptr("rev_1"),
		DryRun:              true,
		QueuedContactPolicy: WorkflowQueuedContactPolicyFail,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/workflows/wf_1/nodes/node_x/recursive" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody["dryRun"] != true || gotBody["queuedContactPolicy"] != "fail" {
		t.Errorf("body = %+v", gotBody)
	}
	if res.Status != WorkflowMutationStatusDryRun {
		t.Errorf("Status = %q, want dryRun", res.Status)
	}
	if res.WorkflowRevisionID != nil {
		t.Errorf("WorkflowRevisionID = %v, want nil", res.WorkflowRevisionID)
	}
	if res.Workflow != nil {
		t.Errorf("Workflow = %+v, want nil on dry run", res.Workflow)
	}
	if len(res.NodeIDs) != 2 {
		t.Errorf("NodeIDs = %v", res.NodeIDs)
	}
}

func TestDeleteWorkflow(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotBody = decodeBody(t, r)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	err := client.DeleteWorkflow("wf_1", DeleteWorkflowRequest{
		ExpectedRevisionID: ptr("rev_1"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", gotMethod)
	}
	if gotPath != "/workflows/wf_1" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody["expectedRevisionId"] != "rev_1" {
		t.Errorf("body = %+v", gotBody)
	}
	if _, ok := gotBody["confirmDelete"]; ok {
		t.Errorf("confirmDelete must be omitted when false: %+v", gotBody)
	}
}

func TestDeleteWorkflow_NullRevision(t *testing.T) {
	var rawBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		rawBody = string(b)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	if err := client.DeleteWorkflow("wf_1", DeleteWorkflowRequest{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(rawBody, `"expectedRevisionId":null`) {
		t.Errorf("body must send explicit null expectedRevisionId when unset: %s", rawBody)
	}
}

func TestDeleteWorkflow_Confirmed(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody = decodeBody(t, r)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	err := client.DeleteWorkflow("wf_1", DeleteWorkflowRequest{
		ExpectedRevisionID: ptr("rev_1"),
		ConfirmDelete:      true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["confirmDelete"] != true {
		t.Errorf("body = %+v", gotBody)
	}
}

func TestDeleteWorkflow_ConfirmationRequired(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{
			name:    "sending",
			message: "This workflow is currently sending. Deleting it will stop sending it. Confirm deletion by sending a second request with confirmDelete: true.",
		},
		{
			name:    "queued contacts",
			message: "This workflow has 3 queued contacts. Deleting it will cancel those queued contacts. Confirm deletion by sending a second request with confirmDelete: true.",
		},
		{
			name:    "sending and queued contacts",
			message: "This workflow is currently sending and has 3 queued contacts. Deleting it will stop sending it and cancel those queued contacts. Confirm deletion by sending a second request with confirmDelete: true.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusConflict)
				w.Write([]byte(`{"message":` + strconv.Quote(tt.message) + `}`))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			err := client.DeleteWorkflow("wf_1", DeleteWorkflowRequest{ExpectedRevisionID: ptr("rev_1")})
			if !errors.Is(err, ErrWorkflowDeleteConfirmationRequired) {
				t.Fatalf("expected ErrWorkflowDeleteConfirmationRequired, got %v", err)
			}
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected a wrapped *APIError, got %T: %v", err, err)
			}
			if apiErr.StatusCode != http.StatusConflict {
				t.Errorf("StatusCode = %d, want 409", apiErr.StatusCode)
			}
			if apiErr.Message != tt.message {
				t.Errorf("Message = %q", apiErr.Message)
			}
		})
	}
}

func TestDeleteWorkflow_StaleRevision409(t *testing.T) {
	const message = "workflowRevisionId mismatch. Latest workflowRevisionId is clx7a3b5c7d9e1f3g5h7i9j1. Refetch and retry."
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"message":` + strconv.Quote(message) + `}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	err := client.DeleteWorkflow("wf_1", DeleteWorkflowRequest{ExpectedRevisionID: ptr("old_rev")})
	if errors.Is(err, ErrWorkflowDeleteConfirmationRequired) {
		t.Fatal("a stale revision must not be reported as requiring confirmation")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusConflict {
		t.Errorf("StatusCode = %d, want 409", apiErr.StatusCode)
	}
	if apiErr.Message != message {
		t.Errorf("Message = %q", apiErr.Message)
	}
}

func TestDeleteWorkflow_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Workflow not found."}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	err := client.DeleteWorkflow("wf_missing", DeleteWorkflowRequest{ExpectedRevisionID: ptr("rev_1")})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want 404", apiErr.StatusCode)
	}
	if apiErr.Message != "Workflow not found." {
		t.Errorf("Message = %q", apiErr.Message)
	}
}

func TestWorkflowMutationNode_UnmarshalVariants(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"AddToListTrigger", `{"id":"n","typeName":"AddToListTrigger","nextNodeIds":[],"mailingListId":"ml_1","reEligible":true}`, WorkflowNodeTypeAddToListTrigger},
		{"AudienceFilter", `{"id":"n","typeName":"AudienceFilter","nextNodeIds":[],"appliesDownstream":true}`, WorkflowNodeTypeAudienceFilter},
		{"Variant", `{"id":"n","typeName":"VariantNode","nextNodeIds":[],"isControl":true}`, WorkflowNodeTypeVariantNode},
		{"ExperimentBranch", `{"id":"n","typeName":"ExperimentBranchNode","nextNodeIds":[],"samplingRate":25}`, WorkflowNodeTypeExperimentBranchNode},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var n WorkflowMutationNode
			if err := json.Unmarshal([]byte(tt.body), &n); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if n.TypeName != tt.want {
				t.Errorf("TypeName = %q, want %q", n.TypeName, tt.want)
			}
		})
	}

	var af WorkflowMutationNode
	if err := json.Unmarshal([]byte(`{"id":"n","typeName":"AddToListTrigger","nextNodeIds":[],"mailingListId":"ml_1","reEligible":true}`), &af); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if af.AddToListTrigger == nil || af.AddToListTrigger.MailingListID == nil || *af.AddToListTrigger.MailingListID != "ml_1" {
		t.Errorf("AddToListTrigger = %+v", af.AddToListTrigger)
	}
}

func TestWorkflowContactPropertyValue_RoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		value WorkflowContactPropertyValue
		want  string
	}{
		{"string", WorkflowContactPropertyValue{String: ptr("pro")}, `"pro"`},
		{"number", WorkflowContactPropertyValue{Number: ptr(5.5)}, `5.5`},
		{"bool true", WorkflowContactPropertyValue{Bool: ptr(true)}, `true`},
		{"bool false", WorkflowContactPropertyValue{Bool: ptr(false)}, `false`},
		{"null", WorkflowContactPropertyValue{}, `null`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := json.Marshal(tt.value)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(raw) != tt.want {
				t.Errorf("Marshal = %s, want %s", raw, tt.want)
			}
			var round WorkflowContactPropertyValue
			if err := json.Unmarshal(raw, &round); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
		})
	}
}

func TestUpdateWorkflowNodePayload_NilVariantErrors(t *testing.T) {
	// A selected config variant with a nil pointer must error, not silently
	// emit "null" — symmetric with the trigger variants.
	for _, p := range []UpdateWorkflowNodePayload{
		{TypeName: WorkflowNodeTypeAudienceFilter},
		{TypeName: WorkflowNodeTypeTimerAction},
		{TypeName: WorkflowNodeTypeExperimentBranchNode},
		{TypeName: WorkflowNodeTypeVariantNode},
	} {
		if _, err := json.Marshal(p); err == nil {
			t.Errorf("%s: expected error marshaling nil variant, got none", p.TypeName)
		}
	}
}

func TestCreateWorkflow_NullMailingList(t *testing.T) {
	var rawBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		rawBody = string(b)
		w.Write([]byte(`{"id":"wf_1","status":"Draft","workflowRevisionId":"rev_1","mailingListId":null,"rootNodeId":"r","nodes":{}}`))
	}))
	defer server.Close()

	client := NewClient("key", WithBaseURL(server.URL))
	if _, err := client.CreateWorkflow(CreateWorkflowRequest{Name: "New flow"}); err != nil {
		t.Fatalf("CreateWorkflow: %v", err)
	}
	if !strings.Contains(rawBody, `"mailingListId":null`) {
		t.Errorf("body must send explicit null mailingListId when unset: %s", rawBody)
	}
}

func TestWorkflowNodeWithRevision_MarshalKeepsRevision(t *testing.T) {
	n := WorkflowNodeWithRevision{
		WorkflowNode: WorkflowNode{
			TypeName:    WorkflowNodeTypeTimerAction,
			TimerAction: &TimerActionWorkflowNode{ID: "n1", WorkflowID: "wf_1", NextNodeIDs: []string{}, Amount: 5, Unit: WorkflowTimerUnitHours},
		},
		WorkflowRevisionID: ptr("rev_9"),
	}
	raw, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"workflowRevisionId":"rev_9"`) {
		t.Errorf("marshal dropped workflowRevisionId: %s", raw)
	}
	if !strings.Contains(string(raw), `"typeName":"TimerAction"`) {
		t.Errorf("marshal dropped node fields: %s", raw)
	}
}

func TestWorkflowMutationNodeWithRevision_MarshalKeepsRevision(t *testing.T) {
	const in = `{"id":"n","typeName":"AddToListTrigger","nextNodeIds":[],"mailingListId":"ml_1","reEligible":true,"workflowRevisionId":"rev_7"}`
	var n WorkflowMutationNodeWithRevision
	if err := json.Unmarshal([]byte(in), &n); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	raw, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"workflowRevisionId":"rev_7"`) {
		t.Errorf("marshal dropped workflowRevisionId: %s", raw)
	}
	if !strings.Contains(string(raw), `"typeName":"AddToListTrigger"`) {
		t.Errorf("marshal dropped node fields: %s", raw)
	}
}

func TestCreatedWorkflowNode_MarshalKeepsRevisionAndChildren(t *testing.T) {
	const in = `{"id":"n","typeName":"AddToListTrigger","nextNodeIds":[],"mailingListId":"ml_1","reEligible":true,"workflowRevisionId":"rev_7","createdChildNodes":[{"id":"c","typeName":"AddToListTrigger","nextNodeIds":[],"mailingListId":"ml_2","reEligible":false}]}`
	var n CreatedWorkflowNode
	if err := json.Unmarshal([]byte(in), &n); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	raw, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"workflowRevisionId":"rev_7"`) {
		t.Errorf("marshal dropped workflowRevisionId: %s", raw)
	}
	if !strings.Contains(string(raw), `"createdChildNodes"`) {
		t.Errorf("marshal dropped createdChildNodes: %s", raw)
	}
}

func TestGetWorkflowNode_AddToListTrigger(t *testing.T) {
	body := `{
		"id": "node_a",
		"workflowId": "wf_1",
		"typeName": "AddToListTrigger",
		"nextNodeIds": ["node_next"],
		"mailingListId": "ml_1",
		"reEligible": true
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	node, err := client.GetWorkflowNode("wf_1", "node_a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.AddToListTrigger == nil {
		t.Fatal("AddToListTrigger is nil")
	}
	if node.AddToListTrigger.MailingListID == nil || *node.AddToListTrigger.MailingListID != "ml_1" {
		t.Errorf("MailingListID = %v, want ml_1", node.AddToListTrigger.MailingListID)
	}
}

func TestGetWorkflowNode_SendEmailAction(t *testing.T) {
	body := `{
		"id": "node_s",
		"workflowId": "wf_1",
		"typeName": "SendEmailAction",
		"nextNodeIds": ["node_next"],
		"emailMessageId": "em_1",
		"subject": "Welcome"
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	node, err := client.GetWorkflowNode("wf_1", "node_s")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.SendEmailAction == nil {
		t.Fatal("SendEmailAction is nil")
	}
	if node.SendEmailAction.EmailMessageID != "em_1" {
		t.Errorf("EmailMessageID = %q, want em_1", node.SendEmailAction.EmailMessageID)
	}
	if node.SendEmailAction.Subject != "Welcome" {
		t.Errorf("Subject = %q, want Welcome", node.SendEmailAction.Subject)
	}
}

func TestSimplifiedAddToListNode_DecodesMailingListID(t *testing.T) {
	const in = `{"typeName":"AddToListTrigger","nextNodeIds":[],"mailingListId":"ml_1","reEligible":true}`
	var n SimplifiedWorkflowNode
	if err := json.Unmarshal([]byte(in), &n); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if n.AddToListTrigger == nil {
		t.Fatal("AddToListTrigger is nil")
	}
	if n.AddToListTrigger.MailingListID == nil || *n.AddToListTrigger.MailingListID != "ml_1" {
		t.Errorf("MailingListID = %v, want ml_1", n.AddToListTrigger.MailingListID)
	}
}

func TestMarshalDiscriminated_TypedNilVariantErrors(t *testing.T) {
	// A node with the discriminator set but the variant pointer nil must return
	// an error, not panic on a nil-map write.
	if _, err := json.Marshal(WorkflowNode{TypeName: WorkflowNodeTypeTimerAction}); err == nil {
		t.Fatal("expected error marshaling a node with a nil variant, got none")
	}
}

func TestGetWorkflowNodeMetrics(t *testing.T) {
	tests := []struct {
		name       string
		workflowID string
		nodeID     string
		statusCode int
		body       string
		want       EmailMetrics
		wantAPIErr *APIError
		wantErrMsg string
	}{
		{
			name:       "success",
			workflowID: "wf_1",
			nodeID:     "node_s",
			statusCode: http.StatusOK,
			body:       `{"sends":4210,"opens":1922,"clicks":301,"unsubscribes":12,"spamReports":1,"hardBounces":9,"softBounces":13}`,
			want: EmailMetrics{
				Sends:        4210,
				Opens:        1922,
				Clicks:       301,
				Unsubscribes: 12,
				SpamReports:  1,
				HardBounces:  9,
				SoftBounces:  13,
			},
		},
		{
			name:       "never sent reports zero counters",
			workflowID: "wf_1",
			nodeID:     "node_new",
			statusCode: http.StatusOK,
			body:       `{"sends":0,"opens":0,"clicks":0,"unsubscribes":0,"spamReports":0,"hardBounces":0,"softBounces":0}`,
			want:       EmailMetrics{},
		},
		{
			name:       "not found",
			workflowID: "wf_missing",
			nodeID:     "node_s",
			statusCode: http.StatusNotFound,
			body:       `{"message":"Workflow not found."}`,
			wantAPIErr: &APIError{StatusCode: http.StatusNotFound, Message: "Workflow not found."},
		},
		{
			name:       "not a SendEmailAction node",
			workflowID: "wf_1",
			nodeID:     "node_timer",
			statusCode: http.StatusBadRequest,
			body:       `{"message":"Node is not a SendEmailAction node."}`,
			wantAPIErr: &APIError{StatusCode: http.StatusBadRequest, Message: "Node is not a SendEmailAction node."},
		},
		{
			name:       "invalid json",
			workflowID: "wf_1",
			nodeID:     "node_s",
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
			result, err := client.GetWorkflowNodeMetrics(tt.workflowID, tt.nodeID)

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
			if wantPath := "/workflows/" + tt.workflowID + "/nodes/" + tt.nodeID + "/metrics"; gotPath != wantPath {
				t.Errorf("path = %q, want %q", gotPath, wantPath)
			}
			if *result != tt.want {
				t.Errorf("metrics = %+v, want %+v", *result, tt.want)
			}
		})
	}
}
