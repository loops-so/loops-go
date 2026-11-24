package loops

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// GetDedicatedSendingIPs returns the dedicated sending IP addresses assigned
// to the team. Returns an empty slice when no dedicated IPs are assigned.
func (c *Client) GetDedicatedSendingIPs(ctx context.Context) ([]string, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/dedicated-sending-ips", nil)
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

	var result []string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}
