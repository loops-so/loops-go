package loops

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetDedicatedSendingIPs(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantAPIErr *APIError
		wantIPs    []string
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
			body:       `["1.2.3.4","5.6.7.8"]`,
			wantIPs:    []string{"1.2.3.4", "5.6.7.8"},
		},
		{
			name:       "empty",
			statusCode: http.StatusOK,
			body:       `[]`,
			wantIPs:    []string{},
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			body:       `{"success":false,"message":"oops"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusInternalServerError, Message: "oops"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			ips, err := client.GetDedicatedSendingIPs(context.Background())

			if tt.wantAPIErr != nil {
				var apiErr *APIError
				if !errors.As(err, &apiErr) {
					t.Fatalf("expected *APIError, got %T: %v", err, err)
				}
				if apiErr.StatusCode != tt.wantAPIErr.StatusCode {
					t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, tt.wantAPIErr.StatusCode)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotPath != "/dedicated-sending-ips" {
				t.Errorf("path = %q, want /dedicated-sending-ips", gotPath)
			}
			if len(ips) != len(tt.wantIPs) {
				t.Errorf("len(ips) = %d, want %d", len(ips), len(tt.wantIPs))
			}
			for i := range tt.wantIPs {
				if ips[i] != tt.wantIPs[i] {
					t.Errorf("ips[%d] = %q, want %q", i, ips[i], tt.wantIPs[i])
				}
			}
		})
	}
}
