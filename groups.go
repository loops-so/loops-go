package loops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// Group is a campaign, transactional-email, or workflow group. Each team has
// a reserved "Unsorted" group that cannot be renamed or edited.
type Group struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// CreateGroupRequest is the request body for [Client.CreateCampaignGroup],
// [Client.CreateTransactionalGroup], and [Client.CreateWorkflowGroup]. Name
// cannot be the reserved value "Unsorted".
type CreateGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// UpdateGroupRequest is the request body for [Client.UpdateCampaignGroup],
// [Client.UpdateTransactionalGroup], and [Client.UpdateWorkflowGroup]. At
// least one field must be set.
type UpdateGroupRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// CreateCampaignGroup creates a new campaign group.
func (c *Client) CreateCampaignGroup(ctx context.Context, req CreateGroupRequest) (*Group, error) {
	return c.createGroup(ctx, "/campaign-groups", req)
}

// GetCampaignGroup returns the campaign group identified by id.
func (c *Client) GetCampaignGroup(ctx context.Context, id string) (*Group, error) {
	return c.getGroup(ctx, "/campaign-groups/"+id)
}

// UpdateCampaignGroup updates the campaign group identified by id.
func (c *Client) UpdateCampaignGroup(ctx context.Context, id string, req UpdateGroupRequest) (*Group, error) {
	return c.updateGroup(ctx, "/campaign-groups/"+id, req)
}

// ListCampaignGroups returns a single page of campaign groups along with
// pagination information. To iterate every page, use [Paginate].
func (c *Client) ListCampaignGroups(ctx context.Context, params PaginationParams) ([]Group, *Pagination, error) {
	return c.listGroups(ctx, "/campaign-groups", params)
}

// CreateTransactionalGroup creates a new transactional-email group.
func (c *Client) CreateTransactionalGroup(ctx context.Context, req CreateGroupRequest) (*Group, error) {
	return c.createGroup(ctx, "/transactional-groups", req)
}

// GetTransactionalGroup returns the transactional-email group identified by id.
func (c *Client) GetTransactionalGroup(ctx context.Context, id string) (*Group, error) {
	return c.getGroup(ctx, "/transactional-groups/"+id)
}

// UpdateTransactionalGroup updates the transactional-email group identified
// by id.
func (c *Client) UpdateTransactionalGroup(ctx context.Context, id string, req UpdateGroupRequest) (*Group, error) {
	return c.updateGroup(ctx, "/transactional-groups/"+id, req)
}

// ListTransactionalGroups returns a single page of transactional-email
// groups along with pagination information. To iterate every page, use
// [Paginate].
func (c *Client) ListTransactionalGroups(ctx context.Context, params PaginationParams) ([]Group, *Pagination, error) {
	return c.listGroups(ctx, "/transactional-groups", params)
}

// CreateWorkflowGroup creates a new workflow group.
func (c *Client) CreateWorkflowGroup(ctx context.Context, req CreateGroupRequest) (*Group, error) {
	return c.createGroup(ctx, "/workflow-groups", req)
}

// GetWorkflowGroup returns the workflow group identified by id.
func (c *Client) GetWorkflowGroup(ctx context.Context, id string) (*Group, error) {
	return c.getGroup(ctx, "/workflow-groups/"+id)
}

// UpdateWorkflowGroup updates the workflow group identified by id.
func (c *Client) UpdateWorkflowGroup(ctx context.Context, id string, req UpdateGroupRequest) (*Group, error) {
	return c.updateGroup(ctx, "/workflow-groups/"+id, req)
}

// ListWorkflowGroups returns a single page of workflow groups along with
// pagination information. To iterate every page, use [Paginate].
func (c *Client) ListWorkflowGroups(ctx context.Context, params PaginationParams) ([]Group, *Pagination, error) {
	return c.listGroups(ctx, "/workflow-groups", params)
}

func (c *Client) createGroup(ctx context.Context, path string, req CreateGroupRequest) (*Group, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(ctx, http.MethodPost, path, bytes.NewReader(b))
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

	var result Group
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

func (c *Client) getGroup(ctx context.Context, path string) (*Group, error) {
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
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

	var result Group
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

func (c *Client) updateGroup(ctx context.Context, path string, req UpdateGroupRequest) (*Group, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(ctx, http.MethodPost, path, bytes.NewReader(b))
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

	var result Group
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

func (c *Client) listGroups(ctx context.Context, basePath string, params PaginationParams) ([]Group, *Pagination, error) {
	q := url.Values{}
	if params.PerPage != "" {
		q.Set("perPage", params.PerPage)
	}
	if params.Cursor != "" {
		q.Set("cursor", params.Cursor)
	}

	path := basePath
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
		Pagination Pagination `json:"pagination"`
		Data       []Group    `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Data, &result.Pagination, nil
}
