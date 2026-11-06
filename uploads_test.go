package loops

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const fakePresignedQuery = "X-Amz-Algorithm=AWS4-HMAC-SHA256" +
	"&X-Amz-Credential=dummy" +
	"&X-Amz-Date=dummy" +
	"&X-Amz-Expires=900" +
	"&X-Amz-SignedHeaders=host" +
	"&X-Amz-Security-Token=dummy" +
	"&X-Amz-Signature=dummy"

const fakePresignedURL = "https://example.s3.us-east-1.amazonaws.com/team_abc123/asset_abc123.png?" + fakePresignedQuery

var createUploadResponse = fmt.Sprintf(`{
	"success": true,
	"emailAssetId": "asset_abc123",
	"presignedUrl": %q
}`, fakePresignedURL)

func TestCreateUpload(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantAPIErr *APIError
		wantErrMsg string
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
			body:       createUploadResponse,
		},
		{
			name:       "unsupported content type",
			statusCode: http.StatusBadRequest,
			body:       `{"success":false,"message":"Unsupported content type","supportedContentTypes":["image/jpeg","image/png","image/gif","image/webp"]}`,
			wantAPIErr: &APIError{StatusCode: http.StatusBadRequest, Message: "Unsupported content type"},
		},
		{
			name:       "file too large",
			statusCode: http.StatusRequestEntityTooLarge,
			body:       `{"success":false,"message":"File too large","maxBytes":4000000}`,
			wantAPIErr: &APIError{StatusCode: http.StatusRequestEntityTooLarge, Message: "File too large"},
		},
		{
			name:       "invalid json",
			statusCode: http.StatusOK,
			body:       `not json`,
			wantErrMsg: "failed to decode response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath, gotMethod string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotMethod = r.Method
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			result, err := client.CreateUpload(CreateUploadRequest{
				ContentType:   "image/png",
				ContentLength: 1024,
			})

			if tt.wantAPIErr != nil {
				var apiErr *APIError
				if !errors.As(err, &apiErr) {
					t.Fatalf("expected *APIError, got %T: %v", err, err)
				}
				if apiErr.StatusCode != tt.wantAPIErr.StatusCode {
					t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, tt.wantAPIErr.StatusCode)
				}
				if apiErr.Message != tt.wantAPIErr.Message {
					t.Errorf("Message = %q, want %q", apiErr.Message, tt.wantAPIErr.Message)
				}
				return
			}

			if tt.wantErrMsg != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrMsg)
				}
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("error = %q, want it to contain %q", err.Error(), tt.wantErrMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotMethod != http.MethodPost {
				t.Errorf("method = %q, want POST", gotMethod)
			}
			if gotPath != "/uploads" {
				t.Errorf("path = %q, want /uploads", gotPath)
			}
			if result.EmailAssetID != "asset_abc123" {
				t.Errorf("EmailAssetID = %q, want asset_abc123", result.EmailAssetID)
			}
			if result.PresignedURL != fakePresignedURL {
				t.Errorf("PresignedURL = %q", result.PresignedURL)
			}
		})
	}
}

func TestCreateUpload_RequestBody(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(createUploadResponse))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	_, err := client.CreateUpload(CreateUploadRequest{
		ContentType:   "image/png",
		ContentLength: 2048,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := body["emailMessageId"]; ok {
		t.Errorf("emailMessageId should not be in request body, got %v", body["emailMessageId"])
	}
	if body["contentType"] != "image/png" {
		t.Errorf("contentType = %v, want image/png", body["contentType"])
	}
	if body["contentLength"] != float64(2048) {
		t.Errorf("contentLength = %v, want 2048", body["contentLength"])
	}
}

func TestUpload(t *testing.T) {
	var (
		gotCreatePath, gotPutPath, gotCompletePath  string
		gotPutMethod, gotPutContentType, gotPutAuth string
		gotPutBody                                  []byte
		gotPutContentLength                         int64
		createCalls, putCalls, completeCalls        int
	)

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/uploads", func(w http.ResponseWriter, r *http.Request) {
		createCalls++
		gotCreatePath = r.URL.Path
		body := fmt.Sprintf(`{
			"success": true,
			"emailAssetId": "asset_abc123",
			"presignedUrl": "%s/asset_abc123.png?%s"
		}`, server.URL, fakePresignedQuery)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	})
	mux.HandleFunc("/asset_abc123.png", func(w http.ResponseWriter, r *http.Request) {
		putCalls++
		gotPutPath = r.URL.Path
		gotPutMethod = r.Method
		gotPutContentType = r.Header.Get("Content-Type")
		gotPutAuth = r.Header.Get("Authorization")
		gotPutContentLength = r.ContentLength
		gotPutBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/uploads/asset_abc123/complete", func(w http.ResponseWriter, r *http.Request) {
		completeCalls++
		gotCompletePath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(completeUploadResponse))
	})

	client := NewClient("test-key", WithBaseURL(server.URL))
	imageBytes := []byte("\x89PNG\r\n\x1a\nfake-image-bytes")
	result, err := client.Upload(UploadRequest{
		ContentType:   "image/png",
		ContentLength: int64(len(imageBytes)),
		Body:          bytes.NewReader(imageBytes),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if createCalls != 1 || putCalls != 1 || completeCalls != 1 {
		t.Errorf("call counts: create=%d put=%d complete=%d, want 1/1/1", createCalls, putCalls, completeCalls)
	}
	if gotCreatePath != "/uploads" {
		t.Errorf("create path = %q, want /uploads", gotCreatePath)
	}
	if gotPutMethod != http.MethodPut {
		t.Errorf("put method = %q, want PUT", gotPutMethod)
	}
	if gotPutPath != "/asset_abc123.png" {
		t.Errorf("put path = %q, want /asset_abc123.png", gotPutPath)
	}
	if gotPutContentType != "image/png" {
		t.Errorf("put content-type = %q, want image/png", gotPutContentType)
	}
	if gotPutAuth != "" {
		t.Errorf("put auth header = %q, want empty (no bearer leak)", gotPutAuth)
	}
	if gotPutContentLength != int64(len(imageBytes)) {
		t.Errorf("put content-length = %d, want %d", gotPutContentLength, len(imageBytes))
	}
	if !bytes.Equal(gotPutBody, imageBytes) {
		t.Errorf("put body = %v, want %v", gotPutBody, imageBytes)
	}
	if gotCompletePath != "/uploads/asset_abc123/complete" {
		t.Errorf("complete path = %q", gotCompletePath)
	}
	if result.EmailAssetID != "asset_abc123" || result.FinalURL != "https://assets.example.com/asset_abc123.png" {
		t.Errorf("result = %+v", result)
	}
}

func TestUpload_CreateFails(t *testing.T) {
	var putCalls, completeCalls int
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/uploads", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/uploads" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"success":false,"message":"Unsupported content type"}`))
			return
		}
		completeCalls++
	})
	mux.HandleFunc("/asset_abc123.png", func(w http.ResponseWriter, r *http.Request) {
		putCalls++
		w.WriteHeader(http.StatusOK)
	})

	client := NewClient("test-key", WithBaseURL(server.URL))
	_, err := client.Upload(UploadRequest{
		ContentType:   "image/bmp",
		ContentLength: 4,
		Body:          bytes.NewReader([]byte("body")),
	})

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("StatusCode = %d, want 400", apiErr.StatusCode)
	}
	if putCalls != 0 {
		t.Errorf("put called %d times, want 0", putCalls)
	}
	if completeCalls != 0 {
		t.Errorf("complete called %d times, want 0", completeCalls)
	}
}

func TestUpload_PutFails(t *testing.T) {
	var completeCalls int
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/uploads", func(w http.ResponseWriter, r *http.Request) {
		body := fmt.Sprintf(`{
			"success": true,
			"emailAssetId": "asset_abc123",
			"presignedUrl": "%s/asset_abc123.png?%s"
		}`, server.URL, fakePresignedQuery)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	})
	mux.HandleFunc("/asset_abc123.png", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`<Error><Code>AccessDenied</Code></Error>`))
	})
	mux.HandleFunc("/uploads/asset_abc123/complete", func(w http.ResponseWriter, r *http.Request) {
		completeCalls++
		w.WriteHeader(http.StatusOK)
	})

	client := NewClient("test-key", WithBaseURL(server.URL))
	_, err := client.Upload(UploadRequest{
		ContentType:   "image/png",
		ContentLength: 4,
		Body:          bytes.NewReader([]byte("body")),
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error = %q, want it to mention status 403", err.Error())
	}
	if completeCalls != 0 {
		t.Errorf("complete called %d times, want 0", completeCalls)
	}
}

func TestUpload_CompleteFails(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/uploads", func(w http.ResponseWriter, r *http.Request) {
		body := fmt.Sprintf(`{
			"success": true,
			"emailAssetId": "asset_abc123",
			"presignedUrl": "%s/asset_abc123.png?%s"
		}`, server.URL, fakePresignedQuery)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	})
	mux.HandleFunc("/asset_abc123.png", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/uploads/asset_abc123/complete", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooEarly)
		w.Write([]byte(`{"success":false,"message":"Upload not yet complete"}`))
	})

	client := NewClient("test-key", WithBaseURL(server.URL))
	_, err := client.Upload(UploadRequest{
		ContentType:   "image/png",
		ContentLength: 4,
		Body:          bytes.NewReader([]byte("body")),
	})

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusTooEarly {
		t.Errorf("StatusCode = %d, want 425", apiErr.StatusCode)
	}
}

const completeUploadResponse = `{
	"success": true,
	"emailAssetId": "asset_abc123",
	"finalUrl": "https://assets.example.com/asset_abc123.png"
}`

func TestCompleteUpload(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		statusCode int
		body       string
		wantAPIErr *APIError
		wantErrMsg string
	}{
		{
			name:       "success",
			id:         "asset_abc123",
			statusCode: http.StatusOK,
			body:       completeUploadResponse,
		},
		{
			name:       "not found",
			id:         "asset_missing",
			statusCode: http.StatusNotFound,
			body:       `{"success":false,"message":"Asset not found"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusNotFound, Message: "Asset not found"},
		},
		{
			name:       "too early",
			id:         "asset_abc123",
			statusCode: http.StatusTooEarly,
			body:       `{"success":false,"message":"Upload not yet complete"}`,
			wantAPIErr: &APIError{StatusCode: http.StatusTooEarly, Message: "Upload not yet complete"},
		},
		{
			name:       "invalid json",
			id:         "asset_abc123",
			statusCode: http.StatusOK,
			body:       `not json`,
			wantErrMsg: "failed to decode response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath, gotMethod string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotMethod = r.Method
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient("test-key", WithBaseURL(server.URL))
			result, err := client.CompleteUpload(tt.id)

			if tt.wantAPIErr != nil {
				var apiErr *APIError
				if !errors.As(err, &apiErr) {
					t.Fatalf("expected *APIError, got %T: %v", err, err)
				}
				if apiErr.StatusCode != tt.wantAPIErr.StatusCode {
					t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, tt.wantAPIErr.StatusCode)
				}
				if apiErr.Message != tt.wantAPIErr.Message {
					t.Errorf("Message = %q, want %q", apiErr.Message, tt.wantAPIErr.Message)
				}
				return
			}

			if tt.wantErrMsg != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrMsg)
				}
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("error = %q, want it to contain %q", err.Error(), tt.wantErrMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotMethod != http.MethodPost {
				t.Errorf("method = %q, want POST", gotMethod)
			}
			if want := "/uploads/" + tt.id + "/complete"; gotPath != want {
				t.Errorf("path = %q, want %q", gotPath, want)
			}
			if result.EmailAssetID != "asset_abc123" {
				t.Errorf("EmailAssetID = %q, want asset_abc123", result.EmailAssetID)
			}
			if result.FinalURL != "https://assets.example.com/asset_abc123.png" {
				t.Errorf("FinalURL = %q", result.FinalURL)
			}
		})
	}
}
