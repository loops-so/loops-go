package loops

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// APIKeyResponse is returned by [Client.GetAPIKey] and identifies the team
// the API key belongs to.
type APIKeyResponse struct {
	Success  bool   `json:"success"`
	TeamName string `json:"teamName"`
}

// GetAPIKey verifies the client's API key and returns information about the
// team it belongs to. Useful as a connectivity check at startup.
func (c *Client) GetAPIKey(ctx context.Context) (*APIKeyResponse, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api-key", nil)
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

	var result APIKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
