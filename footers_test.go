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

type footerRequest struct {
	method string
	path   string
	query  string
	body   string
}

func newFooterServer(t *testing.T, statusCode int, body string) (*Client, *footerRequest) {
	t.Helper()
	got := &footerRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got.method = r.Method
		got.path = r.URL.Path
		got.query = r.URL.RawQuery
		got.body = string(b)
		w.WriteHeader(statusCode)
		w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return NewClient("test-key", WithBaseURL(server.URL)), got
}

func assertFooterRequest(t *testing.T, got *footerRequest, method, path, body string) {
	t.Helper()
	if got.method != method {
		t.Errorf("method = %q, want %q", got.method, method)
	}
	if got.path != path {
		t.Errorf("path = %q, want %q", got.path, path)
	}
	if body == "" {
		if got.body != "" {
			t.Errorf("body = %q, want empty", got.body)
		}
		return
	}
	var gotBody, wantBody any
	if err := json.Unmarshal([]byte(got.body), &gotBody); err != nil {
		t.Fatalf("request body is not JSON: %q", got.body)
	}
	if err := json.Unmarshal([]byte(body), &wantBody); err != nil {
		t.Fatalf("want body is not JSON: %q", body)
	}
	gotJSON, _ := json.Marshal(gotBody)
	wantJSON, _ := json.Marshal(wantBody)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("body = %s, want %s", gotJSON, wantJSON)
	}
}

func assertFooterWithVariants(t *testing.T, f FooterWithVariants) {
	t.Helper()
	want := Footer{
		ID:        "ftr_1",
		Name:      "Main footer",
		IsDefault: true,
		Placement: FooterPlacementInside,
		CreatedAt: "2026-10-27T10:00:00.000Z",
		UpdatedAt: "2026-11-02T08:30:00.000Z",
	}
	if f.Footer != want {
		t.Errorf("Footer = %+v, want %+v", f.Footer, want)
	}
	if len(f.Variants) != 1 {
		t.Fatalf("len(Variants) = %d, want 1", len(f.Variants))
	}
	assertFooterVariant(t, f.Variants[0])
}

func assertFooterVariant(t *testing.T, v FooterVariant) {
	t.Helper()
	if v.ID != "var_1" || v.FooterID != "ftr_1" || v.ContentRevisionID != "rev_1" {
		t.Errorf("ID, FooterID, ContentRevisionID = %q, %q, %q", v.ID, v.FooterID, v.ContentRevisionID)
	}
	if v.DraftLMX != "<Paragraph>Draft</Paragraph>" {
		t.Errorf("DraftLMX = %q", v.DraftLMX)
	}
	if v.PublishedLMX == nil || *v.PublishedLMX != "<Paragraph>Published</Paragraph>" {
		t.Errorf("PublishedLMX = %v", v.PublishedLMX)
	}
	if v.PublishedAt == nil || *v.PublishedAt != "2026-11-02T08:30:00.000Z" {
		t.Errorf("PublishedAt = %v", v.PublishedAt)
	}
	if v.CreatedAt != "2026-10-27T10:00:00.000Z" || v.UpdatedAt != "2026-11-02T08:30:00.000Z" {
		t.Errorf("CreatedAt, UpdatedAt = %q, %q", v.CreatedAt, v.UpdatedAt)
	}
}

func TestListFooters(t *testing.T) {
	client, got := newFooterServer(t, http.StatusOK, listFootersResponse)

	footers, page, err := client.ListFooters(PaginationParams{PerPage: "20", Cursor: "cur_1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertFooterRequest(t, got, http.MethodGet, "/footers", "")
	if got.query != "cursor=cur_1&perPage=20" {
		t.Errorf("query = %q, want cursor=cur_1&perPage=20", got.query)
	}
	if len(footers) != 2 {
		t.Fatalf("len(footers) = %d, want 2", len(footers))
	}
	if footers[0].ID != "ftr_1" || !footers[0].IsDefault || footers[0].Placement != FooterPlacementInside {
		t.Errorf("footers[0] = %+v", footers[0])
	}
	if footers[1].ID != "ftr_2" || footers[1].IsDefault || footers[1].Placement != FooterPlacementBelow {
		t.Errorf("footers[1] = %+v", footers[1])
	}
	if page.TotalResults != 2 || page.NextCursor != "" {
		t.Errorf("pagination = %+v", page)
	}
}

func TestListFooters_NoParams(t *testing.T) {
	client, got := newFooterServer(t, http.StatusOK, listFootersResponse)

	if _, _, err := client.ListFooters(PaginationParams{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.query != "" {
		t.Errorf("query = %q, want empty", got.query)
	}
}

func TestCreateFooter(t *testing.T) {
	tests := []struct {
		name     string
		req      CreateFooterRequest
		wantBody string
	}{
		{name: "with name", req: CreateFooterRequest{Name: "Main footer"}, wantBody: `{"name":"Main footer"}`},
		{name: "without name", req: CreateFooterRequest{}, wantBody: `{}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, got := newFooterServer(t, http.StatusCreated, footerWithVariantsResponse)

			result, err := client.CreateFooter(tt.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertFooterRequest(t, got, http.MethodPost, "/footers", tt.wantBody)
			assertFooterWithVariants(t, *result)
		})
	}
}

func TestCreateFooter_RejectsOK(t *testing.T) {
	client, _ := newFooterServer(t, http.StatusOK, footerWithVariantsResponse)

	var apiErr *APIError
	_, err := client.CreateFooter(CreateFooterRequest{})
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusOK)
	}
}

func TestGetFooter(t *testing.T) {
	client, got := newFooterServer(t, http.StatusOK, footerWithVariantsResponse)

	result, err := client.GetFooter("ftr_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertFooterRequest(t, got, http.MethodGet, "/footers/ftr_1", "")
	assertFooterWithVariants(t, *result)
}

func TestUpdateFooter(t *testing.T) {
	tests := []struct {
		name     string
		req      UpdateFooterRequest
		wantBody string
	}{
		{
			name:     "name and placement",
			req:      UpdateFooterRequest{Name: "Marketing footer", Placement: FooterPlacementBelow},
			wantBody: `{"name":"Marketing footer","placement":"below"}`,
		},
		{
			name:     "name only",
			req:      UpdateFooterRequest{Name: "Marketing footer"},
			wantBody: `{"name":"Marketing footer"}`,
		},
		{
			name:     "placement only",
			req:      UpdateFooterRequest{Placement: FooterPlacementInside},
			wantBody: `{"placement":"inside"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, got := newFooterServer(t, http.StatusOK, footerWithVariantsResponse)

			result, err := client.UpdateFooter("ftr_1", tt.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertFooterRequest(t, got, http.MethodPost, "/footers/ftr_1", tt.wantBody)
			assertFooterWithVariants(t, *result)
		})
	}
}

func TestDeleteFooter(t *testing.T) {
	client, got := newFooterServer(t, http.StatusNoContent, "")

	if err := client.DeleteFooter("ftr_1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertFooterRequest(t, got, http.MethodDelete, "/footers/ftr_1", "")
}

func TestSetDefaultFooter(t *testing.T) {
	client, got := newFooterServer(t, http.StatusOK, setDefaultFooterResponse)

	result, err := client.SetDefaultFooter("ftr_1", SetDefaultFooterRequest{ExistingEmails: FooterExistingEmailsPreviousDefault})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertFooterRequest(t, got, http.MethodPost, "/footers/ftr_1/set-default", `{"existingEmails":"previousDefault"}`)
	assertFooterWithVariants(t, result.FooterWithVariants)
	if result.UpdatedEmailCount != 12 {
		t.Errorf("UpdatedEmailCount = %d, want 12", result.UpdatedEmailCount)
	}
}

func TestGetFooterVariant(t *testing.T) {
	client, got := newFooterServer(t, http.StatusOK, footerVariantResponse)

	result, err := client.GetFooterVariant("ftr_1", "var_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertFooterRequest(t, got, http.MethodGet, "/footers/ftr_1/variants/var_1", "")
	assertFooterVariant(t, *result)
}

func TestGetFooterVariant_Unpublished(t *testing.T) {
	client, _ := newFooterServer(t, http.StatusOK, unpublishedFooterVariantResponse)

	result, err := client.GetFooterVariant("ftr_1", "var_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PublishedLMX != nil {
		t.Errorf("PublishedLMX = %q, want nil", *result.PublishedLMX)
	}
	if result.PublishedAt != nil {
		t.Errorf("PublishedAt = %q, want nil", *result.PublishedAt)
	}
}

func TestUpdateFooterVariant(t *testing.T) {
	client, got := newFooterServer(t, http.StatusOK, footerVariantResponse)

	result, err := client.UpdateFooterVariant("ftr_1", "var_1", UpdateFooterVariantRequest{
		ExpectedRevisionID: "rev_0",
		LMX:                "<Paragraph>Draft</Paragraph>",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertFooterRequest(t, got, http.MethodPost, "/footers/ftr_1/variants/var_1",
		`{"expectedRevisionId":"rev_0","lmx":"<Paragraph>Draft</Paragraph>"}`)
	assertFooterVariant(t, *result)
}

func TestPublishFooterVariant(t *testing.T) {
	client, got := newFooterServer(t, http.StatusOK, footerVariantResponse)

	result, err := client.PublishFooterVariant("ftr_1", "var_1", PublishFooterVariantRequest{ExpectedRevisionID: "rev_1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertFooterRequest(t, got, http.MethodPost, "/footers/ftr_1/variants/var_1/publish", `{"expectedRevisionId":"rev_1"}`)
	assertFooterVariant(t, *result)
}

func TestFooters_Errors(t *testing.T) {
	ops := []struct {
		name          string
		successStatus int
		call          func(c *Client) error
	}{
		{"ListFooters", http.StatusOK, func(c *Client) error {
			_, _, err := c.ListFooters(PaginationParams{})
			return err
		}},
		{"CreateFooter", http.StatusCreated, func(c *Client) error {
			_, err := c.CreateFooter(CreateFooterRequest{})
			return err
		}},
		{"GetFooter", http.StatusOK, func(c *Client) error {
			_, err := c.GetFooter("ftr_1")
			return err
		}},
		{"UpdateFooter", http.StatusOK, func(c *Client) error {
			_, err := c.UpdateFooter("ftr_1", UpdateFooterRequest{Name: "n"})
			return err
		}},
		{"DeleteFooter", http.StatusNoContent, func(c *Client) error {
			return c.DeleteFooter("ftr_1")
		}},
		{"SetDefaultFooter", http.StatusOK, func(c *Client) error {
			_, err := c.SetDefaultFooter("ftr_1", SetDefaultFooterRequest{ExistingEmails: FooterExistingEmailsAll})
			return err
		}},
		{"GetFooterVariant", http.StatusOK, func(c *Client) error {
			_, err := c.GetFooterVariant("ftr_1", "var_1")
			return err
		}},
		{"UpdateFooterVariant", http.StatusOK, func(c *Client) error {
			_, err := c.UpdateFooterVariant("ftr_1", "var_1", UpdateFooterVariantRequest{})
			return err
		}},
		{"PublishFooterVariant", http.StatusOK, func(c *Client) error {
			_, err := c.PublishFooterVariant("ftr_1", "var_1", PublishFooterVariantRequest{})
			return err
		}},
	}

	for _, op := range ops {
		name, call := op.name, op.call
		t.Run(name+"/api error", func(t *testing.T) {
			client, _ := newFooterServer(t, http.StatusConflict,
				`{"message":"3 emails use the footer. Change their footer before you delete it.","emailCount":3}`)

			var apiErr *APIError
			err := call(client)
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected *APIError, got %T: %v", err, err)
			}
			if apiErr.StatusCode != http.StatusConflict {
				t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusConflict)
			}
			if want := "3 emails use the footer. Change their footer before you delete it."; apiErr.Message != want {
				t.Errorf("Message = %q, want %q", apiErr.Message, want)
			}
		})

		if op.successStatus == http.StatusNoContent {
			continue
		}
		t.Run(name+"/invalid json", func(t *testing.T) {
			client, _ := newFooterServer(t, op.successStatus, `not json`)

			err := call(client)
			if err == nil || !strings.Contains(err.Error(), "failed to decode response") {
				t.Errorf("error = %v, want it to contain %q", err, "failed to decode response")
			}
		})
	}
}
