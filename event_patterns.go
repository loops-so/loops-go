package loops

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// EventPatternSummary is the list-item representation of an event pattern
// returned by [Client.ListEventPatterns].
type EventPatternSummary struct {
	ID        string `json:"id"`
	EventName string `json:"eventName"`
	// IncomingWebhookPlatform names the platform that sent this event
	// pattern when it originates from an incoming webhook (one of "clerk",
	// "polar", "stripe", or "supabase"). It is nil for custom events.
	IncomingWebhookPlatform *string `json:"incomingWebhookPlatform"`
}

// EventPattern is the full representation of an event pattern, including the
// event properties usable in emails.
type EventPattern struct {
	ID              string                  `json:"id"`
	EventName       string                  `json:"eventName"`
	EventProperties []WorkflowEventProperty `json:"eventProperties"`
	// IncomingWebhookPlatform names the platform that sent this event
	// pattern when it originates from an incoming webhook (one of "clerk",
	// "polar", "stripe", or "supabase"). It is nil for custom events.
	IncomingWebhookPlatform *string `json:"incomingWebhookPlatform"`
}

// ListEventPatterns returns a single page of event patterns along with
// pagination information. To iterate every page, use [Paginate].
func (c *Client) ListEventPatterns(ctx context.Context, params PaginationParams) ([]EventPatternSummary, *Pagination, error) {
	q := url.Values{}
	if params.PerPage != 0 {
		q.Set("perPage", strconv.Itoa(params.PerPage))
	}
	if params.Cursor != "" {
		q.Set("cursor", params.Cursor)
	}

	path := "/event-patterns"
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
		Pagination Pagination            `json:"pagination"`
		Data       []EventPatternSummary `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Data, &result.Pagination, nil
}

// GetEventPatternByName returns the event pattern identified by eventName.
func (c *Client) GetEventPatternByName(ctx context.Context, eventName string) (*EventPattern, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/event-patterns/by-name/"+url.PathEscape(eventName), nil)
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

	var result EventPattern
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetEventPattern returns the event pattern identified by id.
func (c *Client) GetEventPattern(ctx context.Context, id string) (*EventPattern, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/event-patterns/"+id, nil)
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

	var result EventPattern
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
