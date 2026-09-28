package loops

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// WorkflowSummary is an entry in the [Client.ListWorkflows] response. URL is
// the workflow's URL in the Loops app.
type WorkflowSummary struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// SimplifiedWorkflow is returned by [Client.GetWorkflow]. Nodes is keyed by
// node ID; entry shapes are discriminated by their TypeName. Status is one of
// the WorkflowStatus* constants. WorkflowRevisionID is the current revision
// token; pass it back as the expectedRevisionId on the next mutation (it may
// be nil for workflows without a revision token yet). URL is the workflow's
// URL in the Loops app.
type SimplifiedWorkflow struct {
	ID                 string                            `json:"id"`
	URL                string                            `json:"url"`
	Status             string                            `json:"status,omitempty"`
	WorkflowRevisionID *string                           `json:"workflowRevisionId"`
	Name               string                            `json:"name,omitempty"`
	Description        string                            `json:"description,omitempty"`
	Emoji              string                            `json:"emoji,omitempty"`
	MailingListID      *string                           `json:"mailingListId"`
	RootNodeID         *string                           `json:"rootNodeId"`
	Nodes              map[string]SimplifiedWorkflowNode `json:"nodes"`
}

// Workflow status values, used in [SimplifiedWorkflow].
const (
	WorkflowStatusDraft             = "Draft"
	WorkflowStatusSending           = "Sending"
	WorkflowStatusPaused            = "Paused"
	WorkflowStatusPausedAndQueueing = "PausedAndQueueing"
)

// Workflow node typeName discriminator values, used in [WorkflowNode] and
// [SimplifiedWorkflowNode].
const (
	WorkflowNodeTypeSignupTrigger          = "SignupTrigger"
	WorkflowNodeTypeEventTrigger           = "EventTrigger"
	WorkflowNodeTypeContactPropertyTrigger = "ContactPropertyTrigger"
	WorkflowNodeTypeAddToListTrigger       = "AddToListTrigger"
	WorkflowNodeTypeBlankTrigger           = "BlankTrigger"
	WorkflowNodeTypeAudienceFilter         = "AudienceFilter"
	WorkflowNodeTypeTimerAction            = "TimerAction"
	WorkflowNodeTypeSendEmailAction        = "SendEmailAction"
	WorkflowNodeTypeExitAction             = "ExitAction"
	WorkflowNodeTypeBranchNode             = "BranchNode"
	WorkflowNodeTypeExperimentBranchNode   = "ExperimentBranchNode"
	WorkflowNodeTypeVariantNode            = "VariantNode"
)

// WorkflowTimerUnit is the unit of a [TimerActionWorkflowNode] delay.
type WorkflowTimerUnit string

const (
	WorkflowTimerUnitMinutes WorkflowTimerUnit = "m"
	WorkflowTimerUnitHours   WorkflowTimerUnit = "h"
	WorkflowTimerUnitDays    WorkflowTimerUnit = "d"
)

// WorkflowExperimentType is the kind of experiment driving an
// [ExperimentBranchWorkflowNode].
type WorkflowExperimentType string

const (
	WorkflowExperimentTypeWebhook   WorkflowExperimentType = "webhook"
	WorkflowExperimentTypeAutosplit WorkflowExperimentType = "autosplit"
)

// WorkflowEventProperty describes one event property surfaced by an
// [EventTriggerWorkflowNode]. Type is one of "string", "number", "boolean",
// or "date".
type WorkflowEventProperty struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// WorkflowContactPropertyQuery is the contact-property predicate of a
// [ContactPropertyTriggerWorkflowNode] or
// [SimplifiedContactPropertyTriggerWorkflowNode].
type WorkflowContactPropertyQuery struct {
	Key string                            `json:"key"`
	Is  WorkflowContactPropertyComparison `json:"is"`
	Was WorkflowContactPropertyComparison `json:"was"`
}

// WorkflowContactPropertyComparison is a single comparison in a
// [WorkflowContactPropertyQuery].
type WorkflowContactPropertyComparison struct {
	Value    WorkflowContactPropertyValue `json:"value"`
	Operator string                       `json:"operator"`
}

// WorkflowContactPropertyValue holds the right-hand side of a comparison;
// exactly one of String, Number, or Bool is set depending on the operator.
// Value-less operators may leave all fields zero.
type WorkflowContactPropertyValue struct {
	String *string
	Number *float64
	Bool   *bool
}

// MarshalJSON serializes the value as a string, number, or boolean.
func (v WorkflowContactPropertyValue) MarshalJSON() ([]byte, error) {
	switch {
	case v.String != nil:
		return json.Marshal(*v.String)
	case v.Number != nil:
		return json.Marshal(*v.Number)
	case v.Bool != nil:
		return json.Marshal(*v.Bool)
	}
	return []byte("null"), nil
}

// UnmarshalJSON decodes a string, number, or boolean into the value.
func (v *WorkflowContactPropertyValue) UnmarshalJSON(data []byte) error {
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
	case 't', 'f':
		var b bool
		if err := json.Unmarshal(data, &b); err != nil {
			return err
		}
		v.Bool = &b
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

// SimplifiedWorkflowNode is one entry in [SimplifiedWorkflow.Nodes]. Exactly
// one variant pointer is set; TypeName is the discriminator and matches the
// active variant (see the WorkflowNodeType* constants).
type SimplifiedWorkflowNode struct {
	TypeName               string
	SignupTrigger          *SimplifiedSignupTriggerWorkflowNode
	EventTrigger           *SimplifiedEventTriggerWorkflowNode
	ContactPropertyTrigger *SimplifiedContactPropertyTriggerWorkflowNode
	AddToListTrigger       *SimplifiedAddToListTriggerWorkflowNode
	BlankTrigger           *SimplifiedBlankTriggerWorkflowNode
	AudienceFilter         *SimplifiedAudienceFilterWorkflowNode
	TimerAction            *SimplifiedTimerActionWorkflowNode
	SendEmailAction        *SimplifiedSendEmailActionWorkflowNode
	ExitAction             *SimplifiedExitActionWorkflowNode
	BranchNode             *SimplifiedBranchWorkflowNode
	ExperimentBranchNode   *SimplifiedExperimentBranchWorkflowNode
	VariantNode            *SimplifiedVariantWorkflowNode
}

// SimplifiedSignupTriggerWorkflowNode is the SignupTrigger variant of
// [SimplifiedWorkflowNode].
type SimplifiedSignupTriggerWorkflowNode struct {
	NextNodeIDs []string `json:"nextNodeIds"`
}

// SimplifiedEventTriggerWorkflowNode is the EventTrigger variant of
// [SimplifiedWorkflowNode].
type SimplifiedEventTriggerWorkflowNode struct {
	NextNodeIDs []string `json:"nextNodeIds"`
	EventName   string   `json:"eventName,omitempty"`
	ReEligible  *bool    `json:"reEligible,omitempty"`
}

// SimplifiedContactPropertyTriggerWorkflowNode is the ContactPropertyTrigger
// variant of [SimplifiedWorkflowNode].
type SimplifiedContactPropertyTriggerWorkflowNode struct {
	NextNodeIDs          []string                      `json:"nextNodeIds"`
	ContactPropertyQuery *WorkflowContactPropertyQuery `json:"contactPropertyQuery,omitempty"`
	ReEligible           *bool                         `json:"reEligible,omitempty"`
}

// SimplifiedAddToListTriggerWorkflowNode is the AddToListTrigger variant of
// [SimplifiedWorkflowNode].
type SimplifiedAddToListTriggerWorkflowNode struct {
	NextNodeIDs   []string `json:"nextNodeIds"`
	MailingListID *string  `json:"mailingListId"`
	ReEligible    *bool    `json:"reEligible,omitempty"`
}

// SimplifiedBlankTriggerWorkflowNode is the BlankTrigger variant of
// [SimplifiedWorkflowNode].
type SimplifiedBlankTriggerWorkflowNode struct {
	NextNodeIDs []string `json:"nextNodeIds"`
}

// SimplifiedAudienceFilterWorkflowNode is the AudienceFilter variant of
// [SimplifiedWorkflowNode].
type SimplifiedAudienceFilterWorkflowNode struct {
	NextNodeIDs []string `json:"nextNodeIds"`
}

// SimplifiedTimerActionWorkflowNode is the TimerAction variant of
// [SimplifiedWorkflowNode].
type SimplifiedTimerActionWorkflowNode struct {
	NextNodeIDs []string          `json:"nextNodeIds"`
	Amount      *float64          `json:"amount,omitempty"`
	Unit        WorkflowTimerUnit `json:"unit,omitempty"`
}

// SimplifiedSendEmailActionWorkflowNode is the SendEmailAction variant of
// [SimplifiedWorkflowNode].
type SimplifiedSendEmailActionWorkflowNode struct {
	NextNodeIDs    []string `json:"nextNodeIds"`
	EmailMessageID string   `json:"emailMessageId,omitempty"`
	Subject        string   `json:"subject,omitempty"`
}

// SimplifiedExitActionWorkflowNode is the ExitAction variant of
// [SimplifiedWorkflowNode].
type SimplifiedExitActionWorkflowNode struct {
	NextNodeIDs []string `json:"nextNodeIds"`
}

// SimplifiedBranchWorkflowNode is the BranchNode variant of
// [SimplifiedWorkflowNode].
type SimplifiedBranchWorkflowNode struct {
	NextNodeIDs []string `json:"nextNodeIds"`
}

// SimplifiedExperimentBranchWorkflowNode is the ExperimentBranchNode variant
// of [SimplifiedWorkflowNode].
type SimplifiedExperimentBranchWorkflowNode struct {
	NextNodeIDs    []string               `json:"nextNodeIds"`
	SamplingRate   *float64               `json:"samplingRate,omitempty"`
	URL            string                 `json:"url,omitempty"`
	ExperimentID   string                 `json:"experimentId,omitempty"`
	ExperimentType WorkflowExperimentType `json:"experimentType,omitempty"`
}

// SimplifiedVariantWorkflowNode is the VariantNode variant of
// [SimplifiedWorkflowNode].
type SimplifiedVariantWorkflowNode struct {
	NextNodeIDs []string `json:"nextNodeIds"`
	VariantID   string   `json:"variantId,omitempty"`
	IsControl   *bool    `json:"isControl,omitempty"`
}

// MarshalJSON encodes the active variant inline with a "typeName"
// discriminator.
func (n SimplifiedWorkflowNode) MarshalJSON() ([]byte, error) {
	inner, err := pickSimplifiedWorkflowNode(n)
	if err != nil {
		return nil, err
	}
	return marshalDiscriminated(n.TypeName, inner)
}

// UnmarshalJSON dispatches on "typeName" and decodes the matching variant.
func (n *SimplifiedWorkflowNode) UnmarshalJSON(data []byte) error {
	var head struct {
		TypeName string `json:"typeName"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return err
	}
	n.TypeName = head.TypeName
	switch head.TypeName {
	case WorkflowNodeTypeSignupTrigger:
		var v SimplifiedSignupTriggerWorkflowNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.SignupTrigger = &v
	case WorkflowNodeTypeEventTrigger:
		var v SimplifiedEventTriggerWorkflowNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.EventTrigger = &v
	case WorkflowNodeTypeContactPropertyTrigger:
		var v SimplifiedContactPropertyTriggerWorkflowNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.ContactPropertyTrigger = &v
	case WorkflowNodeTypeAddToListTrigger:
		var v SimplifiedAddToListTriggerWorkflowNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.AddToListTrigger = &v
	case WorkflowNodeTypeBlankTrigger:
		var v SimplifiedBlankTriggerWorkflowNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.BlankTrigger = &v
	case WorkflowNodeTypeAudienceFilter:
		var v SimplifiedAudienceFilterWorkflowNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.AudienceFilter = &v
	case WorkflowNodeTypeTimerAction:
		var v SimplifiedTimerActionWorkflowNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.TimerAction = &v
	case WorkflowNodeTypeSendEmailAction:
		var v SimplifiedSendEmailActionWorkflowNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.SendEmailAction = &v
	case WorkflowNodeTypeExitAction:
		var v SimplifiedExitActionWorkflowNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.ExitAction = &v
	case WorkflowNodeTypeBranchNode:
		var v SimplifiedBranchWorkflowNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.BranchNode = &v
	case WorkflowNodeTypeExperimentBranchNode:
		var v SimplifiedExperimentBranchWorkflowNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.ExperimentBranchNode = &v
	case WorkflowNodeTypeVariantNode:
		var v SimplifiedVariantWorkflowNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.VariantNode = &v
	default:
		return fmt.Errorf("workflow node: unknown typeName %q", head.TypeName)
	}
	return nil
}

func pickSimplifiedWorkflowNode(n SimplifiedWorkflowNode) (any, error) {
	switch n.TypeName {
	case WorkflowNodeTypeSignupTrigger:
		return n.SignupTrigger, nil
	case WorkflowNodeTypeEventTrigger:
		return n.EventTrigger, nil
	case WorkflowNodeTypeContactPropertyTrigger:
		return n.ContactPropertyTrigger, nil
	case WorkflowNodeTypeAddToListTrigger:
		return n.AddToListTrigger, nil
	case WorkflowNodeTypeBlankTrigger:
		return n.BlankTrigger, nil
	case WorkflowNodeTypeAudienceFilter:
		return n.AudienceFilter, nil
	case WorkflowNodeTypeTimerAction:
		return n.TimerAction, nil
	case WorkflowNodeTypeSendEmailAction:
		return n.SendEmailAction, nil
	case WorkflowNodeTypeExitAction:
		return n.ExitAction, nil
	case WorkflowNodeTypeBranchNode:
		return n.BranchNode, nil
	case WorkflowNodeTypeExperimentBranchNode:
		return n.ExperimentBranchNode, nil
	case WorkflowNodeTypeVariantNode:
		return n.VariantNode, nil
	}
	return nil, fmt.Errorf("workflow node: unknown typeName %q", n.TypeName)
}

// WorkflowNode is returned by [Client.GetWorkflowNode]. Exactly one variant
// pointer is set; TypeName is the discriminator and matches the active
// variant (see the WorkflowNodeType* constants).
type WorkflowNode struct {
	TypeName               string
	SignupTrigger          *SignupTriggerWorkflowNode
	EventTrigger           *EventTriggerWorkflowNode
	ContactPropertyTrigger *ContactPropertyTriggerWorkflowNode
	AddToListTrigger       *AddToListTriggerWorkflowNode
	BlankTrigger           *BlankTriggerWorkflowNode
	AudienceFilter         *AudienceFilterWorkflowNode
	TimerAction            *TimerActionWorkflowNode
	SendEmailAction        *SendEmailActionWorkflowNode
	ExitAction             *ExitActionWorkflowNode
	BranchNode             *BranchWorkflowNode
	ExperimentBranchNode   *ExperimentBranchWorkflowNode
	VariantNode            *VariantWorkflowNode
}

// SignupTriggerWorkflowNode is the SignupTrigger variant of [WorkflowNode].
type SignupTriggerWorkflowNode struct {
	ID          string   `json:"id"`
	WorkflowID  string   `json:"workflowId"`
	NextNodeIDs []string `json:"nextNodeIds"`
}

// EventTriggerWorkflowNode is the EventTrigger variant of [WorkflowNode].
// EventName is nullable; EventProperties may be empty.
type EventTriggerWorkflowNode struct {
	ID              string                  `json:"id"`
	WorkflowID      string                  `json:"workflowId"`
	NextNodeIDs     []string                `json:"nextNodeIds"`
	EventName       *string                 `json:"eventName"`
	EventProperties []WorkflowEventProperty `json:"eventProperties,omitempty"`
	ReEligible      bool                    `json:"reEligible"`
}

// ContactPropertyTriggerWorkflowNode is the ContactPropertyTrigger variant
// of [WorkflowNode]. ContactPropertyQuery is required by the spec but may be
// null.
type ContactPropertyTriggerWorkflowNode struct {
	ID                   string                        `json:"id"`
	WorkflowID           string                        `json:"workflowId"`
	NextNodeIDs          []string                      `json:"nextNodeIds"`
	ContactPropertyQuery *WorkflowContactPropertyQuery `json:"contactPropertyQuery"`
	ReEligible           bool                          `json:"reEligible"`
}

// AddToListTriggerWorkflowNode is the AddToListTrigger variant of
// [WorkflowNode].
type AddToListTriggerWorkflowNode struct {
	ID            string   `json:"id"`
	WorkflowID    string   `json:"workflowId"`
	NextNodeIDs   []string `json:"nextNodeIds"`
	MailingListID *string  `json:"mailingListId"`
	ReEligible    bool     `json:"reEligible"`
}

// BlankTriggerWorkflowNode is the BlankTrigger variant of [WorkflowNode].
type BlankTriggerWorkflowNode struct {
	ID          string   `json:"id"`
	WorkflowID  string   `json:"workflowId"`
	NextNodeIDs []string `json:"nextNodeIds"`
}

// AudienceFilterWorkflowNode is the AudienceFilter variant of [WorkflowNode].
// AudienceFilter reuses the package-level [AudienceFilter] type.
type AudienceFilterWorkflowNode struct {
	ID                string          `json:"id"`
	WorkflowID        string          `json:"workflowId"`
	NextNodeIDs       []string        `json:"nextNodeIds"`
	AudienceFilter    *AudienceFilter `json:"audienceFilter,omitempty"`
	AudienceSegmentID string          `json:"audienceSegmentId,omitempty"`
	AppliesDownstream bool            `json:"appliesDownstream"`
}

// TimerActionWorkflowNode is the TimerAction variant of [WorkflowNode].
type TimerActionWorkflowNode struct {
	ID          string            `json:"id"`
	WorkflowID  string            `json:"workflowId"`
	NextNodeIDs []string          `json:"nextNodeIds"`
	Amount      float64           `json:"amount"`
	Unit        WorkflowTimerUnit `json:"unit"`
}

// SendEmailActionWorkflowNode is the SendEmailAction variant of [WorkflowNode].
type SendEmailActionWorkflowNode struct {
	ID             string   `json:"id"`
	WorkflowID     string   `json:"workflowId"`
	NextNodeIDs    []string `json:"nextNodeIds"`
	EmailMessageID string   `json:"emailMessageId"`
	Subject        string   `json:"subject"`
}

// ExitActionWorkflowNode is the ExitAction variant of [WorkflowNode].
type ExitActionWorkflowNode struct {
	ID          string   `json:"id"`
	WorkflowID  string   `json:"workflowId"`
	NextNodeIDs []string `json:"nextNodeIds"`
}

// BranchWorkflowNode is the BranchNode variant of [WorkflowNode].
type BranchWorkflowNode struct {
	ID           string   `json:"id"`
	WorkflowID   string   `json:"workflowId"`
	NextNodeIDs  []string `json:"nextNodeIds"`
	EvalStrategy string   `json:"evalStrategy,omitempty"`
}

// ExperimentBranchWorkflowNode is the ExperimentBranchNode variant of
// [WorkflowNode].
type ExperimentBranchWorkflowNode struct {
	ID             string                 `json:"id"`
	WorkflowID     string                 `json:"workflowId"`
	NextNodeIDs    []string               `json:"nextNodeIds"`
	SamplingRate   float64                `json:"samplingRate"`
	URL            string                 `json:"url,omitempty"`
	ExperimentID   string                 `json:"experimentId,omitempty"`
	ExperimentType WorkflowExperimentType `json:"experimentType"`
}

// VariantWorkflowNode is the VariantNode variant of [WorkflowNode].
type VariantWorkflowNode struct {
	ID          string   `json:"id"`
	WorkflowID  string   `json:"workflowId"`
	NextNodeIDs []string `json:"nextNodeIds"`
	VariantID   string   `json:"variantId,omitempty"`
	IsControl   *bool    `json:"isControl,omitempty"`
}

// MarshalJSON encodes the active variant inline with a "typeName"
// discriminator.
func (n WorkflowNode) MarshalJSON() ([]byte, error) {
	inner, err := pickWorkflowNode(n)
	if err != nil {
		return nil, err
	}
	return marshalDiscriminated(n.TypeName, inner)
}

// UnmarshalJSON dispatches on "typeName" and decodes the matching variant.
func (n *WorkflowNode) UnmarshalJSON(data []byte) error {
	var head struct {
		TypeName string `json:"typeName"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return err
	}
	n.TypeName = head.TypeName
	switch head.TypeName {
	case WorkflowNodeTypeSignupTrigger:
		var v SignupTriggerWorkflowNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.SignupTrigger = &v
	case WorkflowNodeTypeEventTrigger:
		var v EventTriggerWorkflowNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.EventTrigger = &v
	case WorkflowNodeTypeContactPropertyTrigger:
		var v ContactPropertyTriggerWorkflowNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.ContactPropertyTrigger = &v
	case WorkflowNodeTypeAddToListTrigger:
		var v AddToListTriggerWorkflowNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.AddToListTrigger = &v
	case WorkflowNodeTypeBlankTrigger:
		var v BlankTriggerWorkflowNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.BlankTrigger = &v
	case WorkflowNodeTypeAudienceFilter:
		var v AudienceFilterWorkflowNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.AudienceFilter = &v
	case WorkflowNodeTypeTimerAction:
		var v TimerActionWorkflowNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.TimerAction = &v
	case WorkflowNodeTypeSendEmailAction:
		var v SendEmailActionWorkflowNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.SendEmailAction = &v
	case WorkflowNodeTypeExitAction:
		var v ExitActionWorkflowNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.ExitAction = &v
	case WorkflowNodeTypeBranchNode:
		var v BranchWorkflowNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.BranchNode = &v
	case WorkflowNodeTypeExperimentBranchNode:
		var v ExperimentBranchWorkflowNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.ExperimentBranchNode = &v
	case WorkflowNodeTypeVariantNode:
		var v VariantWorkflowNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.VariantNode = &v
	default:
		return fmt.Errorf("workflow node: unknown typeName %q", head.TypeName)
	}
	return nil
}

func pickWorkflowNode(n WorkflowNode) (any, error) {
	switch n.TypeName {
	case WorkflowNodeTypeSignupTrigger:
		return n.SignupTrigger, nil
	case WorkflowNodeTypeEventTrigger:
		return n.EventTrigger, nil
	case WorkflowNodeTypeContactPropertyTrigger:
		return n.ContactPropertyTrigger, nil
	case WorkflowNodeTypeAddToListTrigger:
		return n.AddToListTrigger, nil
	case WorkflowNodeTypeBlankTrigger:
		return n.BlankTrigger, nil
	case WorkflowNodeTypeAudienceFilter:
		return n.AudienceFilter, nil
	case WorkflowNodeTypeTimerAction:
		return n.TimerAction, nil
	case WorkflowNodeTypeSendEmailAction:
		return n.SendEmailAction, nil
	case WorkflowNodeTypeExitAction:
		return n.ExitAction, nil
	case WorkflowNodeTypeBranchNode:
		return n.BranchNode, nil
	case WorkflowNodeTypeExperimentBranchNode:
		return n.ExperimentBranchNode, nil
	case WorkflowNodeTypeVariantNode:
		return n.VariantNode, nil
	}
	return nil, fmt.Errorf("workflow node: unknown typeName %q", n.TypeName)
}

func marshalDiscriminated(typeName string, inner any) ([]byte, error) {
	if typeName == "" {
		return nil, fmt.Errorf("workflow node: typeName is empty")
	}
	raw, err := json.Marshal(inner)
	if err != nil {
		return nil, err
	}
	// A typed-nil variant pointer is a non-nil interface but marshals to
	// "null"; treat that (and an untyped nil) as a missing variant instead of
	// panicking on the nil map below.
	if string(raw) == "null" {
		return nil, fmt.Errorf("workflow node: %s variant is nil", typeName)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	tn, _ := json.Marshal(typeName)
	fields["typeName"] = tn
	return json.Marshal(fields)
}

// mergeMarshal marshals inner (which has its own MarshalJSON) into a JSON
// object and overlays the extra fields. Used by the revision-bearing response
// types, whose promoted embedded MarshalJSON would otherwise drop their own
// fields.
func mergeMarshal(inner any, extra map[string]any) ([]byte, error) {
	raw, err := json.Marshal(inner)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	for k, v := range extra {
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		fields[k] = b
	}
	return json.Marshal(fields)
}

// ListWorkflows returns a single page of workflow summaries along with
// pagination information. To iterate every page, use [Paginate].
func (c *Client) ListWorkflows(params PaginationParams) ([]WorkflowSummary, *Pagination, error) {
	q := url.Values{}
	if params.PerPage != "" {
		q.Set("perPage", params.PerPage)
	}
	if params.Cursor != "" {
		q.Set("cursor", params.Cursor)
	}

	path := "/workflows"
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
		Pagination Pagination        `json:"pagination"`
		Data       []WorkflowSummary `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Data, &result.Pagination, nil
}

// GetWorkflow returns the simplified workflow graph identified by id.
func (c *Client) GetWorkflow(id string) (*SimplifiedWorkflow, error) {
	req, err := c.newRequest(http.MethodGet, "/workflows/"+id, nil)
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

	var result SimplifiedWorkflow
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// WorkflowNodeWithRevision is returned by [Client.GetWorkflowNode]. It is the
// full [WorkflowNode] (its fields stay flat via the embedded type) plus the
// current workflow revision token. WorkflowRevisionID may be nil for workflows
// without a revision token yet.
type WorkflowNodeWithRevision struct {
	WorkflowNode
	WorkflowRevisionID *string `json:"workflowRevisionId"`
}

// UnmarshalJSON decodes the node fields via the embedded [WorkflowNode] and
// then the revision token, since Go does not promote the embedded type's
// UnmarshalJSON when the outer struct has its own fields.
func (n *WorkflowNodeWithRevision) UnmarshalJSON(data []byte) error {
	if err := n.WorkflowNode.UnmarshalJSON(data); err != nil {
		return err
	}
	var rev struct {
		WorkflowRevisionID *string `json:"workflowRevisionId"`
	}
	if err := json.Unmarshal(data, &rev); err != nil {
		return err
	}
	n.WorkflowRevisionID = rev.WorkflowRevisionID
	return nil
}

// MarshalJSON encodes the node fields via the embedded [WorkflowNode] and adds
// the revision token, which the promoted embedded MarshalJSON would otherwise
// drop.
func (n WorkflowNodeWithRevision) MarshalJSON() ([]byte, error) {
	return mergeMarshal(n.WorkflowNode, map[string]any{"workflowRevisionId": n.WorkflowRevisionID})
}

// GetWorkflowNode returns the detailed data for a single workflow node along
// with the current workflow revision token. The result is a
// [WorkflowNodeWithRevision]; the node fields remain accessible via the
// embedded [WorkflowNode].
func (c *Client) GetWorkflowNode(workflowID, nodeID string) (*WorkflowNodeWithRevision, error) {
	req, err := c.newRequest(http.MethodGet, "/workflows/"+workflowID+"/nodes/"+nodeID, nil)
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

	var result WorkflowNodeWithRevision
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetWorkflowNodeMetrics returns all-time email engagement metrics for the
// SendEmailAction node identified by nodeID in the workflow identified by
// workflowID. The server responds with a 400 error for nodes of any other
// type or for a node with no email message. A SendEmailAction node that has
// not sent anything yet reports every counter as 0.
func (c *Client) GetWorkflowNodeMetrics(workflowID, nodeID string) (*EmailMetrics, error) {
	req, err := c.newRequest(http.MethodGet, "/workflows/"+workflowID+"/nodes/"+nodeID+"/metrics", nil)
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

// WorkflowMutationNode is the detailed node returned by workflow mutations
// (create, update, add-branch). Unlike [WorkflowNode] it omits workflowId and
// its variant fields differ. Exactly one variant pointer is set; TypeName is
// the discriminator and matches the active variant (see the WorkflowNodeType*
// constants).
type WorkflowMutationNode struct {
	TypeName               string
	SignupTrigger          *SignupTriggerWorkflowMutationNode
	EventTrigger           *EventTriggerWorkflowMutationNode
	ContactPropertyTrigger *ContactPropertyTriggerWorkflowMutationNode
	AddToListTrigger       *AddToListTriggerWorkflowMutationNode
	BlankTrigger           *BlankTriggerWorkflowMutationNode
	AudienceFilter         *AudienceFilterWorkflowMutationNode
	TimerAction            *TimerActionWorkflowMutationNode
	SendEmailAction        *SendEmailActionWorkflowMutationNode
	ExitAction             *ExitActionWorkflowMutationNode
	BranchNode             *BranchWorkflowMutationNode
	ExperimentBranchNode   *ExperimentBranchWorkflowMutationNode
	VariantNode            *VariantWorkflowMutationNode
}

// SignupTriggerWorkflowMutationNode is the SignupTrigger variant of
// [WorkflowMutationNode].
type SignupTriggerWorkflowMutationNode struct {
	ID          string   `json:"id"`
	NextNodeIDs []string `json:"nextNodeIds"`
}

// EventTriggerWorkflowMutationNode is the EventTrigger variant of
// [WorkflowMutationNode].
type EventTriggerWorkflowMutationNode struct {
	ID              string                  `json:"id"`
	NextNodeIDs     []string                `json:"nextNodeIds"`
	EventName       string                  `json:"eventName,omitempty"`
	EventProperties []WorkflowEventProperty `json:"eventProperties,omitempty"`
	ReEligible      bool                    `json:"reEligible"`
}

// ContactPropertyTriggerWorkflowMutationNode is the ContactPropertyTrigger
// variant of [WorkflowMutationNode]. ContactPropertyQuery may be null.
type ContactPropertyTriggerWorkflowMutationNode struct {
	ID                   string                        `json:"id"`
	NextNodeIDs          []string                      `json:"nextNodeIds"`
	ContactPropertyQuery *WorkflowContactPropertyQuery `json:"contactPropertyQuery"`
	ReEligible           bool                          `json:"reEligible"`
}

// AddToListTriggerWorkflowMutationNode is the AddToListTrigger variant of
// [WorkflowMutationNode].
type AddToListTriggerWorkflowMutationNode struct {
	ID            string   `json:"id"`
	NextNodeIDs   []string `json:"nextNodeIds"`
	MailingListID *string  `json:"mailingListId"`
	ReEligible    bool     `json:"reEligible"`
}

// BlankTriggerWorkflowMutationNode is the BlankTrigger variant of
// [WorkflowMutationNode].
type BlankTriggerWorkflowMutationNode struct {
	ID          string   `json:"id"`
	NextNodeIDs []string `json:"nextNodeIds"`
}

// AudienceFilterWorkflowMutationNode is the AudienceFilter variant of
// [WorkflowMutationNode]. AudienceFilter reuses the package-level
// [AudienceFilter] type.
type AudienceFilterWorkflowMutationNode struct {
	ID                string          `json:"id"`
	NextNodeIDs       []string        `json:"nextNodeIds"`
	AudienceFilter    *AudienceFilter `json:"audienceFilter,omitempty"`
	AudienceSegmentID string          `json:"audienceSegmentId,omitempty"`
	AppliesDownstream bool            `json:"appliesDownstream"`
}

// TimerActionWorkflowMutationNode is the TimerAction variant of
// [WorkflowMutationNode].
type TimerActionWorkflowMutationNode struct {
	ID          string            `json:"id"`
	NextNodeIDs []string          `json:"nextNodeIds"`
	Amount      float64           `json:"amount"`
	Unit        WorkflowTimerUnit `json:"unit"`
}

// SendEmailActionWorkflowMutationNode is the SendEmailAction variant of
// [WorkflowMutationNode].
type SendEmailActionWorkflowMutationNode struct {
	ID             string   `json:"id"`
	NextNodeIDs    []string `json:"nextNodeIds"`
	EmailMessageID string   `json:"emailMessageId"`
	Subject        string   `json:"subject"`
}

// ExitActionWorkflowMutationNode is the ExitAction variant of
// [WorkflowMutationNode].
type ExitActionWorkflowMutationNode struct {
	ID          string   `json:"id"`
	NextNodeIDs []string `json:"nextNodeIds"`
}

// BranchWorkflowMutationNode is the BranchNode variant of
// [WorkflowMutationNode].
type BranchWorkflowMutationNode struct {
	ID          string   `json:"id"`
	NextNodeIDs []string `json:"nextNodeIds"`
}

// ExperimentBranchWorkflowMutationNode is the ExperimentBranchNode variant of
// [WorkflowMutationNode].
type ExperimentBranchWorkflowMutationNode struct {
	ID           string   `json:"id"`
	NextNodeIDs  []string `json:"nextNodeIds"`
	SamplingRate float64  `json:"samplingRate"`
}

// VariantWorkflowMutationNode is the VariantNode variant of
// [WorkflowMutationNode].
type VariantWorkflowMutationNode struct {
	ID          string   `json:"id"`
	NextNodeIDs []string `json:"nextNodeIds"`
	IsControl   *bool    `json:"isControl,omitempty"`
}

// MarshalJSON encodes the active variant inline with a "typeName"
// discriminator.
func (n WorkflowMutationNode) MarshalJSON() ([]byte, error) {
	inner, err := pickWorkflowMutationNode(n)
	if err != nil {
		return nil, err
	}
	return marshalDiscriminated(n.TypeName, inner)
}

// UnmarshalJSON dispatches on "typeName" and decodes the matching variant.
func (n *WorkflowMutationNode) UnmarshalJSON(data []byte) error {
	var head struct {
		TypeName string `json:"typeName"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return err
	}
	n.TypeName = head.TypeName
	switch head.TypeName {
	case WorkflowNodeTypeSignupTrigger:
		var v SignupTriggerWorkflowMutationNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.SignupTrigger = &v
	case WorkflowNodeTypeEventTrigger:
		var v EventTriggerWorkflowMutationNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.EventTrigger = &v
	case WorkflowNodeTypeContactPropertyTrigger:
		var v ContactPropertyTriggerWorkflowMutationNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.ContactPropertyTrigger = &v
	case WorkflowNodeTypeAddToListTrigger:
		var v AddToListTriggerWorkflowMutationNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.AddToListTrigger = &v
	case WorkflowNodeTypeBlankTrigger:
		var v BlankTriggerWorkflowMutationNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.BlankTrigger = &v
	case WorkflowNodeTypeAudienceFilter:
		var v AudienceFilterWorkflowMutationNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.AudienceFilter = &v
	case WorkflowNodeTypeTimerAction:
		var v TimerActionWorkflowMutationNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.TimerAction = &v
	case WorkflowNodeTypeSendEmailAction:
		var v SendEmailActionWorkflowMutationNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.SendEmailAction = &v
	case WorkflowNodeTypeExitAction:
		var v ExitActionWorkflowMutationNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.ExitAction = &v
	case WorkflowNodeTypeBranchNode:
		var v BranchWorkflowMutationNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.BranchNode = &v
	case WorkflowNodeTypeExperimentBranchNode:
		var v ExperimentBranchWorkflowMutationNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.ExperimentBranchNode = &v
	case WorkflowNodeTypeVariantNode:
		var v VariantWorkflowMutationNode
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		n.VariantNode = &v
	default:
		return fmt.Errorf("workflow node: unknown typeName %q", head.TypeName)
	}
	return nil
}

func pickWorkflowMutationNode(n WorkflowMutationNode) (any, error) {
	switch n.TypeName {
	case WorkflowNodeTypeSignupTrigger:
		return n.SignupTrigger, nil
	case WorkflowNodeTypeEventTrigger:
		return n.EventTrigger, nil
	case WorkflowNodeTypeContactPropertyTrigger:
		return n.ContactPropertyTrigger, nil
	case WorkflowNodeTypeAddToListTrigger:
		return n.AddToListTrigger, nil
	case WorkflowNodeTypeBlankTrigger:
		return n.BlankTrigger, nil
	case WorkflowNodeTypeAudienceFilter:
		return n.AudienceFilter, nil
	case WorkflowNodeTypeTimerAction:
		return n.TimerAction, nil
	case WorkflowNodeTypeSendEmailAction:
		return n.SendEmailAction, nil
	case WorkflowNodeTypeExitAction:
		return n.ExitAction, nil
	case WorkflowNodeTypeBranchNode:
		return n.BranchNode, nil
	case WorkflowNodeTypeExperimentBranchNode:
		return n.ExperimentBranchNode, nil
	case WorkflowNodeTypeVariantNode:
		return n.VariantNode, nil
	}
	return nil, fmt.Errorf("workflow node: unknown typeName %q", n.TypeName)
}

// WorkflowMutationNodeWithRevision is a [WorkflowMutationNode] returned by a
// mutation (its fields stay flat via the embedded type) plus the current
// workflow revision token.
type WorkflowMutationNodeWithRevision struct {
	WorkflowMutationNode
	WorkflowRevisionID string `json:"workflowRevisionId"`
}

// UnmarshalJSON decodes the node fields via the embedded
// [WorkflowMutationNode] and then the revision token.
func (n *WorkflowMutationNodeWithRevision) UnmarshalJSON(data []byte) error {
	if err := n.WorkflowMutationNode.UnmarshalJSON(data); err != nil {
		return err
	}
	var rev struct {
		WorkflowRevisionID string `json:"workflowRevisionId"`
	}
	if err := json.Unmarshal(data, &rev); err != nil {
		return err
	}
	n.WorkflowRevisionID = rev.WorkflowRevisionID
	return nil
}

// MarshalJSON encodes the node fields via the embedded [WorkflowMutationNode]
// and adds the revision token.
func (n WorkflowMutationNodeWithRevision) MarshalJSON() ([]byte, error) {
	return mergeMarshal(n.WorkflowMutationNode, map[string]any{"workflowRevisionId": n.WorkflowRevisionID})
}

// CreatedWorkflowNode is the node returned by [Client.CreateWorkflowNode]. It
// is a [WorkflowMutationNodeWithRevision] (fields stay flat via the embedded
// type) plus any default child nodes created alongside the requested node —
// a BranchNode returns two AudienceFilter children; an ExperimentBranchNode
// returns two variant children and one control variant.
type CreatedWorkflowNode struct {
	WorkflowMutationNodeWithRevision
	CreatedChildNodes []WorkflowMutationNode `json:"createdChildNodes,omitempty"`
}

// UnmarshalJSON decodes the node and revision via the embedded type and then
// the createdChildNodes list.
func (n *CreatedWorkflowNode) UnmarshalJSON(data []byte) error {
	if err := n.WorkflowMutationNodeWithRevision.UnmarshalJSON(data); err != nil {
		return err
	}
	var extra struct {
		CreatedChildNodes []WorkflowMutationNode `json:"createdChildNodes"`
	}
	if err := json.Unmarshal(data, &extra); err != nil {
		return err
	}
	n.CreatedChildNodes = extra.CreatedChildNodes
	return nil
}

// MarshalJSON encodes the embedded [WorkflowMutationNodeWithRevision] and adds
// the createdChildNodes list when present.
func (n CreatedWorkflowNode) MarshalJSON() ([]byte, error) {
	extra := map[string]any{}
	if len(n.CreatedChildNodes) > 0 {
		extra["createdChildNodes"] = n.CreatedChildNodes
	}
	return mergeMarshal(n.WorkflowMutationNodeWithRevision, extra)
}

// CreateWorkflowRequest is the request body for [Client.CreateWorkflow].
// MailingListID is nullable.
type CreateWorkflowRequest struct {
	Name          string  `json:"name"`
	Description   string  `json:"description,omitempty"`
	MailingListID *string `json:"mailingListId"`
}

// CreateWorkflow creates a new workflow and returns it.
func (c *Client) CreateWorkflow(req CreateWorkflowRequest) (*SimplifiedWorkflow, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(http.MethodPost, "/workflows", bytes.NewReader(b))
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

	var result SimplifiedWorkflow
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// UpdateWorkflowPropertiesRequest is the request body for
// [Client.UpdateWorkflow]. ExpectedRevisionID is required for optimistic
// concurrency and is always sent (as JSON null when nil); a stale value
// returns a 409. At least one of Name or Description should be set.
type UpdateWorkflowPropertiesRequest struct {
	ExpectedRevisionID *string
	Name               string
	Description        string
}

// UpdateWorkflow updates the name and/or description of the workflow
// identified by id and returns its new state.
func (c *Client) UpdateWorkflow(id string, req UpdateWorkflowPropertiesRequest) (*SimplifiedWorkflow, error) {
	body := map[string]any{
		"expectedRevisionId": req.ExpectedRevisionID,
	}
	if req.Name != "" {
		body["name"] = req.Name
	}
	if req.Description != "" {
		body["description"] = req.Description
	}

	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(http.MethodPost, "/workflows/"+id, bytes.NewReader(b))
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

	var result SimplifiedWorkflow
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// Queued-contact policies controlling how a mutation treats queued contacts
// that would be excluded by the change.
const (
	// WorkflowQueuedContactPolicyFail returns queued-contact impact instead of
	// mutating. It is the default when omitted.
	WorkflowQueuedContactPolicyFail = "fail"
	// WorkflowQueuedContactPolicyDiscard confirms that matching queued
	// contacts should be discarded.
	WorkflowQueuedContactPolicyDiscard = "discard"
)

// Workflow mailing-list and node-deletion status values, returned in
// [ChangeWorkflowMailingListResponse] and [DeleteWorkflowNodeResponse].
const (
	WorkflowMutationStatusDryRun              = "dryRun"
	WorkflowMutationStatusQueuedContactsFound = "queuedContactsFound"
	WorkflowMutationStatusUpdated             = "updated"
	WorkflowMutationStatusDeleted             = "deleted"
)

// ChangeWorkflowMailingListRequest is the request body for
// [Client.ChangeWorkflowMailingList]. ExpectedRevisionID and MailingListID are
// both required and nullable and always sent (as JSON null when nil).
type ChangeWorkflowMailingListRequest struct {
	ExpectedRevisionID  *string
	MailingListID       *string
	DryRun              bool
	QueuedContactPolicy string
}

// ChangeWorkflowMailingListResponse is returned by
// [Client.ChangeWorkflowMailingList]. Status is one of the
// WorkflowMutationStatus* values. WorkflowRevisionID and Workflow are set only
// when Status is "updated".
type ChangeWorkflowMailingListResponse struct {
	Status             string              `json:"status"`
	MailingListID      *string             `json:"mailingListId"`
	WorkflowRevisionID *string             `json:"workflowRevisionId,omitempty"`
	QueuedContactCount float64             `json:"queuedContactCount"`
	Workflow           *SimplifiedWorkflow `json:"workflow,omitempty"`
}

// ChangeWorkflowMailingList changes the mailing list of the workflow
// identified by id. With the default "fail" policy (or DryRun), a response
// whose Status is not "updated" reports the queued-contact impact instead of
// applying the change.
func (c *Client) ChangeWorkflowMailingList(id string, req ChangeWorkflowMailingListRequest) (*ChangeWorkflowMailingListResponse, error) {
	body := map[string]any{
		"expectedRevisionId": req.ExpectedRevisionID,
		"mailingListId":      req.MailingListID,
	}
	if req.DryRun {
		body["dryRun"] = req.DryRun
	}
	if req.QueuedContactPolicy != "" {
		body["queuedContactPolicy"] = req.QueuedContactPolicy
	}

	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(http.MethodPost, "/workflows/"+id+"/mailing-list", bytes.NewReader(b))
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

	var result ChangeWorkflowMailingListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// Workflow node insert modes for [CreateWorkflowNodeRequest].
const (
	WorkflowInsertModeBetween = "between"
	WorkflowInsertModeBefore  = "before"
	WorkflowInsertModeAfter   = "after"
)

// Node types that can be created via [Client.CreateWorkflowNode]. Trigger and
// ExitAction nodes cannot be created.
const (
	CreateWorkflowNodeTypeAudienceFilter       = WorkflowNodeTypeAudienceFilter
	CreateWorkflowNodeTypeBranchNode           = WorkflowNodeTypeBranchNode
	CreateWorkflowNodeTypeExperimentBranchNode = WorkflowNodeTypeExperimentBranchNode
	CreateWorkflowNodeTypeTimerAction          = WorkflowNodeTypeTimerAction
	CreateWorkflowNodeTypeSendEmailAction      = WorkflowNodeTypeSendEmailAction
	CreateWorkflowNodeTypeVariantNode          = WorkflowNodeTypeVariantNode
)

// CreateWorkflowNodeRequest is the request body for [Client.CreateWorkflowNode].
// InsertMode selects the placement:
//   - "between" inserts between FromNodeID and ToNodeID (FromNodeID must
//     currently point to ToNodeID); both are required.
//   - "before" inserts before ToNodeID, which must have an incoming parent and
//     cannot be a trigger node.
//   - "after" inserts after FromNodeID, valid only when FromNodeID has exactly
//     one outgoing node (not zero, not multiple, and not an exit node). Use
//     "between" with an explicit ToNodeID when the source has multiple outputs.
//
// ExpectedRevisionID is required and always sent (as JSON null when nil).
// NodeTypeName is one of the CreateWorkflowNodeType* constants.
type CreateWorkflowNodeRequest struct {
	ExpectedRevisionID *string
	InsertMode         string
	NodeTypeName       string
	FromNodeID         string
	ToNodeID           string

	// Deprecated: the API now expects ToNodeID for "before" mode. Set ToNodeID
	// instead; BeforeNodeID is only sent when ToNodeID is empty.
	BeforeNodeID string
}

// CreateWorkflowNodeResponse is returned by [Client.CreateWorkflowNode].
type CreateWorkflowNodeResponse struct {
	Node     CreatedWorkflowNode `json:"node"`
	Workflow SimplifiedWorkflow  `json:"workflow"`
}

// CreateWorkflowNode creates a node in the workflow identified by id and
// returns the created node together with the updated workflow. Configure the
// node afterwards with [Client.UpdateWorkflowNode].
func (c *Client) CreateWorkflowNode(id string, req CreateWorkflowNodeRequest) (*CreateWorkflowNodeResponse, error) {
	body := map[string]any{
		"expectedRevisionId": req.ExpectedRevisionID,
		"insertMode":         req.InsertMode,
		"nodeTypeName":       req.NodeTypeName,
	}
	switch req.InsertMode {
	case WorkflowInsertModeBetween:
		body["fromNodeId"] = req.FromNodeID
		body["toNodeId"] = req.ToNodeID
	case WorkflowInsertModeBefore:
		if req.ToNodeID != "" {
			body["toNodeId"] = req.ToNodeID
		} else {
			body["beforeNodeId"] = req.BeforeNodeID
		}
	case WorkflowInsertModeAfter:
		body["fromNodeId"] = req.FromNodeID
	}

	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(http.MethodPost, "/workflows/"+id+"/nodes", bytes.NewReader(b))
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

	var result CreateWorkflowNodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// UpdateWorkflowNodePayload is the node-type-specific configuration applied by
// [Client.UpdateWorkflowNode]. Exactly one variant pointer is set; TypeName is
// the discriminator and matches the active variant (see the WorkflowNodeType*
// constants). Trigger payloads emit a "typeName" field (which may change one
// trigger type to another); the AudienceFilter, TimerAction, ExperimentBranch,
// and Variant config payloads do not.
type UpdateWorkflowNodePayload struct {
	TypeName               string
	SignupTrigger          *WorkflowSignupTriggerPayload
	EventTrigger           *WorkflowEventTriggerPayload
	ContactPropertyTrigger *WorkflowContactPropertyTriggerPayload
	AddToListTrigger       *WorkflowAddToListTriggerPayload
	AudienceFilter         *WorkflowAudienceFilterPayload
	TimerAction            *WorkflowTimerActionPayload
	ExperimentBranch       *WorkflowExperimentBranchPayload
	Variant                *WorkflowVariantPayload
}

// WorkflowSignupTriggerPayload changes an existing trigger node to a signup
// trigger.
type WorkflowSignupTriggerPayload struct{}

// WorkflowEventTriggerPayload updates an event trigger, or changes a trigger
// node to an event trigger. Set EventPatternID or EventName (not both); either
// may be explicit JSON null to clear the relationship.
type WorkflowEventTriggerPayload struct {
	EventPatternID *string `json:"eventPatternId,omitempty"`
	EventName      *string `json:"eventName,omitempty"`
	ReEligible     *bool   `json:"reEligible,omitempty"`
}

// WorkflowContactPropertyTriggerPayload updates a contact-property trigger, or
// changes a trigger node to one. ContactPropertyQuery reuses the package-level
// [WorkflowContactPropertyQuery] type.
type WorkflowContactPropertyTriggerPayload struct {
	ContactPropertyQuery *WorkflowContactPropertyQuery `json:"contactPropertyQuery,omitempty"`
	ReEligible           *bool                         `json:"reEligible,omitempty"`
}

// WorkflowAddToListTriggerPayload updates an add-to-list trigger, or changes a
// trigger node to one.
type WorkflowAddToListTriggerPayload struct {
	ReEligible *bool `json:"reEligible,omitempty"`
}

// WorkflowAudienceFilterPayload configures an audience filter node.
// AudienceFilter reuses the package-level [AudienceFilter] type.
type WorkflowAudienceFilterPayload struct {
	AudienceSegmentID *string         `json:"audienceSegmentId,omitempty"`
	AudienceFilter    *AudienceFilter `json:"audienceFilter,omitempty"`
	AppliesDownstream *bool           `json:"appliesDownstream,omitempty"`
}

// WorkflowTimerActionPayload configures a timer action node.
type WorkflowTimerActionPayload struct {
	Amount *float64          `json:"amount,omitempty"`
	Unit   WorkflowTimerUnit `json:"unit,omitempty"`
}

// WorkflowExperimentBranchPayload configures an experiment branch node.
// SamplingRate is a percentage between 0 and 100.
type WorkflowExperimentBranchPayload struct {
	SamplingRate *float64 `json:"samplingRate,omitempty"`
}

// WorkflowVariantPayload configures a variant node.
type WorkflowVariantPayload struct {
	IsControl *bool `json:"isControl,omitempty"`
}

// MarshalJSON encodes the active payload variant. Trigger variants include a
// "typeName" discriminator; config variants are emitted as-is.
func (p UpdateWorkflowNodePayload) MarshalJSON() ([]byte, error) {
	switch p.TypeName {
	case WorkflowNodeTypeSignupTrigger:
		return marshalDiscriminated(p.TypeName, p.SignupTrigger)
	case WorkflowNodeTypeEventTrigger:
		return marshalDiscriminated(p.TypeName, p.EventTrigger)
	case WorkflowNodeTypeContactPropertyTrigger:
		return marshalDiscriminated(p.TypeName, p.ContactPropertyTrigger)
	case WorkflowNodeTypeAddToListTrigger:
		return marshalDiscriminated(p.TypeName, p.AddToListTrigger)
	case WorkflowNodeTypeAudienceFilter:
		if p.AudienceFilter == nil {
			return nil, fmt.Errorf("workflow node payload: %s variant is nil", p.TypeName)
		}
		return json.Marshal(p.AudienceFilter)
	case WorkflowNodeTypeTimerAction:
		if p.TimerAction == nil {
			return nil, fmt.Errorf("workflow node payload: %s variant is nil", p.TypeName)
		}
		return json.Marshal(p.TimerAction)
	case WorkflowNodeTypeExperimentBranchNode:
		if p.ExperimentBranch == nil {
			return nil, fmt.Errorf("workflow node payload: %s variant is nil", p.TypeName)
		}
		return json.Marshal(p.ExperimentBranch)
	case WorkflowNodeTypeVariantNode:
		if p.Variant == nil {
			return nil, fmt.Errorf("workflow node payload: %s variant is nil", p.TypeName)
		}
		return json.Marshal(p.Variant)
	}
	return nil, fmt.Errorf("workflow node payload: unknown typeName %q", p.TypeName)
}

// UpdateWorkflowNodeRequest is the request body for [Client.UpdateWorkflowNode].
// ExpectedRevisionID is required and always sent (as JSON null when nil).
type UpdateWorkflowNodeRequest struct {
	ExpectedRevisionID *string
	Payload            UpdateWorkflowNodePayload
}

// UpdateWorkflowNodeResponse is the updated node returned by
// [Client.UpdateWorkflowNode] (its fields stay flat via the embedded type)
// plus the latest simplified workflow.
type UpdateWorkflowNodeResponse struct {
	WorkflowMutationNodeWithRevision
	Workflow SimplifiedWorkflow `json:"workflow"`
}

// UnmarshalJSON decodes the node and revision via the embedded type and then
// the latest workflow.
func (n *UpdateWorkflowNodeResponse) UnmarshalJSON(data []byte) error {
	if err := n.WorkflowMutationNodeWithRevision.UnmarshalJSON(data); err != nil {
		return err
	}
	var extra struct {
		Workflow SimplifiedWorkflow `json:"workflow"`
	}
	if err := json.Unmarshal(data, &extra); err != nil {
		return err
	}
	n.Workflow = extra.Workflow
	return nil
}

// MarshalJSON encodes the embedded node and revision and adds the workflow.
func (n UpdateWorkflowNodeResponse) MarshalJSON() ([]byte, error) {
	return mergeMarshal(n.WorkflowMutationNodeWithRevision, map[string]any{"workflow": n.Workflow})
}

// UpdateWorkflowNode applies req.Payload to the node identified by
// workflowID/nodeID and returns the updated node.
func (c *Client) UpdateWorkflowNode(workflowID, nodeID string, req UpdateWorkflowNodeRequest) (*UpdateWorkflowNodeResponse, error) {
	body := map[string]any{
		"expectedRevisionId": req.ExpectedRevisionID,
		"payload":            req.Payload,
	}

	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(http.MethodPost, "/workflows/"+workflowID+"/nodes/"+nodeID, bytes.NewReader(b))
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

	var result UpdateWorkflowNodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// AddWorkflowBranchRequest is the request body for [Client.AddWorkflowBranch].
// ExpectedRevisionID is required and always sent (as JSON null when nil).
type AddWorkflowBranchRequest struct {
	ExpectedRevisionID *string
}

// AddWorkflowBranchResponse is returned by [Client.AddWorkflowBranch].
type AddWorkflowBranchResponse struct {
	Node     WorkflowMutationNodeWithRevision `json:"node"`
	Workflow SimplifiedWorkflow               `json:"workflow"`
}

// AddWorkflowBranch adds a branch to the branch node identified by
// workflowID/nodeID and returns the updated node and workflow.
func (c *Client) AddWorkflowBranch(workflowID, nodeID string, req AddWorkflowBranchRequest) (*AddWorkflowBranchResponse, error) {
	body := map[string]any{
		"expectedRevisionId": req.ExpectedRevisionID,
	}

	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(http.MethodPost, "/workflows/"+workflowID+"/nodes/"+nodeID+"/add-branch", bytes.NewReader(b))
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

	var result AddWorkflowBranchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// DeleteWorkflowNodeRequest is the request body for [Client.DeleteWorkflowNode]
// and [Client.DeleteWorkflowNodeRecursive]. ExpectedRevisionID is required and
// always sent (as JSON null when nil). QueuedContactPolicy is one of the
// WorkflowQueuedContactPolicy* constants.
type DeleteWorkflowNodeRequest struct {
	ExpectedRevisionID  *string
	DryRun              bool
	QueuedContactPolicy string
}

// DeleteWorkflowNodeResponse is returned by [Client.DeleteWorkflowNode] and
// [Client.DeleteWorkflowNodeRecursive]. Status is one of the
// WorkflowMutationStatus* values. WorkflowRevisionID and Workflow are set only
// when Status is "deleted".
type DeleteWorkflowNodeResponse struct {
	Status             string              `json:"status"`
	NodeIDs            []string            `json:"nodeIds"`
	WorkflowRevisionID *string             `json:"workflowRevisionId,omitempty"`
	QueuedContactCount float64             `json:"queuedContactCount"`
	Workflow           *SimplifiedWorkflow `json:"workflow,omitempty"`
}

func (c *Client) deleteWorkflowNode(path string, req DeleteWorkflowNodeRequest) (*DeleteWorkflowNodeResponse, error) {
	body := map[string]any{
		"expectedRevisionId": req.ExpectedRevisionID,
	}
	if req.DryRun {
		body["dryRun"] = req.DryRun
	}
	if req.QueuedContactPolicy != "" {
		body["queuedContactPolicy"] = req.QueuedContactPolicy
	}

	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(http.MethodDelete, path, bytes.NewReader(b))
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

	var result DeleteWorkflowNodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// DeleteWorkflowNode deletes the node identified by workflowID/nodeID. With the
// default "fail" policy (or DryRun), a response whose Status is not "deleted"
// reports the queued-contact impact instead of applying the deletion.
func (c *Client) DeleteWorkflowNode(workflowID, nodeID string, req DeleteWorkflowNodeRequest) (*DeleteWorkflowNodeResponse, error) {
	return c.deleteWorkflowNode("/workflows/"+workflowID+"/nodes/"+nodeID, req)
}

// DeleteWorkflowNodeRecursive deletes the node identified by workflowID/nodeID
// along with its downstream nodes. Semantics otherwise match
// [Client.DeleteWorkflowNode].
func (c *Client) DeleteWorkflowNodeRecursive(workflowID, nodeID string, req DeleteWorkflowNodeRequest) (*DeleteWorkflowNodeResponse, error) {
	return c.deleteWorkflowNode("/workflows/"+workflowID+"/nodes/"+nodeID+"/recursive", req)
}

// ErrWorkflowDeleteConfirmationRequired is wrapped into the error returned by
// [Client.DeleteWorkflow] when the workflow is sending or has queued contacts.
// Retry with the same ExpectedRevisionID and ConfirmDelete set to true. The
// underlying *[APIError] remains reachable with [errors.As].
var ErrWorkflowDeleteConfirmationRequired = errors.New("loops: workflow deletion requires confirmation")

// The API reports both a stale revision and a required confirmation as a 409
// carrying only a message, so the two are told apart by this suffix.
const workflowDeleteConfirmationSuffix = "Confirm deletion by sending a second request with confirmDelete: true."

// DeleteWorkflowRequest is the request body for [Client.DeleteWorkflow].
// ExpectedRevisionID is required and always sent (as JSON null when nil).
// ConfirmDelete is sent only when true.
type DeleteWorkflowRequest struct {
	ExpectedRevisionID *string
	ConfirmDelete      bool
}

// DeleteWorkflow deletes the workflow identified by id. Deleted workflows are
// reported as not found by the other workflow endpoints.
//
// A workflow that is sending or has queued contacts is not deleted on the
// first attempt: the returned error wraps
// [ErrWorkflowDeleteConfirmationRequired] and its message describes the impact.
// Retry with the same ExpectedRevisionID and ConfirmDelete set to true to
// delete the workflow, stop sending it, and cancel its queued contacts.
//
// A stale ExpectedRevisionID also fails with a 409, as a plain *[APIError].
func (c *Client) DeleteWorkflow(id string, req DeleteWorkflowRequest) error {
	body := map[string]any{
		"expectedRevisionId": req.ExpectedRevisionID,
	}
	if req.ConfirmDelete {
		body["confirmDelete"] = req.ConfirmDelete
	}

	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(http.MethodDelete, "/workflows/"+id, bytes.NewReader(b))
	if err != nil {
		return err
	}

	resp, err := c.do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		apiErr := errorFromResponse(resp)
		if resp.StatusCode == http.StatusConflict && strings.HasSuffix(apiErr.Message, workflowDeleteConfirmationSuffix) {
			return fmt.Errorf("%w: %w", ErrWorkflowDeleteConfirmationRequired, apiErr)
		}
		return apiErr
	}

	return nil
}

// RerouteNodeConnectionRequest is the request body for
// [Client.RerouteNodeConnection]. ExpectedRevisionID is required and always
// sent (as JSON null when nil). NewTargetNodeID is the valid node that should
// receive the source node's outgoing connection.
type RerouteNodeConnectionRequest struct {
	ExpectedRevisionID *string
	NewTargetNodeID    string
}

// RerouteNodeConnectionResponse is the updated source node returned by
// [Client.RerouteNodeConnection] (its fields stay flat via the embedded type)
// plus the latest simplified workflow.
type RerouteNodeConnectionResponse struct {
	WorkflowMutationNodeWithRevision
	Workflow SimplifiedWorkflow `json:"workflow"`
}

// UnmarshalJSON decodes the node and revision via the embedded type and then
// the latest workflow.
func (n *RerouteNodeConnectionResponse) UnmarshalJSON(data []byte) error {
	if err := n.WorkflowMutationNodeWithRevision.UnmarshalJSON(data); err != nil {
		return err
	}
	var extra struct {
		Workflow SimplifiedWorkflow `json:"workflow"`
	}
	if err := json.Unmarshal(data, &extra); err != nil {
		return err
	}
	n.Workflow = extra.Workflow
	return nil
}

// MarshalJSON encodes the embedded node and revision and adds the workflow.
func (n RerouteNodeConnectionResponse) MarshalJSON() ([]byte, error) {
	return mergeMarshal(n.WorkflowMutationNodeWithRevision, map[string]any{"workflow": n.Workflow})
}

// RerouteNodeConnection moves the single outgoing connection of the source node
// identified by workflowID/nodeID to req.NewTargetNodeID, returning the updated
// source node and the latest workflow. The source node must have exactly one
// outgoing connection; branch and experiment branch nodes cannot be rerouted.
func (c *Client) RerouteNodeConnection(workflowID, nodeID string, req RerouteNodeConnectionRequest) (*RerouteNodeConnectionResponse, error) {
	body := map[string]any{
		"expectedRevisionId": req.ExpectedRevisionID,
		"newTargetNodeId":    req.NewTargetNodeID,
	}

	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(http.MethodPost, "/workflows/"+workflowID+"/nodes/"+nodeID+"/reroute", bytes.NewReader(b))
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

	var result RerouteNodeConnectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
