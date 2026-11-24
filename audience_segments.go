package loops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// AudienceSegment is a saved audience filter. The Filter is null for the
// reserved "All contacts" segment.
type AudienceSegment struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description *string         `json:"description"`
	CreatedAt   string          `json:"createdAt"`
	UpdatedAt   string          `json:"updatedAt"`
	Filter      *AudienceFilter `json:"filter"`
}

// AudienceFilter is a tree of [AudienceFilterCondition] values combined with
// Match ("all" or "any"). Used by [AudienceSegment] and by campaign targeting.
type AudienceFilter struct {
	Match      string                    `json:"match"`
	Conditions []AudienceFilterCondition `json:"conditions"`
}

// AudienceFilterCondition is one node in an [AudienceFilter] tree. Exactly
// one of Property, OptIn, or Activity is set; Type is the discriminator and
// matches the variant ("property", "optIn", or "activity").
//
// The condition (de)serializes as a flat object with a "type" field — the
// variant fields are inlined, not nested.
type AudienceFilterCondition struct {
	Type     string
	Property *PropertyCondition
	OptIn    *OptInCondition
	Activity *ActivityCondition
}

const (
	AudienceConditionTypeProperty = "property"
	AudienceConditionTypeOptIn    = "optIn"
	AudienceConditionTypeActivity = "activity"
)

// PropertyCondition matches contacts by the value of a contact property.
type PropertyCondition struct {
	Key      string                  `json:"key"`
	Operator string                  `json:"operator"`
	Value    *PropertyConditionValue `json:"value,omitempty"`
}

// PropertyConditionValue is the right-hand side of a [PropertyCondition].
// Exactly one of String, Number, or Range is set, depending on Operator. For
// most operators a String or Number is used; the "between" operator uses
// Range. Value-less operators (such as "isTrue" or "empty") leave all fields
// zero and should be passed as a nil pointer.
type PropertyConditionValue struct {
	String *string
	Number *float64
	Range  *PropertyConditionRange
}

// PropertyConditionRange is the value for the "between" operator on a
// [PropertyCondition]. Both bounds are ISO 8601 timestamps.
type PropertyConditionRange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// MarshalJSON serializes the value as a string, number, or range object.
func (v PropertyConditionValue) MarshalJSON() ([]byte, error) {
	switch {
	case v.Range != nil:
		return json.Marshal(v.Range)
	case v.Number != nil:
		return json.Marshal(*v.Number)
	case v.String != nil:
		return json.Marshal(*v.String)
	}
	return []byte("null"), nil
}

// UnmarshalJSON decodes a string, number, or range object into the value.
func (v *PropertyConditionValue) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	switch data[0] {
	case '"':
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		v.String = &s
		return nil
	case '{':
		var r PropertyConditionRange
		if err := json.Unmarshal(data, &r); err != nil {
			return err
		}
		v.Range = &r
		return nil
	default:
		var n float64
		if err := json.Unmarshal(data, &n); err != nil {
			return err
		}
		v.Number = &n
		return nil
	}
}

// OptInCondition matches contacts by their opt-in status on a mailing list.
// Status may be "accepted", "pending", "rejected", or nil for any.
type OptInCondition struct {
	Status *string `json:"status"`
}

// ActivityCondition matches contacts by their activity on a campaign,
// workflow, or workflow email.
type ActivityCondition struct {
	Action string `json:"action"`
	Negate bool   `json:"negate"`
	Target string `json:"target"`
	ID     string `json:"id"`
}

// MarshalJSON encodes the active variant inline with a "type" discriminator.
func (c AudienceFilterCondition) MarshalJSON() ([]byte, error) {
	var inner any
	switch c.Type {
	case AudienceConditionTypeProperty:
		inner = c.Property
	case AudienceConditionTypeOptIn:
		inner = c.OptIn
	case AudienceConditionTypeActivity:
		inner = c.Activity
	default:
		return nil, fmt.Errorf("audience condition: unknown type %q", c.Type)
	}
	if inner == nil {
		return nil, fmt.Errorf("audience condition: %s variant is nil", c.Type)
	}
	raw, err := json.Marshal(inner)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	typeRaw, _ := json.Marshal(c.Type)
	fields["type"] = typeRaw
	return json.Marshal(fields)
}

// UnmarshalJSON dispatches on "type" and decodes the matching variant.
func (c *AudienceFilterCondition) UnmarshalJSON(data []byte) error {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return err
	}
	c.Type = head.Type
	switch head.Type {
	case AudienceConditionTypeProperty:
		var v PropertyCondition
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		c.Property = &v
	case AudienceConditionTypeOptIn:
		var v OptInCondition
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		c.OptIn = &v
	case AudienceConditionTypeActivity:
		var v ActivityCondition
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		c.Activity = &v
	default:
		return fmt.Errorf("audience condition: unknown type %q", head.Type)
	}
	return nil
}

// GetAudienceSegment returns the audience segment identified by id.
func (c *Client) GetAudienceSegment(ctx context.Context, id string) (*AudienceSegment, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/audience-segments/"+id, nil)
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

	var result AudienceSegment
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// CreateAudienceSegmentRequest is the body for [Client.CreateAudienceSegment].
// Name must be unique within the team; Filter must have at least one
// condition.
type CreateAudienceSegmentRequest struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Filter      AudienceFilter `json:"filter"`
}

// CreateAudienceSegment creates a new audience segment and returns it.
func (c *Client) CreateAudienceSegment(ctx context.Context, req CreateAudienceSegmentRequest) (*AudienceSegment, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(ctx, http.MethodPost, "/audience-segments", bytes.NewReader(b))
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

	var result AudienceSegment
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ListAudienceSegments returns a single page of audience segments along with
// pagination information. To iterate every page, use [Paginate].
func (c *Client) ListAudienceSegments(ctx context.Context, params PaginationParams) ([]AudienceSegment, *Pagination, error) {
	q := url.Values{}
	if params.PerPage != "" {
		q.Set("perPage", params.PerPage)
	}
	if params.Cursor != "" {
		q.Set("cursor", params.Cursor)
	}

	path := "/audience-segments"
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
		Pagination Pagination        `json:"pagination"`
		Data       []AudienceSegment `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Data, &result.Pagination, nil
}
