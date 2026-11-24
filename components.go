package loops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// Component is a reusable LMX snippet that can be included from other email
// messages. It is identified by ID and has an LMX body.
type Component struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	LMX  string `json:"lmx"`
}

// CreateComponentRequest is the request body for [Client.CreateComponent].
// Both fields are required.
type CreateComponentRequest struct {
	Name string `json:"name"`
	LMX  string `json:"lmx"`
}

// UpdateComponentRequest is the request body for [Client.UpdateComponent].
// At least one field must be set.
type UpdateComponentRequest struct {
	Name string `json:"name,omitempty"`
	LMX  string `json:"lmx,omitempty"`
}

// UpdateComponentResult is the result of [Client.UpdateComponent]. It embeds
// the updated [Component] and adds AffectedEmailCount, the number of emails
// using this component that were updated by the body change (0 when only the
// name changed).
type UpdateComponentResult struct {
	Component
	AffectedEmailCount int `json:"affectedEmailCount"`
}

// CreateComponent creates a new component.
func (c *Client) CreateComponent(ctx context.Context, req CreateComponentRequest) (*Component, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(ctx, http.MethodPost, "/components", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}

	resp, err := c.do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, errorFromResponse(resp)
	}

	var result Component
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// UpdateComponent updates the component identified by id.
func (c *Client) UpdateComponent(ctx context.Context, id string, req UpdateComponentRequest) (*UpdateComponentResult, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(ctx, http.MethodPost, "/components/"+id, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}

	resp, err := c.do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errorFromResponse(resp)
	}

	var result UpdateComponentResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetComponent returns the component identified by id.
func (c *Client) GetComponent(ctx context.Context, id string) (*Component, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/components/"+id, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errorFromResponse(resp)
	}

	var result Component
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ListComponents returns a single page of components along with pagination
// information. To iterate every page, use [Paginate].
func (c *Client) ListComponents(ctx context.Context, params PaginationParams) ([]Component, *Pagination, error) {
	q := url.Values{}
	if params.PerPage != "" {
		q.Set("perPage", params.PerPage)
	}
	if params.Cursor != "" {
		q.Set("cursor", params.Cursor)
	}

	path := "/components"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}

	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, nil, err
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, errorFromResponse(resp)
	}

	var result struct {
		Pagination Pagination  `json:"pagination"`
		Data       []Component `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Data, &result.Pagination, nil
}
