package loops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// ThemeStyles is the set of visual style values for a [Theme]. Colors are
// CSS color strings (typically hex), and numeric fields are pixel values
// unless otherwise implied by the field name.
type ThemeStyles struct {
	BackgroundColor       string  `json:"backgroundColor"`
	BackgroundXPadding    float64 `json:"backgroundXPadding"`
	BackgroundYPadding    float64 `json:"backgroundYPadding"`
	BodyColor             string  `json:"bodyColor"`
	BodyXPadding          float64 `json:"bodyXPadding"`
	BodyYPadding          float64 `json:"bodyYPadding"`
	BodyFontFamily        string  `json:"bodyFontFamily"`
	BodyFontCategory      string  `json:"bodyFontCategory"`
	BorderColor           string  `json:"borderColor"`
	BorderWidth           float64 `json:"borderWidth"`
	BorderRadius          float64 `json:"borderRadius"`
	ButtonBodyColor       string  `json:"buttonBodyColor"`
	ButtonBodyXPadding    float64 `json:"buttonBodyXPadding"`
	ButtonBodyYPadding    float64 `json:"buttonBodyYPadding"`
	ButtonBorderColor     string  `json:"buttonBorderColor"`
	ButtonBorderWidth     float64 `json:"buttonBorderWidth"`
	ButtonBorderRadius    float64 `json:"buttonBorderRadius"`
	ButtonTextColor       string  `json:"buttonTextColor"`
	ButtonTextFormat      float64 `json:"buttonTextFormat"`
	ButtonTextFontSize    float64 `json:"buttonTextFontSize"`
	DividerColor          string  `json:"dividerColor"`
	DividerBorderWidth    float64 `json:"dividerBorderWidth"`
	TextBaseColor         string  `json:"textBaseColor"`
	TextBaseFontSize      float64 `json:"textBaseFontSize"`
	TextBaseLineHeight    float64 `json:"textBaseLineHeight"`
	TextBaseLetterSpacing float64 `json:"textBaseLetterSpacing"`
	TextLinkColor         string  `json:"textLinkColor"`
	Heading1Color         string  `json:"heading1Color"`
	Heading1FontSize      float64 `json:"heading1FontSize"`
	Heading1LineHeight    float64 `json:"heading1LineHeight"`
	Heading1LetterSpacing float64 `json:"heading1LetterSpacing"`
	Heading2Color         string  `json:"heading2Color"`
	Heading2FontSize      float64 `json:"heading2FontSize"`
	Heading2LineHeight    float64 `json:"heading2LineHeight"`
	Heading2LetterSpacing float64 `json:"heading2LetterSpacing"`
	Heading3Color         string  `json:"heading3Color"`
	Heading3FontSize      float64 `json:"heading3FontSize"`
	Heading3LineHeight    float64 `json:"heading3LineHeight"`
	Heading3LetterSpacing float64 `json:"heading3LetterSpacing"`
}

// Theme is a named collection of styles applied to email messages. Exactly
// one theme on the account has IsDefault set to true.
type Theme struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Styles    ThemeStyles `json:"styles"`
	IsDefault bool        `json:"isDefault"`
	CreatedAt string      `json:"createdAt"`
	UpdatedAt string      `json:"updatedAt"`
}

// CreateThemeRequest is the request body for [Client.CreateTheme]. Name is
// required; Styles is optional and defaults are applied server-side when
// omitted.
type CreateThemeRequest struct {
	Name   string       `json:"name"`
	Styles *ThemeStyles `json:"styles,omitempty"`
}

// UpdateThemeRequest is the request body for [Client.UpdateTheme]. At least
// one field must be set.
type UpdateThemeRequest struct {
	Name   string       `json:"name,omitempty"`
	Styles *ThemeStyles `json:"styles,omitempty"`
}

// CreateTheme creates a new theme.
func (c *Client) CreateTheme(ctx context.Context, req CreateThemeRequest) (*Theme, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(ctx, http.MethodPost, "/themes", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}

	resp, err := c.do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, errorFromResponse(resp)
	}

	var result Theme
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// UpdateTheme updates the theme identified by id.
func (c *Client) UpdateTheme(ctx context.Context, id string, req UpdateThemeRequest) (*Theme, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(ctx, http.MethodPost, "/themes/"+id, bytes.NewReader(b))
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

	var result Theme
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetTheme returns the theme identified by id.
func (c *Client) GetTheme(ctx context.Context, id string) (*Theme, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/themes/"+id, nil)
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

	var result Theme
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ListThemes returns a single page of themes along with pagination
// information. To iterate every page, use [Paginate].
func (c *Client) ListThemes(ctx context.Context, params PaginationParams) ([]Theme, *Pagination, error) {
	q := url.Values{}
	if params.PerPage != 0 {
		q.Set("perPage", strconv.Itoa(params.PerPage))
	}
	if params.Cursor != "" {
		q.Set("cursor", params.Cursor)
	}

	path := "/themes"
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
		Data       []Theme    `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Data, &result.Pagination, nil
}
