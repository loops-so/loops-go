package loops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// SendEventRequest is the request body for [Client.SendEvent].
//
// One of Email or UserID must be set to identify the contact. EventName is
// required. EventProperties are sent alongside the event; ContactProperties
// are merged onto the contact at the same time.
//
// If IdempotencyKey is set, it is sent as the Idempotency-Key HTTP header so
// the event can be safely retried without duplication.
type SendEventRequest struct {
	Email             string          `json:"-"`
	UserID            string          `json:"-"`
	EventName         string          `json:"-"`
	EventProperties   map[string]any  `json:"-"`
	MailingLists      map[string]bool `json:"-"`
	ContactProperties map[string]any  `json:"-"`
	IdempotencyKey    string          `json:"-"`
}

// SendEvent sends an event to Loops, optionally updating contact properties
// and mailing-list memberships at the same time. See [SendEventRequest] for
// field semantics.
func (c *Client) SendEvent(ctx context.Context, req SendEventRequest) error {
	body := make(map[string]any)
	for k, v := range req.ContactProperties {
		body[k] = v
	}
	body["eventName"] = req.EventName
	if req.Email != "" {
		body["email"] = req.Email
	}
	if req.UserID != "" {
		body["userId"] = req.UserID
	}
	if len(req.EventProperties) > 0 {
		body["eventProperties"] = req.EventProperties
	}
	if len(req.MailingLists) > 0 {
		body["mailingLists"] = req.MailingLists
	}

	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(ctx, http.MethodPost, "/events/send", bytes.NewReader(b))
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
