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

const groupResponse = `{
	"id": "grp_abc123",
	"name": "Newsletters",
	"description": "Weekly newsletter campaigns",
	"createdAt": "2026-04-01T10:00:00Z",
	"updatedAt": "2026-04-02T10:00:00Z"
}`

const listGroupsResponseBody = `{
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
			"id": "grp_1",
			"name": "Newsletters",
			"description": "",
			"createdAt": "2026-04-01T10:00:00Z",
			"updatedAt": "2026-04-02T10:00:00Z"
		},
		{
			"id": "grp_2",
			"name": "Product updates",
			"description": "Release notes",
			"createdAt": "2026-03-01T10:00:00Z",
			"updatedAt": "2026-03-02T10:00:00Z"
		}
	]
}`

type groupOps struct {
	name     string
	basePath string
	create   func(c *Client, req CreateGroupRequest) (*Group, error)
	get      func(c *Client, id string) (*Group, error)
	update   func(c *Client, id string, req UpdateGroupRequest) (*Group, error)
	list     func(c *Client, params PaginationParams) ([]Group, *Pagination, error)
	createOK int
}

func allGroupOps() []groupOps {
	return []groupOps{
		{
			name:     "campaign",
			basePath: "/campaign-groups",
			create:   func(c *Client, req CreateGroupRequest) (*Group, error) { return c.CreateCampaignGroup(req) },
			get:      func(c *Client, id string) (*Group, error) { return c.GetCampaignGroup(id) },
			update: func(c *Client, id string, req UpdateGroupRequest) (*Group, error) {
				return c.UpdateCampaignGroup(id, req)
			},
			list: func(c *Client, params PaginationParams) ([]Group, *Pagination, error) {
				return c.ListCampaignGroups(params)
			},
			createOK: http.StatusOK,
		},
		{
			name:     "transactional",
			basePath: "/transactional-groups",
			create:   func(c *Client, req CreateGroupRequest) (*Group, error) { return c.CreateTransactionalGroup(req) },
			get:      func(c *Client, id string) (*Group, error) { return c.GetTransactionalGroup(id) },
			update: func(c *Client, id string, req UpdateGroupRequest) (*Group, error) {
				return c.UpdateTransactionalGroup(id, req)
			},
			list: func(c *Client, params PaginationParams) ([]Group, *Pagination, error) {
				return c.ListTransactionalGroups(params)
			},
			createOK: http.StatusOK,
		},
		{
			name:     "workflow",
			basePath: "/workflow-groups",
			create:   func(c *Client, req CreateGroupRequest) (*Group, error) { return c.CreateWorkflowGroup(req) },
			get:      func(c *Client, id string) (*Group, error) { return c.GetWorkflowGroup(id) },
			update: func(c *Client, id string, req UpdateGroupRequest) (*Group, error) {
				return c.UpdateWorkflowGroup(id, req)
			},
			list: func(c *Client, params PaginationParams) ([]Group, *Pagination, error) {
				return c.ListWorkflowGroups(params)
			},
			createOK: http.StatusOK,
		},
	}
}

func TestGroups_CreateAndGet(t *testing.T) {
	for _, ops := range allGroupOps() {
		t.Run(ops.name+"/create", func(t *testing.T) {
			var gotPath, gotMethod, gotBody string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotMethod = r.Method
				b, _ := io.ReadAll(r.Body)
				gotBody = string(b)
				w.WriteHeader(ops.createOK)
				w.Write([]byte(groupResponse))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			result, err := ops.create(client, CreateGroupRequest{Name: "Newsletters", Description: "Weekly newsletter campaigns"})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotMethod != http.MethodPost {
				t.Errorf("method = %q, want POST", gotMethod)
			}
			if gotPath != ops.basePath {
				t.Errorf("path = %q, want %q", gotPath, ops.basePath)
			}
			var sent map[string]string
			if err := json.Unmarshal([]byte(gotBody), &sent); err != nil {
				t.Fatalf("body unmarshal: %v", err)
			}
			if sent["name"] != "Newsletters" || sent["description"] != "Weekly newsletter campaigns" {
				t.Errorf("body = %v", sent)
			}
			if result.ID != "grp_abc123" {
				t.Errorf("ID = %q, want grp_abc123", result.ID)
			}
		})

		t.Run(ops.name+"/get", func(t *testing.T) {
			var gotPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(groupResponse))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			result, err := ops.get(client, "grp_abc123")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if want := ops.basePath + "/grp_abc123"; gotPath != want {
				t.Errorf("path = %q, want %q", gotPath, want)
			}
			if result.Name != "Newsletters" {
				t.Errorf("Name = %q", result.Name)
			}
		})

		t.Run(ops.name+"/update", func(t *testing.T) {
			var gotPath, gotBody string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				b, _ := io.ReadAll(r.Body)
				gotBody = string(b)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(groupResponse))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			if _, err := ops.update(client, "grp_abc123", UpdateGroupRequest{Description: "Updated"}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if want := ops.basePath + "/grp_abc123"; gotPath != want {
				t.Errorf("path = %q, want %q", gotPath, want)
			}
			if !strings.Contains(gotBody, `"description":"Updated"`) {
				t.Errorf("body = %q", gotBody)
			}
			if strings.Contains(gotBody, `"name"`) {
				t.Errorf("body should omit empty name: %q", gotBody)
			}
		})

		t.Run(ops.name+"/list", func(t *testing.T) {
			var gotPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(listGroupsResponseBody))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			groups, pagination, err := ops.list(client, PaginationParams{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotPath != ops.basePath {
				t.Errorf("path = %q, want %q", gotPath, ops.basePath)
			}
			if len(groups) != 2 {
				t.Errorf("len(groups) = %d, want 2", len(groups))
			}
			if pagination == nil {
				t.Fatal("pagination is nil")
			}
		})

		t.Run(ops.name+"/create_reserved_name", func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"message":"\"Unsorted\" is a reserved group name"}`))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			_, err := ops.create(client, CreateGroupRequest{Name: "Unsorted"})
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected *APIError, got %T: %v", err, err)
			}
			if apiErr.StatusCode != http.StatusBadRequest {
				t.Errorf("StatusCode = %d, want 400", apiErr.StatusCode)
			}
		})
	}
}

func TestGroups_ListQueryParams(t *testing.T) {
	var gotPerPage, gotCursor string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPerPage = r.URL.Query().Get("perPage")
		gotCursor = r.URL.Query().Get("cursor")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"pagination":{},"data":[]}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	if _, _, err := client.ListCampaignGroups(PaginationParams{PerPage: "10", Cursor: "xyz"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPerPage != "10" || gotCursor != "xyz" {
		t.Errorf("perPage=%q cursor=%q", gotPerPage, gotCursor)
	}
}
