package loops

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// EmailFormatStyled is the rendered email format with HTML styling.
const EmailFormatStyled = "styled"

// EmailFormatPlain is the rendered email format with no HTML styling.
const EmailFormatPlain = "plain"

// EmailMessage is the persisted form of an email's content and metadata,
// including its LMX body. Warnings is populated by the LMX linter when the
// message is fetched or updated.
//
// CampaignID and TransactionalID are mutually exclusive. At most one is
// non-nil and identifies the parent. Both are nil for a workflow email
// message.
type EmailMessage struct {
	ID                         string            `json:"id"`
	CampaignID                 *string           `json:"campaignId,omitempty"`
	TransactionalID            *string           `json:"transactionalId,omitempty"`
	Subject                    string            `json:"subject"`
	PreviewText                string            `json:"previewText"`
	FromName                   string            `json:"fromName"`
	FromEmail                  string            `json:"fromEmail"`
	ReplyToEmail               string            `json:"replyToEmail"`
	CCEmail                    string            `json:"ccEmail,omitempty"`
	BCCEmail                   string            `json:"bccEmail,omitempty"`
	LanguageCode               string            `json:"languageCode,omitempty"`
	EmailFormat                string            `json:"emailFormat"`
	LMX                        string            `json:"lmx"`
	ContentRevisionID          *string           `json:"contentRevisionId"`
	UpdatedAt                  string            `json:"updatedAt"`
	ContactPropertiesFallbacks map[string]string `json:"contactPropertiesFallbacks,omitempty"`
	EventPropertiesFallbacks   map[string]string `json:"eventPropertiesFallbacks,omitempty"`
	DataVariablesFallbacks     map[string]string `json:"dataVariablesFallbacks,omitempty"`
	Warnings                   []LmxWarning      `json:"warnings,omitempty"`
}

// UpdateEmailMessageRequest is the request body for [Client.UpdateEmailMessage].
//
// Set selects which fields from the embedded [EmailMessageFields] are
// applied — only fields whose key is true in Set are sent. This lets the
// caller distinguish "leave alone" from "set to empty string".
//
// For the *Fallbacks maps, a nil value in the map clears the fallback for
// that key; a string value sets it. Each map is a full replacement of the
// server-side map.
//
// ExpectedRevisionID is optional; if set, the update fails if the message's
// current ContentRevisionID does not match (optimistic concurrency control).
type UpdateEmailMessageRequest struct {
	EmailMessageFields
	Set                map[string]bool
	ExpectedRevisionID string
}

// EmailMessagePreviewRequest is the request body for
// [Client.PreviewEmailMessage]. The accepted variable fields depend on the
// parent email message's type:
//   - campaign previews accept ContactProperties
//   - workflow previews accept ContactProperties and EventProperties
//   - transactional previews accept DataVariables
type EmailMessagePreviewRequest struct {
	Emails            []string          `json:"emails"`
	ContactProperties map[string]string `json:"contactProperties,omitempty"`
	EventProperties   map[string]string `json:"eventProperties,omitempty"`
	DataVariables     map[string]any    `json:"dataVariables,omitempty"`
}

// EmailMessagePreviewResponse is returned by [Client.PreviewEmailMessage].
// ID is the email message ID the preview was sent for.
type EmailMessagePreviewResponse struct {
	ID string `json:"id"`
}

// EmailMessageRenderRequest is the request body for
// [Client.RenderEmailMessage]. All fields are optional. The accepted variable
// fields depend on the parent email message's type:
//   - campaign renders accept ContactProperties
//   - workflow renders accept ContactProperties and EventProperties
//   - transactional renders accept DataVariables
type EmailMessageRenderRequest struct {
	ContactProperties map[string]string `json:"contactProperties,omitempty"`
	EventProperties   map[string]string `json:"eventProperties,omitempty"`
	DataVariables     map[string]string `json:"dataVariables,omitempty"`
}

// EmailMessageRenderResponse is returned by [Client.RenderEmailMessage].
// HTML is the rendered email body. For an email made in the editor, HTML
// contains the rendered subject in <title> and the rendered preview text.
type EmailMessageRenderResponse struct {
	HTML string `json:"html"`
}

// GuardianRuleItem is a single element that triggered a [GuardianRule].
type GuardianRuleItem struct {
	Label    string `json:"label"`
	CodeName string `json:"codeName,omitempty"`
}

// GuardianRule is a Guardian check that fired against an email message,
// grouping the specific Items that triggered it.
type GuardianRule struct {
	Rule        string             `json:"rule"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Items       []GuardianRuleItem `json:"items"`
}

// EmailMessageGuardianResponse is returned by [Client.GetEmailMessageGuardian].
// Errors must be resolved before the email can be published; Warnings are
// advisory and do not block publishing.
type EmailMessageGuardianResponse struct {
	Errors   []GuardianRule `json:"errors"`
	Warnings []GuardianRule `json:"warnings"`
}

// GetEmailMessage returns the email message identified by id.
func (c *Client) GetEmailMessage(id string) (*EmailMessage, error) {
	req, err := c.newRequest(http.MethodGet, "/email-messages/"+id, nil)
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

	var result EmailMessage
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// UpdateEmailMessage updates the email message identified by id with the
// fields selected in req.Set and returns its new state. See
// [UpdateEmailMessageRequest] for the selection semantics. The API rejects
// the update when the parent campaign is not a draft, when the email message
// is the published email message of a transactional email, or when the
// parent workflow is sending.
func (c *Client) UpdateEmailMessage(id string, req UpdateEmailMessageRequest) (*EmailMessage, error) {
	body := map[string]any{}
	if req.Set["subject"] {
		body["subject"] = req.Subject
	}
	if req.Set["previewText"] {
		body["previewText"] = req.PreviewText
	}
	if req.Set["fromName"] {
		body["fromName"] = req.FromName
	}
	if req.Set["fromEmail"] {
		body["fromEmail"] = req.FromEmail
	}
	if req.Set["replyToEmail"] {
		body["replyToEmail"] = req.ReplyToEmail
	}
	if req.Set["ccEmail"] {
		body["ccEmail"] = req.CCEmail
	}
	if req.Set["bccEmail"] {
		body["bccEmail"] = req.BCCEmail
	}
	if req.Set["languageCode"] {
		body["languageCode"] = req.LanguageCode
	}
	if req.Set["emailFormat"] {
		body["emailFormat"] = req.EmailFormat
	}
	if req.Set["lmx"] {
		body["lmx"] = req.LMX
	}
	if req.Set["contactPropertiesFallbacks"] {
		body["contactPropertiesFallbacks"] = req.ContactPropertiesFallbacks
	}
	if req.Set["eventPropertiesFallbacks"] {
		body["eventPropertiesFallbacks"] = req.EventPropertiesFallbacks
	}
	if req.Set["dataVariablesFallbacks"] {
		body["dataVariablesFallbacks"] = req.DataVariablesFallbacks
	}
	if req.ExpectedRevisionID != "" {
		body["expectedRevisionId"] = req.ExpectedRevisionID
	}

	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(http.MethodPost, "/email-messages/"+id, bytes.NewReader(b))
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

	var result EmailMessage
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// PreviewEmailMessage sends a test preview of the email message identified
// by id to the addresses in req.Emails. See [EmailMessagePreviewRequest]
// for the allowed variable fields by parent type.
func (c *Client) PreviewEmailMessage(id string, req EmailMessagePreviewRequest) (*EmailMessagePreviewResponse, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(http.MethodPost, "/email-messages/"+id+"/preview", bytes.NewReader(b))
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

	var result EmailMessagePreviewResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// RenderEmailMessage renders the HTML of the email message identified by id
// without sending it. See [EmailMessageRenderRequest] for the allowed
// variable fields by parent type.
func (c *Client) RenderEmailMessage(id string, req EmailMessageRenderRequest) (*EmailMessageRenderResponse, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(http.MethodPost, "/email-messages/"+id+"/render", bytes.NewReader(b))
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

	var result EmailMessageRenderResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetEmailMessageGuardian runs Guardian content validation on the email
// message identified by id and returns any errors and warnings.
func (c *Client) GetEmailMessageGuardian(id string) (*EmailMessageGuardianResponse, error) {
	req, err := c.newRequest(http.MethodGet, "/email-messages/"+id+"/guardian", nil)
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

	var result EmailMessageGuardianResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
