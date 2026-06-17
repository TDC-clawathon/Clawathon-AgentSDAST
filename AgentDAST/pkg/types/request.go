package types

import "time"

// RequestLog captures a single HTTP request/response exchange performed during a scan.
type RequestLog struct {
	ID              string            `json:"id"`
	Method          string            `json:"method"`
	URL             string            `json:"url"`
	RequestHeaders  map[string]string `json:"request_headers,omitempty"`
	RequestBody     string            `json:"request_body,omitempty"`
	StatusCode      int               `json:"status_code"`
	ResponseHeaders map[string]string `json:"response_headers,omitempty"`
	ResponseBody    string            `json:"response_body,omitempty"`
	DurationMS      int64             `json:"duration_ms"`
	Timestamp       time.Time         `json:"timestamp"`
	Error           string            `json:"error,omitempty"`
}
