package loops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestDo_RetryResetsBody(t *testing.T) {
	origSleep := sleep
	sleep = func(context.Context, time.Duration) error { return nil }
	defer func() { sleep = origSleep }()

	var bodies []string
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		n := attempts.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	req, _ := client.newRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{"hello":"world"}`)))
	resp, err := client.do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if attempts.Load() != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts.Load())
	}
	for i, body := range bodies {
		if body != `{"hello":"world"}` {
			t.Errorf("attempt %d body = %q, want non-empty JSON", i+1, body)
		}
	}
}

func TestNewRequest(t *testing.T) {
	client := NewClient("test-key", WithBaseURL("https://example.com/api/v1"))

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"GET", http.MethodGet, "/api-key"},
		{"POST", http.MethodPost, "/some-resource"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := client.newRequest(tt.method, tt.path, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if req.Method != tt.method {
				t.Errorf("method = %q, want %q", req.Method, tt.method)
			}

			wantURL := "https://example.com/api/v1" + tt.path
			if req.URL.String() != wantURL {
				t.Errorf("url = %q, want %q", req.URL.String(), wantURL)
			}

			wantAuth := "Bearer test-key"
			if got := req.Header.Get("Authorization"); got != wantAuth {
				t.Errorf("Authorization = %q, want %q", got, wantAuth)
			}

			if got := req.Header.Get("User-Agent"); got != fmt.Sprintf("loops-go/%s", Version) {
				t.Errorf("User-Agent = %q, want %q", got, fmt.Sprintf("loops-go/%s", Version))
			}
		})
	}
}

func TestWithUserAgent(t *testing.T) {
	client := NewClient("test-key", WithBaseURL("https://example.com/api/v1"), WithUserAgent("loops-cli/1.2.3"))
	req, err := client.newRequest(http.MethodGet, "/api-key", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := req.Header.Get("User-Agent"); got != "loops-cli/1.2.3" {
		t.Errorf("User-Agent = %q, want %q", got, "loops-cli/1.2.3")
	}
}

func TestNewRequest_InvalidURL(t *testing.T) {
	client := NewClient("test-key", WithBaseURL("://bad-url"))
	_, err := client.newRequest(http.MethodGet, "/path", nil)
	if err == nil {
		t.Error("expected error for invalid URL, got nil")
	}
}

func TestErrorFromResponse(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		body        string
		wantMessage string
	}{
		{
			name:        "reads error field",
			statusCode:  http.StatusBadRequest,
			body:        `{"error":"something went wrong"}`,
			wantMessage: "something went wrong",
		},
		{
			name:        "falls back to message field",
			statusCode:  http.StatusBadRequest,
			body:        `{"message":"something went wrong"}`,
			wantMessage: "something went wrong",
		},
		{
			name:        "prefers error over message",
			statusCode:  http.StatusBadRequest,
			body:        `{"error":"error field","message":"message field"}`,
			wantMessage: "error field",
		},
		{
			name:        "falls back to generic when body is empty",
			statusCode:  http.StatusBadRequest,
			body:        ``,
			wantMessage: "unexpected status: 400",
		},
		{
			name:        "falls back to generic when fields are absent",
			statusCode:  http.StatusBadRequest,
			body:        `{"success":false}`,
			wantMessage: "unexpected status: 400",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			resp, err := http.Get(server.URL)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer resp.Body.Close()

			apiErr := errorFromResponse(resp)
			if apiErr.Message != tt.wantMessage {
				t.Errorf("Message = %q, want %q", apiErr.Message, tt.wantMessage)
			}
		})
	}
}

func TestDo_Retries(t *testing.T) {
	origSleep := sleep
	sleep = func(context.Context, time.Duration) error { return nil }
	defer func() { sleep = origSleep }()

	tests := []struct {
		name         string
		responses    []int
		wantStatus   int
		wantAttempts int32
	}{
		{
			name:         "success on first attempt",
			responses:    []int{200},
			wantStatus:   200,
			wantAttempts: 1,
		},
		{
			name:         "retries on 429 then succeeds",
			responses:    []int{429, 200},
			wantStatus:   200,
			wantAttempts: 2,
		},
		{
			name:         "retries on 500 then succeeds",
			responses:    []int{500, 200},
			wantStatus:   200,
			wantAttempts: 2,
		},
		{
			name:         "exhausts retries on persistent 429",
			responses:    []int{429, 429, 429},
			wantStatus:   429,
			wantAttempts: 3,
		},
		{
			name:         "no retry on 401",
			responses:    []int{401},
			wantStatus:   401,
			wantAttempts: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var attempts atomic.Int32
			idx := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts.Add(1)
				w.WriteHeader(tt.responses[idx])
				idx++
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			req, _ := client.newRequest(http.MethodGet, "/", nil)
			resp, err := client.do(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
			if attempts.Load() != tt.wantAttempts {
				t.Errorf("attempts = %d, want %d", attempts.Load(), tt.wantAttempts)
			}
		})
	}
}

func TestDo_Exported(t *testing.T) {
	var gotMethod, gotPath, gotAuth, gotUA, gotCT string
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotUA = r.Header.Get("User-Agent")
		gotCT = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"id":"abc"}`)
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	payload := map[string]any{"email": "user@example.com"}
	resp, err := client.Do(context.Background(), http.MethodPost, "/contacts/create", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want %q", gotMethod, http.MethodPost)
	}
	if gotPath != "/contacts/create" {
		t.Errorf("path = %q, want %q", gotPath, "/contacts/create")
	}
	if gotAuth != "Bearer test-key" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer test-key")
	}
	if gotUA != "loops-go/"+Version {
		t.Errorf("User-Agent = %q, want %q", gotUA, "loops-go/"+Version)
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q, want %q", gotCT, "application/json")
	}
	var decoded map[string]any
	if err := json.Unmarshal(gotBody, &decoded); err != nil {
		t.Fatalf("body not valid JSON: %v (body=%q)", err, gotBody)
	}
	if decoded["email"] != "user@example.com" {
		t.Errorf("body email = %v, want %q", decoded["email"], "user@example.com")
	}
}

func TestDo_NilBody(t *testing.T) {
	var gotCT string
	var gotLen int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		gotLen = r.ContentLength
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	resp, err := client.Do(context.Background(), http.MethodGet, "/api-key", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if gotCT != "" {
		t.Errorf("Content-Type = %q, want empty", gotCT)
	}
	if gotLen > 0 {
		t.Errorf("ContentLength = %d, want 0", gotLen)
	}
}

func TestDo_NonOKReturnsRawResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"bad request"}`)
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	resp, err := client.Do(context.Background(), http.MethodGet, "/whatever", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"error":"bad request"}` {
		t.Errorf("body = %q, want raw error body passthrough", body)
	}
}

func TestDo_ContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	resp, err := client.Do(ctx, http.MethodGet, "/", nil)
	if err == nil {
		resp.Body.Close()
		t.Fatal("expected error from cancelled context, got nil")
	}
}

type cancellationTransport struct {
	started chan struct{}
	status  int
}

func (t cancellationTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	select {
	case <-t.started:
	default:
		close(t.started)
	}
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	return &http.Response{StatusCode: t.status, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(nil)), Request: req}, nil
}

func TestDo_CancelsRetryBackoff(t *testing.T) {
	for _, status := range []int{429, 503} {
		for _, deadline := range []bool{false, true} {
			t.Run(fmt.Sprintf("status%d/deadline%v", status, deadline), func(t *testing.T) {
				var ctx context.Context
				var cancel context.CancelFunc
				if deadline {
					ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
				} else {
					ctx, cancel = context.WithCancel(context.Background())
				}
				defer cancel()
				started := make(chan struct{})
				client := NewClient("test-key", WithHTTPClient(&http.Client{Transport: cancellationTransport{started, status}}))
				done := make(chan error, 1)
				go func() {
					resp, err := client.Do(ctx, http.MethodGet, "/test", nil)
					if resp != nil {
						resp.Body.Close()
					}
					done <- err
				}()
				<-started
				if !deadline {
					time.Sleep(20 * time.Millisecond)
					cancel()
				}
				select {
				case err := <-done:
					if !errors.Is(err, ctx.Err()) {
						t.Fatalf("got %v, want context error %v", err, ctx.Err())
					}
				case <-time.After(250 * time.Millisecond):
					err := <-done // Join the request goroutine before restoring test state.
					t.Fatalf("cancellation waited for retry backoff: %v", err)
				}
			})
		}
	}
}
