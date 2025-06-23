// Header indicating AI generation
// Model: Gemini 2.5 Pro
// Knowledge Cutoff: Most likely 2023 (as standard for many models, though specific date isn't provided by the API)
// Sources: Standard Go library documentation (net/http, net/url, encoding/json, etc.), general Go programming practices.
// Note: This code has been refactored to simplify client configuration, improve ergonomics,
// and provide clearer workflows for making HTTP requests.
package netcom

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// RequestOption defines a function type for modifying an http.Request.
// It allows for request-specific configurations like headers or query parameters.
type RequestOption func(*http.Request) error

// ClientConfig holds configuration for the netcom Client.
type ClientConfig struct {
	BaseURL        string        // Optional base URL for all requests.
	Timeout        time.Duration // Optional timeout for requests.
	DefaultHeaders http.Header   // Optional default headers for all requests.
	// Advanced users can provide their own http.Client.
	// If nil, a default one will be created (with Timeout if specified).
	// If HTTPClient is provided, ClientConfig.Timeout is ignored.
	HTTPClient *http.Client
}

// Client represents a configurable HTTP client.
type Client struct {
	baseURL        *url.URL
	httpClient     *http.Client
	defaultHeaders http.Header // Default headers applied to every request.
}

// ErrRequestOptionFailed indicates an error applying a request option.
var ErrRequestOptionFailed = errors.New("failed to apply request option")

// ErrRequestCreationFailed indicates an error during http.Request creation.
var ErrRequestCreationFailed = errors.New("failed to create request")

// ErrURLResolutionFailed indicates an error resolving a path against the base URL.
var ErrURLResolutionFailed = errors.New("failed to resolve URL")

// ErrRequestFailed indicates an error executing the HTTP request.
var ErrRequestFailed = errors.New("request failed")

// ErrJSONMarshalFailed indicates an error marshalling data to JSON.
var ErrJSONMarshalFailed = errors.New("failed to marshal JSON")

// ErrReadResponseFailed indicates an error reading the response body.
var ErrReadResponseFailed = errors.New("failed to read response body")

// ErrBadStatusCode indicates a non-2xx HTTP status code.
var ErrBadStatusCode = errors.New("request failed with non-2xx status code")

// NewClient creates a new HTTP client with the given configuration.
func NewClient(config ClientConfig) (*Client, error) {
	c := &Client{}

	if config.BaseURL != "" {
		u, err := url.Parse(config.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("parsing base URL '%s' failed: %w", config.BaseURL, err)
		}
		if !u.IsAbs() {
			return nil, fmt.Errorf("base URL '%s' must be absolute", config.BaseURL)
		}
		c.baseURL = u
	}

	if config.HTTPClient != nil {
		c.httpClient = config.HTTPClient
		// If user provides a client, ClientConfig.Timeout is ignored.
		// The provided client's configuration (including timeout) is used as-is.
	} else {
		c.httpClient = &http.Client{}
		if config.Timeout > 0 {
			c.httpClient.Timeout = config.Timeout
		}
	}

	if config.DefaultHeaders != nil {
		// Clone to prevent modification of the original map if it's reused.
		c.defaultHeaders = config.DefaultHeaders.Clone()
	} else {
		c.defaultHeaders = make(http.Header) // Ensure it's initialized
	}

	return c, nil
}

// SetBaseURL updates the base URL for the client.
// The newBaseURL string must be a valid absolute URL.
// Passing an empty string will clear the base URL.
func (c *Client) SetBaseURL(newBaseURL string) error {
	if newBaseURL == "" {
		c.baseURL = nil
		return nil
	}
	u, err := url.Parse(newBaseURL)
	if err != nil {
		return fmt.Errorf("parsing new base URL failed: %w", err)
	}
	if !u.IsAbs() {
		return fmt.Errorf("new base URL must be absolute: %s", newBaseURL)
	}
	c.baseURL = u
	return nil
}

// SetDefaultHeader sets a default header, replacing any existing values for the key.
func (c *Client) SetDefaultHeader(key, value string) {
	if c.defaultHeaders == nil { // Should be initialized by NewClient
		c.defaultHeaders = make(http.Header)
	}
	c.defaultHeaders.Set(key, value)
}

// AddDefaultHeader adds a default header value. If the header key already exists,
// it appends the new value to the existing ones.
func (c *Client) AddDefaultHeader(key, value string) {
	if c.defaultHeaders == nil { // Should be initialized by NewClient
		c.defaultHeaders = make(http.Header)
	}
	c.defaultHeaders.Add(key, value)
}

// --- Request Options ---

// WithContext adds a context to the request.
func WithContext(ctx context.Context) RequestOption {
	return func(req *http.Request) error {
		*req = *req.WithContext(ctx)
		return nil
	}
}

// WithHeader adds a header to the request. It appends to any existing values
// associated with the key. To overwrite headers, use `req.Header.Set` directly
// or ensure client/request options are ordered appropriately.
func WithHeader(key, value string) RequestOption {
	return func(req *http.Request) error {
		req.Header.Add(key, value)
		return nil
	}
}

// WithSetHeader sets a header on the request, replacing any existing values for the key.
func WithSetHeader(key, value string) RequestOption {
	return func(req *http.Request) error {
		req.Header.Set(key, value)
		return nil
	}
}

// WithQueryParams sets query parameters from a map.
// Existing query parameters with the same keys will be replaced by the values from the map.
func WithQueryParams(params map[string]string) RequestOption {
	return func(req *http.Request) error {
		if len(params) == 0 {
			return nil
		}
		q := req.URL.Query() // Get existing query params to preserve them
		for k, v := range params {
			q.Set(k, v) // Set replaces existing values for the key k
		}
		req.URL.RawQuery = q.Encode()
		return nil
	}
}

// --- Core Logic ---

// resolveURL resolves a relative path against the client's base URL.
// If the client has no base URL, it attempts to parse the path as an absolute URL.
func (c *Client) resolveURL(path string) (*url.URL, error) {
	if c.baseURL == nil {
		// If no base URL, the path must be absolute or parsing will fail correctly.
		u, err := url.Parse(path)
		if err != nil {
			return nil, fmt.Errorf(
				"%w: parsing path '%s' failed: %v",
				ErrURLResolutionFailed,
				path,
				err,
			)
		}
		if !u.IsAbs() {
			return nil, fmt.Errorf(
				"%w: path '%s' is not absolute and no base URL is set",
				ErrURLResolutionFailed,
				path,
			)
		}
		return u, nil
	}
	// Resolve the path relative to the base URL.
	relativeURL, err := url.Parse(path)
	if err != nil {
		// This catches cases where the path itself is malformed.
		return nil, fmt.Errorf(
			"%w: parsing relative path '%s' failed: %v",
			ErrURLResolutionFailed,
			path,
			err,
		)
	}
	return c.baseURL.ResolveReference(relativeURL), nil
}

// newRequest creates a new http.Request with client defaults and request options applied.
func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader, options ...RequestOption) (*http.Request, error) {
	resolvedURL, err := c.resolveURL(path)
	if err != nil {
		// Error already wrapped in resolveURL
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, resolvedURL.String(), body)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRequestCreationFailed, err)
	}

	// 1. Apply client-level default headers.
	// These are added first. Request-specific options can then override (using Set)
	// or add further values (using Add).
	if c.defaultHeaders != nil {
		for key, values := range c.defaultHeaders {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}

	// 2. Apply request-specific options.
	for _, option := range options {
		if err := option(req); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrRequestOptionFailed, err)
		}
	}

	return req, nil
}

// Do sends an HTTP request using the configured underlying client.
// It wraps errors related to the HTTP execution itself.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Add context about the request method and URL if possible
		errCtx := fmt.Sprintf("method=%s url=%s", req.Method, req.URL.String())
		// Check for context cancellation or deadline exceeded
		if ctxErr := req.Context().Err(); ctxErr != nil {
			return nil, fmt.Errorf(
				"%w: context error: %v (%s)",
				ErrRequestFailed,
				ctxErr,
				errCtx,
			)
		}
		// Check for URL errors (e.g., DNS resolution)
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return nil, fmt.Errorf(
				"%w: network error: %v (%s)",
				ErrRequestFailed,
				urlErr,
				errCtx,
			)
		}
		// Generic request failure
		return nil, fmt.Errorf("%w: %v (%s)", ErrRequestFailed, err, errCtx)
	}
	return resp, nil
}

// Request sends an HTTP request with the given method, path, body, and options.
// This is the fundamental method used by helpers like Get, Post, etc.
func (c *Client) Request(ctx context.Context, method, path string, body io.Reader, options ...RequestOption) (*http.Response, error) {
	req, err := c.newRequest(ctx, method, path, body, options...)
	if err != nil {
		// Error already wrapped appropriately by newRequest
		return nil, err
	}
	return c.Do(req)
}

// --- HTTP Method Helpers ---

// Get sends a GET request to the specified path.
func (c *Client) Get(ctx context.Context, path string, options ...RequestOption) (*http.Response, error) {
	return c.Request(ctx, http.MethodGet, path, nil, options...)
}

// Post sends a POST request to the specified path with the given body.
func (c *Client) Post(ctx context.Context, path string, body io.Reader, options ...RequestOption) (*http.Response, error) {
	return c.Request(ctx, http.MethodPost, path, body, options...)
}

// PostJSON sends a POST request with the body marshalled from the data any
// as JSON. It automatically sets the "Content-Type" header to "application/json".
func (c *Client) PostJSON(ctx context.Context, path string, data any, options ...RequestOption) (*http.Response, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrJSONMarshalFailed, err)
	}

	// Prepend the Content-Type header option. User-provided options later
	// in the slice can override it if they specifically use WithSetHeader for Content-Type.
	finalOptions := []RequestOption{WithSetHeader("Content-Type", "application/json")}
	finalOptions = append(finalOptions, options...) // User options come after

	return c.Post(ctx, path, bytes.NewReader(jsonData), finalOptions...)
}

// Put sends a PUT request to the specified path with the given body.
func (c *Client) Put(ctx context.Context, path string, body io.Reader, options ...RequestOption) (*http.Response, error) {
	return c.Request(ctx, http.MethodPut, path, body, options...)
}

// Delete sends a DELETE request to the specified path.
func (c *Client) Delete(ctx context.Context, path string, options ...RequestOption) (*http.Response, error) {
	return c.Request(ctx, http.MethodDelete, path, nil, options...)
}

// Patch sends a PATCH request to the specified path with the given body.
func (c *Client) Patch(ctx context.Context, path string, body io.Reader, options ...RequestOption) (*http.Response, error) {
	return c.Request(ctx, http.MethodPatch, path, body, options...)
}

// --- Response Handling Helpers ---

// DecodeResponse checks for non-2xx status codes, reads and closes the response body,
// and then decodes the JSON body into the provided value `v`.
// If `v` is nil, the body is read and discarded (useful for checking success without needing data).
// Returns ErrBadStatusCode if the status code is outside the 200-299 range.
func DecodeResponse(resp *http.Response, v any) error {
	defer resp.Body.Close()

	// Check for non-successful status codes first.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { // Check 2xx range
		bodyBytes, err := io.ReadAll(resp.Body)
		// Even if reading fails, report the status code error.
		errMsg := fmt.Sprintf("status %d", resp.StatusCode)
		if err == nil && len(bodyBytes) > 0 {
			// Limit the body size in the error message
			const maxBodyErr = 1024
			if len(bodyBytes) > maxBodyErr {
				errMsg = fmt.Sprintf("%s: %s...", errMsg, string(bodyBytes[:maxBodyErr]))
			} else {
				errMsg = fmt.Sprintf("%s: %s", errMsg, string(bodyBytes))
			}
		} else if err != nil {
			errMsg = fmt.Sprintf("%s (failed to read response body: %v)", errMsg, err)
		}
		// Wrap the specific status code error.
		return fmt.Errorf("%w: %s", ErrBadStatusCode, errMsg)
	}

	// If v is nil, we don't need to decode, just consume the body.
	if v == nil {
		_, err := io.Copy(io.Discard, resp.Body) // Efficiently discard body
		if err != nil {
			return fmt.Errorf(
				"%w: discarding body failed: %v",
				ErrReadResponseFailed,
				err,
			)
		}
		return nil
	}

	// Decode the JSON body.
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		// Check if it's an EOF error on an empty body, which might be acceptable
		// depending on the API contract, but generally indicates an issue if
		// decoding was expected.
		if errors.Is(err, io.EOF) {
			// Treat empty body as a decoding error if v was non-nil.
			return fmt.Errorf(
				"json decode failed: unexpected end of input (empty body?)",
			)
		}
		return fmt.Errorf("json decode failed: %w", err)
	}

	return nil
}

// ReadResponseBody reads the entire response body, closes it, and returns it as a string.
// It also checks for non-2xx status codes before reading.
// Returns ErrBadStatusCode if the status code is outside the 200-299 range.
// If a non-2xx status occurs, the read body content is returned along with the error.
func ReadResponseBody(resp *http.Response) (string, error) {
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		// Still check status code if reading failed, it might be more informative.
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf(
				"%w: status %d (also failed to read body: %v)",
				ErrBadStatusCode,
				resp.StatusCode,
				err,
			)
		}
		return "", fmt.Errorf("%w: %v", ErrReadResponseFailed, err)
	}

	// Check status code after successfully reading the body.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errMsg := fmt.Sprintf("status %d: %s", resp.StatusCode, string(bodyBytes))
		// Return body content along with the status error
		return string(bodyBytes), fmt.Errorf("%w: %s", ErrBadStatusCode, errMsg)
	}

	return string(bodyBytes), nil
}
