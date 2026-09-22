package loops

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// TransactionalEmail describes a transactional email template configured in
// Loops, as returned by [Client.ListTransactional].
type TransactionalEmail struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	LastUpdated   string   `json:"lastUpdated"`
	DataVariables []string `json:"dataVariables"`
}

// Attachment is a file attached to a transactional email. Data must be the
// file contents encoded as a base64 string.
type Attachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Data        string `json:"data"`
}

// SendTransactionalRequest is the request body for [Client.SendTransactional].
//
// TransactionalID identifies the template to send and Email is the
// recipient. DataVariables populates the template's variables. If
// AddToAudience is true, the recipient is also added to your audience.
//
// If IdempotencyKey is set, it is sent as the Idempotency-Key HTTP header so
// the send can be safely retried without duplication.
type SendTransactionalRequest struct {
	Email           string         `json:"email"`
	TransactionalID string         `json:"transactionalId"`
	AddToAudience   *bool          `json:"addToAudience,omitempty"`
	DataVariables   map[string]any `json:"dataVariables,omitempty"`
	Attachments     []Attachment   `json:"attachments,omitempty"`
	IdempotencyKey  string         `json:"-"`
}

// SendTransactional sends a transactional email. See
// [SendTransactionalRequest] for field semantics.
func (c *Client) SendTransactional(req SendTransactionalRequest) error {
	b, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(http.MethodPost, "/transactional", bytes.NewReader(b))
	if err != nil {
		return err
	}
	if req.IdempotencyKey != "" {
		httpReq.Header.Set("Idempotency-Key", req.IdempotencyKey)
	}

	resp, err := c.do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errorFromResponse(resp)
	}

	return nil
}

// ListTransactional returns a single page of transactional email templates
// from the legacy list endpoint, along with pagination information. To
// iterate every page, use [Paginate]:
//
//	all, err := loops.Paginate(func(cursor string) ([]loops.TransactionalEmail, *loops.Pagination, error) {
//	    return client.ListTransactional(loops.PaginationParams{Cursor: cursor})
//	})
//
// Deprecated: use [Client.ListTransactionals], which returns the richer
// [Transactional] shape (draft and published email message IDs, separate
// created/updated timestamps).
func (c *Client) ListTransactional(params PaginationParams) ([]TransactionalEmail, *Pagination, error) {
	q := url.Values{}
	if params.PerPage != "" {
		q.Set("perPage", params.PerPage)
	}
	if params.Cursor != "" {
		q.Set("cursor", params.Cursor)
	}

	path := "/transactional"
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
		Pagination Pagination           `json:"pagination"`
		Data       []TransactionalEmail `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Data, &result.Pagination, nil
}
