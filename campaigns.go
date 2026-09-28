package loops

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// CampaignSchedulingMethodNow means the campaign sends immediately when
// scheduled.
const CampaignSchedulingMethodNow = "now"

// CampaignSchedulingMethodSchedule means the campaign sends at a specific
// time.
const CampaignSchedulingMethodSchedule = "schedule"

// CampaignScheduling describes when a campaign is scheduled to send.
// Timestamp is nil when Method is [CampaignSchedulingMethodNow].
type CampaignScheduling struct {
	Method    string  `json:"method"`
	Timestamp *string `json:"timestamp"`
}

// CampaignSchedulingRequest sets when a campaign should send. Timestamp is
// required (and must be in the future) when Method is
// [CampaignSchedulingMethodSchedule], and must be empty when Method is
// [CampaignSchedulingMethodNow].
type CampaignSchedulingRequest struct {
	Method    string `json:"method"`
	Timestamp string `json:"timestamp,omitempty"`
}

// Campaign describes a campaign, as returned by [Client.GetCampaign],
// [Client.UpdateCampaign], and [Client.ListCampaigns]. A campaign holds a
// single [EmailMessage] linked via EmailMessageID. URL is the campaign's URL
// in the Loops app.
type Campaign struct {
	ID                string             `json:"id"`
	URL               string             `json:"url"`
	EmailMessageID    *string            `json:"emailMessageId"`
	Name              string             `json:"name"`
	Status            string             `json:"status"`
	CreatedAt         string             `json:"createdAt"`
	UpdatedAt         string             `json:"updatedAt"`
	CampaignGroupID   *string            `json:"campaignGroupId"`
	MailingListID     *string            `json:"mailingListId"`
	AudienceSegmentID *string            `json:"audienceSegmentId"`
	AudienceFilter    *AudienceFilter    `json:"audienceFilter"`
	Scheduling        CampaignScheduling `json:"scheduling"`
}

// CampaignListItem is the entry shape returned by [Client.ListCampaigns].
//
// Deprecated: list items are now full [Campaign] values; use [Campaign]
// directly. CampaignListItem is retained as an alias.
type CampaignListItem = Campaign

// LmxWarning is a non-fatal issue reported by the Loops Markup (LMX) linter
// when validating email content. Severity is typically "warning" or "info".
type LmxWarning struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Path     string `json:"path,omitempty"`
}

// EmailMessageFields holds the editable fields of an email message. It is
// embedded in [UpdateEmailMessageRequest]; the request's Set map determines
// which fields are actually written.
//
// CCEmail and BCCEmail require CC/BCC to be enabled for the team and are not
// supported for campaign emails.
type EmailMessageFields struct {
	Subject                    string             `json:"subject,omitempty"`
	PreviewText                string             `json:"previewText,omitempty"`
	FromName                   string             `json:"fromName,omitempty"`
	FromEmail                  string             `json:"fromEmail,omitempty"`
	ReplyToEmail               string             `json:"replyToEmail,omitempty"`
	CCEmail                    string             `json:"ccEmail,omitempty"`
	BCCEmail                   string             `json:"bccEmail,omitempty"`
	LanguageCode               string             `json:"languageCode,omitempty"`
	EmailFormat                string             `json:"emailFormat,omitempty"`
	LMX                        string             `json:"lmx,omitempty"`
	ContactPropertiesFallbacks map[string]*string `json:"contactPropertiesFallbacks,omitempty"`
	EventPropertiesFallbacks   map[string]*string `json:"eventPropertiesFallbacks,omitempty"`
	DataVariablesFallbacks     map[string]*string `json:"dataVariablesFallbacks,omitempty"`
}

// CreateCampaignRequest is the request body for [Client.CreateCampaign].
// MailingListID and AudienceSegmentID are pointer-typed: leave nil to omit,
// or set to a pointer to a string value.
type CreateCampaignRequest struct {
	Name              string                     `json:"name"`
	CampaignGroupID   string                     `json:"campaignGroupId,omitempty"`
	MailingListID     *string                    `json:"mailingListId,omitempty"`
	AudienceSegmentID *string                    `json:"audienceSegmentId,omitempty"`
	AudienceFilter    *AudienceFilter            `json:"audienceFilter,omitempty"`
	Scheduling        *CampaignSchedulingRequest `json:"scheduling,omitempty"`
}

// UpdateCampaignRequest is the request body for [Client.UpdateCampaign].
//
// Set selects which fields are applied — only fields whose key is true in
// Set are sent. This lets the caller distinguish "leave alone" from "set to
// null" for nullable fields, and from "set to empty string" for required
// string fields.
//
// At least one field must be selected. Setting AudienceSegmentID clears any
// AudienceFilter and vice versa.
type UpdateCampaignRequest struct {
	Name              string
	CampaignGroupID   string
	MailingListID     *string
	AudienceSegmentID *string
	AudienceFilter    *AudienceFilter
	Scheduling        *CampaignSchedulingRequest
	Set               map[string]bool
}

// CampaignCreateResponse is returned by [Client.CreateCampaign]. It embeds
// the new [Campaign] and adds the initial content-revision ID for the
// campaign's email message.
type CampaignCreateResponse struct {
	Campaign
	EmailMessageContentRevisionID *string `json:"emailMessageContentRevisionId"`
}

// CreateCampaign creates a new campaign and an empty email message attached
// to it.
func (c *Client) CreateCampaign(req CreateCampaignRequest) (*CampaignCreateResponse, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(http.MethodPost, "/campaigns", bytes.NewReader(b))
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

	var result CampaignCreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// UpdateCampaign updates the campaign identified by id with the fields
// selected in req.Set and returns its new state. See [UpdateCampaignRequest]
// for the selection semantics.
func (c *Client) UpdateCampaign(id string, req UpdateCampaignRequest) (*Campaign, error) {
	body := map[string]any{}
	if req.Set["name"] {
		body["name"] = req.Name
	}
	if req.Set["campaignGroupId"] {
		body["campaignGroupId"] = req.CampaignGroupID
	}
	if req.Set["mailingListId"] {
		body["mailingListId"] = req.MailingListID
	}
	if req.Set["audienceSegmentId"] {
		body["audienceSegmentId"] = req.AudienceSegmentID
	}
	if req.Set["audienceFilter"] {
		body["audienceFilter"] = req.AudienceFilter
	}
	if req.Set["scheduling"] {
		body["scheduling"] = req.Scheduling
	}

	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(http.MethodPost, "/campaigns/"+id, bytes.NewReader(b))
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

	var result Campaign
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetCampaign returns the campaign identified by id.
func (c *Client) GetCampaign(id string) (*Campaign, error) {
	req, err := c.newRequest(http.MethodGet, "/campaigns/"+id, nil)
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

	var result Campaign
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ListCampaigns returns a single page of campaigns along with pagination
// information. To iterate every page, use [Paginate].
func (c *Client) ListCampaigns(params PaginationParams) ([]Campaign, *Pagination, error) {
	q := url.Values{}
	if params.PerPage != "" {
		q.Set("perPage", params.PerPage)
	}
	if params.Cursor != "" {
		q.Set("cursor", params.Cursor)
	}

	path := "/campaigns"
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
		Data       []Campaign `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Data, &result.Pagination, nil
}

// EmailMetrics holds all-time email engagement counters, as returned by
// [Client.GetCampaignMetrics] and [Client.GetWorkflowNodeMetrics]. Opens and
// Clicks count sends, not events: a send opened five times counts once. A
// recipient who received the email twice is counted once per send.
type EmailMetrics struct {
	Sends        int `json:"sends"`
	Opens        int `json:"opens"`
	Clicks       int `json:"clicks"`
	Unsubscribes int `json:"unsubscribes"`
	SpamReports  int `json:"spamReports"`
	HardBounces  int `json:"hardBounces"`
	SoftBounces  int `json:"softBounces"`
}

// GetCampaignMetrics returns all-time email engagement metrics for the
// campaign identified by id. The server responds with a 400 error when the
// campaign has not been sent yet or has no email message.
func (c *Client) GetCampaignMetrics(id string) (*EmailMetrics, error) {
	req, err := c.newRequest(http.MethodGet, "/campaigns/"+id+"/metrics", nil)
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

	var result EmailMetrics
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
