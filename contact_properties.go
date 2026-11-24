package loops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ContactProperty describes a property defined on a contact: its API key,
// human-readable label, and value type (e.g. "string", "number", "boolean",
// "date").
type ContactProperty struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Type  string `json:"type"`
}

// ListContactPropertiesParams controls a [Client.ListContactProperties]
// request. Set CustomOnly to return only custom properties; the zero value
// returns built-in properties as well.
type ListContactPropertiesParams struct {
	CustomOnly bool
}

// ListContactProperties returns the contact properties defined on your
// account.
func (c *Client) ListContactProperties(ctx context.Context, params ListContactPropertiesParams) ([]ContactProperty, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/contacts/properties", nil)
	if err != nil {
		return nil, err
	}

	if params.CustomOnly {
		q := req.URL.Query()
		q.Set("list", "custom")
		req.URL.RawQuery = q.Encode()
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errorFromResponse(resp)
	}

	var result []ContactProperty
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// CreateContactProperty creates a new custom contact property with the
// given name and type (e.g. "string", "number", "boolean", "date").
func (c *Client) CreateContactProperty(ctx context.Context, name, propType string) error {
	b, err := json.Marshal(map[string]string{"name": name, "type": propType})
	if err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}

	req, err := c.newRequest(ctx, http.MethodPost, "/contacts/properties", bytes.NewReader(b))
	if err != nil {
		return err
	}

	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errorFromResponse(resp)
	}

	return nil
}
