// Package loops is the official Go SDK for the Loops API (https://loops.so).
//
// It provides a [Client] for the Loops REST API, covering contacts, contact
// properties, mailing lists, events, transactional and campaign emails,
// email messages, components, and themes.
//
// Construct a client with an API key from your Loops account, then call
// methods on it:
//
//	client := loops.NewClient("YOUR_API_KEY")
//
//	err := client.SendEvent(loops.SendEventRequest{
//	    Email:     "user@example.com",
//	    EventName: "signup",
//	})
//
// Requests to 429 and 5xx responses are retried automatically with
// exponential backoff and jitter. API errors are returned as [*APIError]
// and can be inspected with [errors.As].
package loops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"time"
)

// DefaultBaseURL is the base URL for the Loops API used by [NewClient]
// when [WithBaseURL] is not supplied.
const DefaultBaseURL = "https://app.loops.so/api/v1"

const (
	maxRetries = 2
	baseDelay  = 500 * time.Millisecond
)

var sleep = sleepContext

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func isRetryable(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= 500
}

// APIError is returned by [Client] methods when the Loops API responds with
// a non-2xx status. Use [errors.As] to inspect the status code and message:
//
//	var apiErr *loops.APIError
//	if errors.As(err, &apiErr) {
//	    log.Printf("loops api error %d: %s", apiErr.StatusCode, apiErr.Message)
//	}
type APIError struct {
	StatusCode int
	Message    string
}

// Error returns the API error message.
func (e *APIError) Error() string {
	return e.Message
}

// Client is a Loops API client. Construct one with [NewClient]. A Client is
// safe for concurrent use by multiple goroutines.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	logger     io.Writer
	userAgent  string
}

// Option configures a [Client]. Pass options to [NewClient].
type Option func(*Client)

// WithBaseURL overrides the API base URL used by the [Client]. Most callers
// do not need this; the default is [DefaultBaseURL].
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = u } }

// WithUserAgent sets the User-Agent header sent on outgoing requests. The
// default is "loops-go/" + [Version].
func WithUserAgent(ua string) Option { return func(c *Client) { c.userAgent = ua } }

// WithLogger enables verbose request and response logging to w. The
// Authorization header is redacted. Intended for debugging only.
func WithLogger(w io.Writer) Option { return func(c *Client) { c.logger = w } }

// WithHTTPClient replaces the underlying [*http.Client] used to make
// requests. Use this to configure custom timeouts, transports, or proxies.
// The default client has a 5 second timeout.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.httpClient = h } }

// NewClient returns a new [Client] authenticated with the given API key.
// Apply zero or more [Option] values to override defaults.
func NewClient(apiKey string, opts ...Option) *Client {
	c := &Client{
		baseURL:    DefaultBaseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		userAgent:  "loops-go/" + Version,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func errorFromResponse(resp *http.Response) *APIError {
	var body struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err == nil {
		if body.Error != "" {
			return &APIError{StatusCode: resp.StatusCode, Message: body.Error}
		}
		if body.Message != "" {
			return &APIError{StatusCode: resp.StatusCode, Message: body.Message}
		}
	}
	return &APIError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("unexpected status: %d", resp.StatusCode)}
}

func (c *Client) logResponse(resp *http.Response) {
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(c.logger, "[debug] Response: %s (body read failed: %v)\n", resp.Status, err)
		resp.Body = io.NopCloser(bytes.NewReader(nil))
		return
	}
	resp.Body = io.NopCloser(bytes.NewReader(raw))
	fmt.Fprintf(c.logger, "[debug] Response: %s (%d bytes)\n", resp.Status, len(raw))
	if len(raw) == 0 {
		return
	}
	var pretty bytes.Buffer
	if json.Indent(&pretty, raw, "", "  ") == nil {
		fmt.Fprintf(c.logger, "[debug] Body:\n%s\n", pretty.String())
	} else {
		fmt.Fprintf(c.logger, "[debug] Body: %s\n", raw)
	}
}

// Do executes an arbitrary request against the Loops API. The request is
// built against the configured base URL with the Authorization and
// User-Agent headers attached, body is JSON-encoded when non-nil, and
// the call runs through the shared retry/backoff plumbing. Callers are
// responsible for closing the returned response body.
//
// Do is an escape hatch for endpoints that do not yet have a dedicated
// method, or for callers that want to share this client's retries and
// User-Agent. Non-2xx responses are returned as-is — inspect
// resp.StatusCode and decode the body to suit. Transport errors are
// wrapped, matching the behavior of the higher-level methods.
//
// To send a pre-encoded JSON payload without re-marshaling, pass a
// [json.RawMessage].
func (c *Client) Do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to encode request: %w", err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := c.newRequest(method, path, reader)
	if err != nil {
		return nil, err
	}
	return c.do(req.WithContext(ctx))
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	var (
		resp *http.Response
		err  error
	)
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(1<<(attempt-1)) * baseDelay
			jitter := time.Duration(rand.Int64N(int64(delay / 2)))
			if err := sleep(req.Context(), delay+jitter); err != nil {
				return nil, fmt.Errorf("request failed: %w", err)
			}
			if req.GetBody != nil {
				body, err := req.GetBody()
				if err != nil {
					return nil, fmt.Errorf("failed to reset request body: %w", err)
				}
				req.Body = body
			}
		}
		resp, err = c.httpClient.Do(req)
		if err != nil {
			if req.Context().Err() != nil {
				return nil, fmt.Errorf("request failed: %w", err)
			}
			continue
		}
		if !isRetryable(resp.StatusCode) {
			if c.logger != nil {
				c.logResponse(resp)
			}
			return resp, nil
		}
		if attempt < maxRetries {
			resp.Body.Close()
		}
	}
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	return resp, nil
}

func (c *Client) newRequest(method, path string, body io.Reader) (*http.Request, error) {
	url := fmt.Sprintf("%s%s", c.baseURL, path)

	var bodyBytes []byte
	if body != nil && c.logger != nil {
		var err error
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
		body = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.logger != nil {
		fmt.Fprintf(c.logger, "[debug] %s %s\n", method, url)
		fmt.Fprintf(c.logger, "[debug] Authorization: Bearer [REDACTED]\n")
		if req.Header.Get("Content-Type") != "" {
			fmt.Fprintf(c.logger, "[debug] Content-Type: %s\n", req.Header.Get("Content-Type"))
		}
		if len(bodyBytes) > 0 {
			var pretty bytes.Buffer
			if json.Indent(&pretty, bodyBytes, "", "  ") == nil {
				fmt.Fprintf(c.logger, "[debug] Body:\n%s\n", pretty.String())
			} else {
				fmt.Fprintf(c.logger, "[debug] Body: %s\n", bodyBytes)
			}
		}
	}

	return req, nil
}
