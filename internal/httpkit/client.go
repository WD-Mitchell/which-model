// Package httpkit is the shared HTTP client: retries, redirect rejection, body bounding.
package httpkit

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"
	"time"
)

const (
	// DefaultTimeout is the per-request timeout applied when the caller's
	// context has no deadline (specs/global/CONTRACTS.md §7 DefaultTimeoutSec).
	DefaultTimeout = 10 * time.Second
	// DefaultMaxResponseBytes bounds every response body (specs/global/CONTRACTS.md §7 MaxResponseBytes).
	DefaultMaxResponseBytes = int64(262_144) // 256 KiB
)

var errRedirectRejected = errors.New("httpkit: redirect rejected")

type Client struct {
	timeout   time.Duration
	maxBytes  int64
	retries   int
	backoff   time.Duration
	userAgent string
	allowed   []string
	hc        *http.Client
}

type Option func(*Client)

// WithTimeout overrides the per-request timeout (default DefaultTimeout).
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.timeout = d }
}

// WithMaxBytes overrides the response body bound (default DefaultMaxResponseBytes).
func WithMaxBytes(n int64) Option {
	return func(c *Client) { c.maxBytes = n }
}

// WithUserAgent overrides the User-Agent sent on every request
// (default "which-model/dev").
func WithUserAgent(s string) Option {
	return func(c *Client) { c.userAgent = s }
}

// WithRetries sets the retry budget for 5xx statuses and network errors
// (default 1 retry = 2 attempts, 500ms backoff; 4xx is never retried).
func WithRetries(n int) Option {
	return func(c *Client) { c.retries = n }
}

// NewClient builds a configured client with the documented defaults.
func NewClient(opts ...Option) *Client {
	c := &Client{
		timeout:   DefaultTimeout,
		maxBytes:  DefaultMaxResponseBytes,
		retries:   1,
		backoff:   500 * time.Millisecond,
		userAgent: "which-model/dev",
		allowed:   nil,
		hc: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return errRedirectRejected
			},
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// SetAllowList stores the exact-URL allow-list. When non-empty, every
// request URL must be https, carry no userinfo/fragment, and match an entry
// exactly under Go URL serialization; a mismatch returns
// Error{Code:"endpoint_refused"} before any network I/O. When empty
// (never set), no URL check is applied.
func (c *Client) SetAllowList(urls []string) {
	c.allowed = append([]string(nil), urls...)
}

func validateURL(raw string, allowed []string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return &Error{Code: "endpoint_refused", Err: err}
	}
	if allowed == nil {
		return nil
	}
	if u.Scheme != "https" || u.User != nil || u.Fragment != "" || !slices.Contains(allowed, u.String()) {
		return &Error{Code: "endpoint_refused"}
	}
	return nil
}

// Do executes req, enforcing in order: exact-URL allow-list (if set),
// redirect rejection (ANY 3xx -> Error{Code:"redirect_refused"}, zero
// followed hops), Content-Length pre-check + io.LimitReader body bound
// (checked twice -> Error{Code:"response_too_large"}), context deadline
// -> Error{Code:"timeout"}, transport failure -> Error{Code:"network"}.
// Success returns the bounded body ONLY for 2xx statuses. Every status
// >= 400 returns *Error with StatusCode set, Code mapped: 401/403 ->
// "unauthorized", 429 -> "rate_limited", any other >= 400 ->
// "provider_status" (retryable 5xx are retried first per WithRetries;
// the mapping applies to the final attempt's status). The client sets
// req's User-Agent on every call, overriding any caller value.
func (c *Client) Do(ctx context.Context, req *http.Request) ([]byte, error) {
	if deadline, hasDeadline := ctx.Deadline(); !hasDeadline || time.Until(deadline) > c.timeout {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}
	req = req.WithContext(ctx)
	for attempt := 0; ; attempt++ {
		body, retryable, err := c.doOnce(ctx, req)
		if !retryable || attempt >= c.retries {
			return body, err
		}
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, &Error{Code: "timeout"}
			}
			return nil, &Error{Code: "network"}
		case <-time.After(c.backoff):
		}
	}
}

func (c *Client) doOnce(ctx context.Context, req *http.Request) ([]byte, bool, error) {
	replayable := req.Body == nil || req.GetBody != nil
	if err := validateURL(req.URL.String(), c.allowed); err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.hc.Do(req)
	if err != nil {
		switch {
		case errors.Is(err, errRedirectRejected):
			return nil, false, &Error{Code: "redirect_refused", Err: err}
		case errors.Is(err, context.DeadlineExceeded):
			return nil, false, &Error{Code: "timeout", Err: err}
		case errors.Is(err, context.Canceled):
			return nil, false, &Error{Code: "network", Err: err}
		default:
			return nil, replayable, &Error{Code: "network", Err: err}
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return nil, false, &Error{Code: "redirect_refused"}
	}
	if resp.ContentLength > c.maxBytes {
		return nil, false, &Error{Code: "response_too_large", StatusCode: resp.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, c.maxBytes+1))
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return nil, false, &Error{Code: "timeout", Err: err}
		case errors.Is(err, context.Canceled):
			return nil, false, &Error{Code: "network", Err: err}
		default:
			return nil, replayable, &Error{Code: "network", Err: err}
		}
	}
	if int64(len(body)) > c.maxBytes {
		return nil, false, &Error{Code: "response_too_large", StatusCode: resp.StatusCode}
	}
	if resp.StatusCode >= 400 {
		return nil, replayable && resp.StatusCode >= 500, statusError(resp.StatusCode)
	}
	return body, false, nil
}

func statusError(status int) *Error {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return &Error{Code: "unauthorized", StatusCode: status}
	case http.StatusTooManyRequests:
		return &Error{Code: "rate_limited", StatusCode: status}
	default:
		return &Error{Code: "provider_status", StatusCode: status}
	}
}

// GetJSON is GET + Do + json.Unmarshal into out. Any unmarshal failure
// (including an empty body) returns Error{Code:"response_json"}.
func (c *Client) GetJSON(ctx context.Context, url string, headers map[string]string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	body, err := c.Do(ctx, req)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return &Error{Code: "response_json"}
	}
	return nil
}

