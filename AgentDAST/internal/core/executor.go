package core

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"agentdast/internal/parser"
	"agentdast/pkg/types"
)

const maxBodyBytes = 1 << 20 // cap response capture at 1 MiB

// RequestExecutor builds and sends HTTP requests, returning a RequestLog for each.
type RequestExecutor struct {
	client *http.Client

	mu        sync.Mutex
	collect   bool
	count     int
	collected []types.RequestLog
}

// EnableLogCollection makes the executor retain a copy of every request log,
// retrievable via CollectedLogs. Used for full output mode.
func (e *RequestExecutor) EnableLogCollection() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.collect = true
}

// CollectedLogs returns all request logs recorded since collection was enabled.
func (e *RequestExecutor) CollectedLogs() []types.RequestLog {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]types.RequestLog, len(e.collected))
	copy(out, e.collected)
	return out
}

// RequestCount returns the total number of requests sent, regardless of whether
// log collection is enabled.
func (e *RequestExecutor) RequestCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.count
}

func (e *RequestExecutor) record(log types.RequestLog) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.count++
	if e.collect {
		e.collected = append(e.collected, log)
	}
}

// NewRequestExecutor constructs an executor honoring the scan config's transport
// options. The transport is tuned for many requests to the same host: keep-alive
// connection reuse and a generous per-host idle pool make scans markedly faster.
func NewRequestExecutor(cfg types.ScanConfig) *RequestExecutor {
	cfg = cfg.WithDefaults()
	transport := &http.Transport{
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: cfg.InsecureSkipVerify},
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 64,
		MaxConnsPerHost:     0, // unlimited; concurrency is bounded by the scanner
		IdleConnTimeout:     90 * time.Second,
		ForceAttemptHTTP2:   true,
		DisableKeepAlives:   false,
	}
	client := &http.Client{
		Timeout:   time.Duration(cfg.Timeout) * time.Second,
		Transport: transport,
	}
	if !cfg.FollowRedirects {
		client.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	return &RequestExecutor{client: client}
}

// Inject builds a request for the endpoint with payload substituted into param, then sends it.
func (e *RequestExecutor) Inject(
	ctx context.Context,
	ep parser.Endpoint,
	baseURL string,
	param parser.ParamInfo,
	payload string,
	cfg types.ScanConfig,
) types.RequestLog {
	req, err := e.buildRequest(ctx, ep, baseURL, &param, payload, cfg)
	if err != nil {
		return types.RequestLog{ID: uuid.NewString(), Method: ep.Method, Error: err.Error(), Timestamp: time.Now()}
	}
	return e.do(req)
}

// Baseline sends an unmodified request against the endpoint (no payload injection).
func (e *RequestExecutor) Baseline(
	ctx context.Context,
	ep parser.Endpoint,
	baseURL string,
	cfg types.ScanConfig,
) types.RequestLog {
	req, err := e.buildRequest(ctx, ep, baseURL, nil, "", cfg)
	if err != nil {
		return types.RequestLog{ID: uuid.NewString(), Method: ep.Method, Error: err.Error(), Timestamp: time.Now()}
	}
	return e.do(req)
}

// Send executes an already-built request and returns its log.
func (e *RequestExecutor) Send(req *http.Request) types.RequestLog {
	return e.do(req)
}

// SendRawBody sends a request with a caller-supplied body and content type.
// Path parameters are filled with benign placeholders. Used by plugins that
// need full control over the request body (e.g. XXE, SSRF).
func (e *RequestExecutor) SendRawBody(
	ctx context.Context,
	ep parser.Endpoint,
	baseURL, contentType, body string,
	cfg types.ScanConfig,
) types.RequestLog {
	path := ep.Path
	query := url.Values{}
	for _, p := range ep.Parameters {
		switch p.In {
		case "path":
			path = strings.ReplaceAll(path, "{"+p.Name+"}", url.PathEscape(placeholder(p)))
		case "query":
			query.Set(p.Name, placeholder(p))
		}
	}
	for k, v := range cfg.CustomParams {
		query.Set(k, v)
	}
	fullURL := strings.TrimRight(baseURL, "/") + path
	if enc := query.Encode(); enc != "" {
		fullURL += "?" + enc
	}

	req, err := http.NewRequestWithContext(ctx, ep.Method, fullURL, strings.NewReader(body))
	if err != nil {
		return types.RequestLog{ID: uuid.NewString(), Method: ep.Method, Error: err.Error(), Timestamp: time.Now()}
	}
	req.Header.Set("Content-Type", contentType)
	for k, v := range cfg.CustomHeaders {
		req.Header.Set(k, v)
	}
	return e.do(req)
}

// buildRequest assembles an *http.Request from an endpoint, optionally injecting
// payload into the supplied param. A nil param produces a baseline request.
func (e *RequestExecutor) buildRequest(
	ctx context.Context,
	ep parser.Endpoint,
	baseURL string,
	param *parser.ParamInfo,
	payload string,
	cfg types.ScanConfig,
) (*http.Request, error) {
	path := ep.Path
	query := url.Values{}
	headers := map[string]string{}
	cookies := map[string]string{}
	bodyFields := map[string]interface{}{}

	// apply places a value at a given location (used for both placeholders and
	// the injected payload, so an insert point not declared in the spec is still
	// honored — e.g. a custom header or cookie).
	apply := func(in, name, val string) {
		switch in {
		case "path":
			path = strings.ReplaceAll(path, "{"+name+"}", url.PathEscape(val))
		case "query":
			query.Set(name, val)
		case "header":
			headers[name] = val
		case "cookie":
			cookies[name] = val
		case "body":
			bodyFields[name] = val
		}
	}

	// Seed every declared parameter with a benign placeholder so the request is
	// well-formed, substituting the payload at the targeted insert point.
	injected := false
	for _, p := range ep.Parameters {
		val := placeholder(p)
		if param != nil && p.Name == param.Name && p.In == param.In {
			val = payload
			injected = true
		}
		apply(p.In, p.Name, val)
	}
	// Targeted insert point not declared on the endpoint: inject it explicitly.
	if param != nil && !injected {
		apply(param.In, param.Name, payload)
	}

	// Merge user-supplied custom params into the query string.
	for k, v := range cfg.CustomParams {
		query.Set(k, v)
	}

	fullURL := strings.TrimRight(baseURL, "/") + path
	if encoded := query.Encode(); encoded != "" {
		fullURL += "?" + encoded
	}

	var body io.Reader
	contentType := ""
	if len(bodyFields) > 0 {
		b, _ := json.Marshal(bodyFields)
		body = bytes.NewReader(b)
		contentType = "application/json"
	} else if ep.RequestBody != nil && ep.Method != http.MethodGet {
		// No structured body fields but the spec expects a body: send the example.
		if ep.RequestBody.Example != "" {
			body = strings.NewReader(ep.RequestBody.Example)
			contentType = ep.RequestBody.ContentType
		}
	}

	req, err := http.NewRequestWithContext(ctx, ep.Method, fullURL, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if len(cookies) > 0 {
		req.Header.Set("Cookie", encodeCookies(cookies))
	}
	// Custom headers take precedence over spec/placeholder headers.
	for k, v := range cfg.CustomHeaders {
		req.Header.Set(k, v)
	}
	return req, nil
}

// encodeCookies renders a cookie map as a Cookie header value.
func encodeCookies(cookies map[string]string) string {
	parts := make([]string, 0, len(cookies))
	for k, v := range cookies {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, "; ")
}

// do executes a request and captures the full exchange into a RequestLog.
func (e *RequestExecutor) do(req *http.Request) types.RequestLog {
	start := time.Now()
	log := types.RequestLog{
		ID:             uuid.NewString(),
		Method:         req.Method,
		URL:            req.URL.String(),
		RequestHeaders: flattenHeader(req.Header),
		Timestamp:      start,
	}
	if req.Body != nil {
		if b, err := req.GetBody(); err == nil {
			data, _ := io.ReadAll(b)
			log.RequestBody = string(data)
		}
	}

	resp, err := e.client.Do(req)
	log.DurationMS = time.Since(start).Milliseconds()
	if err != nil {
		log.Error = err.Error()
		slog.Debug("request error", "method", req.Method, "url", req.URL.String(), "error", err.Error())
		e.record(log)
		return log
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	log.StatusCode = resp.StatusCode
	log.ResponseHeaders = flattenHeader(resp.Header)
	log.ResponseBody = string(data)
	slog.Debug("request", "method", req.Method, "url", req.URL.String(),
		"status", log.StatusCode, "ms", log.DurationMS, "bytes", len(data))
	e.record(log)
	return log
}

// placeholder returns a benign default value for a parameter based on its type/example.
func placeholder(p parser.ParamInfo) string {
	if p.Example != "" {
		return p.Example
	}
	switch p.Type {
	case "integer", "number":
		return "1"
	case "boolean":
		return "true"
	default:
		return "test"
	}
}

func flattenHeader(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k, v := range h {
		out[k] = strings.Join(v, ", ")
	}
	return out
}
