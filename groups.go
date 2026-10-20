package loops

import (
	"bytes"
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
func (c *Client) CreateCampaignGroup(req CreateGroupRequest) (*Group, error) {
	return c.createGroup("/campaign-groups", req)
}

// GetCampaignGroup returns the campaign group identified by id.
func (c *Client) GetCampaignGroup(id string) (*Group, error) {
	return c.getGroup("/campaign-groups/" + id)
}

// UpdateCampaignGroup updates the campaign group identified by id.
func (c *Client) UpdateCampaignGroup(id string, req UpdateGroupRequest) (*Group, error) {
	return c.updateGroup("/campaign-groups/"+id, req)
}

// ListCampaignGroups returns a single page of campaign groups along with
// pagination information. To iterate every page, use [Paginate].
func (c *Client) ListCampaignGroups(params PaginationParams) ([]Group, *Pagination, error) {
	return c.listGroups("/campaign-groups", params)
}

// CreateTransactionalGroup creates a new transactional-email group.
func (c *Client) CreateTransactionalGroup(req CreateGroupRequest) (*Group, error) {
	return c.createGroup("/transactional-groups", req)
}

// GetTransactionalGroup returns the transactional-email group identified by id.
func (c *Client) GetTransactionalGroup(id string) (*Group, error) {
	return c.getGroup("/transactional-groups/" + id)
}

// UpdateTransactionalGroup updates the transactional-email group identified
// by id.
func (c *Client) UpdateTransactionalGroup(id string, req UpdateGroupRequest) (*Group, error) {
	return c.updateGroup("/transactional-groups/"+id, req)
}

// ListTransactionalGroups returns a single page of transactional-email
// groups along with pagination information. To iterate every page, use
// [Paginate].
func (c *Client) ListTransactionalGroups(params PaginationParams) ([]Group, *Pagination, error) {
	return c.listGroups("/transactional-groups", params)
}

// CreateWorkflowGroup creates a new workflow group.
func (c *Client) CreateWorkflowGroup(req CreateGroupRequest) (*Group, error) {
	return c.createGroup("/workflow-groups", req)
}

// GetWorkflowGroup returns the workflow group identified by id.
func (c *Client) GetWorkflowGroup(id string) (*Group, error) {
	return c.getGroup("/workflow-groups/" + id)
}

// UpdateWorkflowGroup updates the workflow group identified by id.
func (c *Client) UpdateWorkflowGroup(id string, req UpdateGroupRequest) (*Group, error) {
	return c.updateGroup("/workflow-groups/"+id, req)
}

// ListWorkflowGroups returns a single page of workflow groups along with
// pagination information. To iterate every page, use [Paginate].
func (c *Client) ListWorkflowGroups(params PaginationParams) ([]Group, *Pagination, error) {
	return c.listGroups("/workflow-groups", params)
}

func (c *Client) createGroup(path string, req CreateGroupRequest) (*Group, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(http.MethodPost, path, bytes.NewReader(b))
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

func (c *Client) getGroup(path string) (*Group, error) {
	req, err := c.newRequest(http.MethodGet, path, nil)
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

func (c *Client) updateGroup(path string, req UpdateGroupRequest) (*Group, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(http.MethodPost, path, bytes.NewReader(b))
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

func (c *Client) listGroups(basePath string, params PaginationParams) ([]Group, *Pagination, error) {
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

	req, err := c.newRequest(http.MethodGet, path, nil)
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
