package loops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// CreateUploadRequest is the request body for [Client.CreateUpload]. The
// API restricts ContentType to a small set of image types (currently
// image/jpeg, image/png, image/gif and image/webp) and enforces a maximum
// ContentLength; see the Loops API docs for the current limits.
type CreateUploadRequest struct {
	ContentType   string `json:"contentType"`
	ContentLength int64  `json:"contentLength"`
}

// CreateUploadResponse is returned by [Client.CreateUpload]. PresignedURL
// is a short-lived S3 URL that the caller must PUT the asset bytes to
// before calling [Client.CompleteUpload] with EmailAssetID.
type CreateUploadResponse struct {
	EmailAssetID string `json:"emailAssetId"`
	PresignedURL string `json:"presignedUrl"`
}

// CompleteUploadResponse is returned by [Client.CompleteUpload] and
// [Client.Upload]. FinalURL is the permanent, publicly addressable URL for
// the asset, suitable for referencing in email content.
type CompleteUploadResponse struct {
	EmailAssetID string `json:"emailAssetId"`
	FinalURL     string `json:"finalUrl"`
}

// UploadRequest is the input to [Client.Upload], the one-call helper that
// performs the full three-step upload flow. Body is read up to
// ContentLength bytes and sent as the asset body.
type UploadRequest struct {
	ContentType   string
	ContentLength int64
	Body          io.Reader
}

// CreateUpload reserves an asset slot on the account and returns a
// presigned S3 URL the caller must PUT the asset bytes to. After a
// successful PUT, call [Client.CompleteUpload] to finalize the asset. Most
// callers should use [Client.Upload] instead, which performs all three
// steps.
func (c *Client) CreateUpload(ctx context.Context, req CreateUploadRequest) (*CreateUploadResponse, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	httpReq, err := c.newRequest(ctx, http.MethodPost, "/uploads", bytes.NewReader(b))
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

	var result CreateUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// Upload performs the full three-step asset upload: it calls
// [Client.CreateUpload] to obtain a presigned URL, PUTs req.Body to that
// URL using the SDK's HTTP client, and finalizes the asset with
// [Client.CompleteUpload]. The PUT to S3 deliberately does not carry the
// Loops Authorization header. On success the returned FinalURL is the
// public URL of the uploaded asset.
func (c *Client) Upload(ctx context.Context, req UploadRequest) (*CompleteUploadResponse, error) {
	created, err := c.CreateUpload(ctx, CreateUploadRequest{
		ContentType:   req.ContentType,
		ContentLength: req.ContentLength,
	})
	if err != nil {
		return nil, err
	}

	putReq, err := http.NewRequestWithContext(ctx, http.MethodPut, created.PresignedURL, req.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to build upload request: %w", err)
	}
	putReq.Header.Set("Content-Type", req.ContentType)
	putReq.ContentLength = req.ContentLength

	putResp, err := c.httpClient.Do(putReq)
	if err != nil {
		return nil, fmt.Errorf("upload to presigned URL failed: %w", err)
	}
	defer putResp.Body.Close()

	if putResp.StatusCode < 200 || putResp.StatusCode >= 300 {
		return nil, fmt.Errorf("upload to presigned URL failed: status %d", putResp.StatusCode)
	}

	return c.CompleteUpload(ctx, created.EmailAssetID)
}

// CompleteUpload finalizes an asset previously created with
// [Client.CreateUpload] after its bytes have been PUT to the presigned
// URL. The id argument is the EmailAssetID returned by CreateUpload.
func (c *Client) CompleteUpload(ctx context.Context, id string) (*CompleteUploadResponse, error) {
	req, err := c.newRequest(ctx, http.MethodPost, "/uploads/"+id+"/complete", nil)
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

	var result CompleteUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
