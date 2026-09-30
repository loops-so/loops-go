package loops

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// Transactional describes a transactional email template managed via the
// content API. A transactional email can have a draft and/or a published
// [EmailMessage], linked via DraftEmailMessageID and PublishedEmailMessageID.
// URL is the transactional email's URL in the Loops app.
type Transactional struct {
	ID                      string   `json:"id"`
	URL                     string   `json:"url"`
	Name                    string   `json:"name"`
	DraftEmailMessageID     *string  `json:"draftEmailMessageId"`
	PublishedEmailMessageID *string  `json:"publishedEmailMessageId"`
	TransactionalGroupID    *string  `json:"transactionalGroupId"`
	CreatedAt               string   `json:"createdAt"`
	UpdatedAt               string   `json:"updatedAt"`
	DataVariables           []string `json:"dataVariables"`
}

// TransactionalDraft is returned by [Client.CreateTransactional] and
// [Client.EnsureTransactionalDraft]. It embeds the [Transactional] and adds
// DraftEmailMessageContentRevisionID, which should be passed as
// ExpectedRevisionID on the first call to [Client.UpdateEmailMessage] for
// optimistic concurrency control.
type TransactionalDraft struct {
	Transactional
	DraftEmailMessageContentRevisionID *string `json:"draftEmailMessageContentRevisionId"`
}

// CreateTransactionalRequest is the request body for [Client.CreateTransactional].
type CreateTransactionalRequest struct {
	Name                 string `json:"name"`
	TransactionalGroupID string `json:"transactionalGroupId,omitempty"`
}

// UpdateTransactionalRequest is the request body for [Client.UpdateTransactional].
// Name is required. Set TransactionalGroupID to move the email to a different
// group; leave it empty to keep the existing group.
type UpdateTransactionalRequest struct {
	Name                 string `json:"name"`
	TransactionalGroupID string `json:"transactionalGroupId,omitempty"`
}

// CreateTransactional creates a new transactional email with an empty draft
// email message. Use [Client.UpdateEmailMessage] on the returned
// DraftEmailMessageID to set the subject, sender, and LMX content, then call
// [Client.PublishTransactional] to publish.
func (c *Client) CreateTransactional(req CreateTransactionalRequest) (*TransactionalDraft, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(http.MethodPost, "/transactional-emails", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}

	resp, err := c.do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, errorFromResponse(resp)
	}

	var result TransactionalDraft
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetTransactional returns the transactional email identified by id.
func (c *Client) GetTransactional(id string) (*Transactional, error) {
	req, err := c.newRequest(http.MethodGet, "/transactional-emails/"+id, nil)
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

	var result Transactional
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// UpdateTransactional updates the transactional email identified by id and
// returns its new state.
func (c *Client) UpdateTransactional(id string, req UpdateTransactionalRequest) (*Transactional, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(http.MethodPost, "/transactional-emails/"+id, bytes.NewReader(b))
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

	var result Transactional
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// EnsureTransactionalDraft ensures the transactional email has a draft email
// message. If a draft already exists it is returned unchanged; otherwise a
// new empty draft is created (seeded from the most recent published version
// when present).
func (c *Client) EnsureTransactionalDraft(id string) (*TransactionalDraft, error) {
	req, err := c.newRequest(http.MethodPost, "/transactional-emails/"+id+"/draft", nil)
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

	var result TransactionalDraft
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// PublishTransactional publishes the transactional email's current draft
// email message. The draft becomes the published version and the draft is
// cleared.
func (c *Client) PublishTransactional(id string) (*Transactional, error) {
	req, err := c.newRequest(http.MethodPost, "/transactional-emails/"+id+"/publish", nil)
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

	var result Transactional
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ListTransactionals returns a single page of transactional emails along
// with pagination information. To iterate every page, use [Paginate].
func (c *Client) ListTransactionals(params PaginationParams) ([]Transactional, *Pagination, error) {
	q := url.Values{}
	if params.PerPage != "" {
		q.Set("perPage", params.PerPage)
	}
	if params.Cursor != "" {
		q.Set("cursor", params.Cursor)
	}

	path := "/transactional-emails"
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
		Pagination Pagination      `json:"pagination"`
		Data       []Transactional `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Data, &result.Pagination, nil
}

// TransactionalMetrics holds all-time delivery counters for a transactional
// email, as returned by [Client.GetTransactionalMetrics]. Counters are
// totalled across every published version of the email.
type TransactionalMetrics struct {
	Sends       int `json:"sends"`
	Deliveries  int `json:"deliveries"`
	SpamReports int `json:"spamReports"`
	HardBounces int `json:"hardBounces"`
	SoftBounces int `json:"softBounces"`
}

// GetTransactionalMetrics returns all-time delivery metrics for the
// transactional email identified by id.
func (c *Client) GetTransactionalMetrics(id string) (*TransactionalMetrics, error) {
	req, err := c.newRequest(http.MethodGet, "/transactional-emails/"+id+"/metrics", nil)
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

	var result TransactionalMetrics
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
