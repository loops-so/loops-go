package loops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// FooterPlacementInside renders the footer inside the email body.
const FooterPlacementInside = "inside"

// FooterPlacementBelow renders the footer below the email body.
const FooterPlacementBelow = "below"

// FooterExistingEmailsPreviousDefault switches the existing emails that use
// the previous default footer.
const FooterExistingEmailsPreviousDefault = "previousDefault"

// FooterExistingEmailsAll switches all existing campaign and workflow emails.
const FooterExistingEmailsAll = "all"

// FooterExistingEmailsNone switches no existing emails. Only new emails use
// the footer.
const FooterExistingEmailsNone = "none"

// Footer is a custom email footer. New campaign and workflow emails use the
// footer that has IsDefault set to true. UpdatedAt changes when the footer's
// own fields change, not when a variant changes.
type Footer struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
	Placement string `json:"placement"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// FooterVariant is the content of a [Footer] as LMX. PublishedAt and
// PublishedLMX are nil when the variant was never published. Pass
// ContentRevisionID as ExpectedRevisionID to update or publish the variant.
type FooterVariant struct {
	ID                string  `json:"id"`
	FooterID          string  `json:"footerId"`
	ContentRevisionID string  `json:"contentRevisionId"`
	DraftLMX          string  `json:"draftLmx"`
	PublishedLMX      *string `json:"publishedLmx"`
	PublishedAt       *string `json:"publishedAt"`
	CreatedAt         string  `json:"createdAt"`
	UpdatedAt         string  `json:"updatedAt"`
}

// FooterWithVariants is a [Footer] with the content of each variant. A footer
// has at most one variant.
type FooterWithVariants struct {
	Footer
	Variants []FooterVariant `json:"variants"`
}

// CreateFooterRequest is the request body for [Client.CreateFooter]. Name is
// optional; the API applies a default name when it is omitted.
type CreateFooterRequest struct {
	Name string `json:"name,omitempty"`
}

// UpdateFooterRequest is the request body for [Client.UpdateFooter]. At least
// one field must be set.
type UpdateFooterRequest struct {
	Name      string `json:"name,omitempty"`
	Placement string `json:"placement,omitempty"`
}

// SetDefaultFooterRequest is the request body for [Client.SetDefaultFooter].
// ExistingEmails is required and selects which existing campaign and workflow
// emails switch to the footer.
type SetDefaultFooterRequest struct {
	ExistingEmails string `json:"existingEmails"`
}

// SetDefaultFooterResponse is returned by [Client.SetDefaultFooter].
// UpdatedEmailCount is the number of existing emails that switched to the
// footer.
type SetDefaultFooterResponse struct {
	FooterWithVariants
	UpdatedEmailCount int `json:"updatedEmailCount"`
}

// UpdateFooterVariantRequest is the request body for
// [Client.UpdateFooterVariant]. Both fields are required.
type UpdateFooterVariantRequest struct {
	ExpectedRevisionID string `json:"expectedRevisionId"`
	LMX                string `json:"lmx"`
}

// PublishFooterVariantRequest is the request body for
// [Client.PublishFooterVariant]. ExpectedRevisionID is required.
type PublishFooterVariantRequest struct {
	ExpectedRevisionID string `json:"expectedRevisionId"`
}

// ListFooters returns a single page of footers, most recently created first,
// along with pagination information. To iterate every page, use [Paginate].
func (c *Client) ListFooters(ctx context.Context, params PaginationParams) ([]Footer, *Pagination, error) {
	q := url.Values{}
	if params.PerPage != "" {
		q.Set("perPage", params.PerPage)
	}
	if params.Cursor != "" {
		q.Set("cursor", params.Cursor)
	}

	path := "/footers"
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
		Pagination Pagination `json:"pagination"`
		Data       []Footer   `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Data, &result.Pagination, nil
}

// CreateFooter creates a footer with one variant. The variant starts with an
// unpublished draft. Emails cannot use the footer until the variant is
// published with [Client.PublishFooterVariant].
func (c *Client) CreateFooter(ctx context.Context, req CreateFooterRequest) (*FooterWithVariants, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(ctx, http.MethodPost, "/footers", bytes.NewReader(b))
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

	var result FooterWithVariants
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetFooter returns the footer identified by id, with the content of each
// variant.
func (c *Client) GetFooter(ctx context.Context, id string) (*FooterWithVariants, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/footers/"+id, nil)
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

	var result FooterWithVariants
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// UpdateFooter updates the name or placement of the footer identified by id.
// Changes apply at once, also to running workflows. To change the default
// footer, use [Client.SetDefaultFooter].
func (c *Client) UpdateFooter(ctx context.Context, id string, req UpdateFooterRequest) (*FooterWithVariants, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(ctx, http.MethodPost, "/footers/"+id, bytes.NewReader(b))
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

	var result FooterWithVariants
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// DeleteFooter deletes the footer identified by id and its variants. The API
// rejects the delete with a 409 when the footer is the team's default footer
// or when emails use it.
func (c *Client) DeleteFooter(ctx context.Context, id string) error {
	req, err := c.newRequest(ctx, http.MethodDelete, "/footers/"+id, nil)
	if err != nil {
		return err
	}

	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return errorFromResponse(resp)
	}

	return nil
}

// SetDefaultFooter makes the published footer identified by id the team's
// default footer. New campaign and workflow emails use the default footer.
// The API rejects the request with a 409 when the footer is not published.
func (c *Client) SetDefaultFooter(ctx context.Context, id string, req SetDefaultFooterRequest) (*SetDefaultFooterResponse, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(ctx, http.MethodPost, "/footers/"+id+"/set-default", bytes.NewReader(b))
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

	var result SetDefaultFooterResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetFooterVariant returns the draft and published content of the footer
// variant identified by footerID and variantID.
func (c *Client) GetFooterVariant(ctx context.Context, footerID, variantID string) (*FooterVariant, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/footers/"+footerID+"/variants/"+variantID, nil)
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

	var result FooterVariant
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// UpdateFooterVariant updates the draft LMX content of a footer variant.
// Emails keep the published content until the variant is published with
// [Client.PublishFooterVariant]. The API rejects the update with a 409 when
// ExpectedRevisionID is stale, and with a 422 when the LMX is invalid or uses
// LMX that footers do not support.
func (c *Client) UpdateFooterVariant(ctx context.Context, footerID, variantID string, req UpdateFooterVariantRequest) (*FooterVariant, error) {
	return c.postFooterVariant(ctx, "/footers/"+footerID+"/variants/"+variantID, req)
}

// PublishFooterVariant publishes the draft LMX content of a footer variant.
// Emails that use the footer use the new content at once. The API rejects the
// publish with a 409 when ExpectedRevisionID is stale, and with a 422 when the
// draft does not have {system.company_name} and {system.company_address} in
// the text and {system.unsubscribe_link} as the URL of a link or button.
func (c *Client) PublishFooterVariant(ctx context.Context, footerID, variantID string, req PublishFooterVariantRequest) (*FooterVariant, error) {
	return c.postFooterVariant(ctx, "/footers/"+footerID+"/variants/"+variantID+"/publish", req)
}

func (c *Client) postFooterVariant(ctx context.Context, path string, req any) (*FooterVariant, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(ctx, http.MethodPost, path, bytes.NewReader(b))
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

	var result FooterVariant
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
