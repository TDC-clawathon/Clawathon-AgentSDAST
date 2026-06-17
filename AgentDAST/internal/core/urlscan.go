package core

import (
	"fmt"
	"net/url"
	"strings"

	"agentdast/internal/parser"
)

// BuildEndpointFromURL constructs an ephemeral Endpoint and resolved base URL
// from a single target URL (absolute, e.g. http://host/path?x=y) or a path
// (e.g. /path?x=y) resolved against baseURL.
//
// Query parameters become testable query ParamInfo entries (their original
// values are preserved as examples so non-fuzzed params stay realistic). The
// named bodyParams become body ParamInfo entries for body fuzzing.
func BuildEndpointFromURL(rawURL, method, baseURL, body string, bodyParams []string) (parser.Endpoint, string, error) {
	if method == "" {
		method = "GET"
	}
	method = strings.ToUpper(method)

	var u *url.URL
	var err error
	var resolvedBase string

	if isAbsoluteURL(rawURL) {
		u, err = url.Parse(rawURL)
		if err != nil {
			return parser.Endpoint{}, "", fmt.Errorf("invalid target URL: %w", err)
		}
		resolvedBase = u.Scheme + "://" + u.Host
	} else {
		if baseURL == "" {
			return parser.Endpoint{}, "", fmt.Errorf("target URL %q is a path but no base URL was provided", rawURL)
		}
		base, perr := url.Parse(strings.TrimRight(baseURL, "/"))
		if perr != nil {
			return parser.Endpoint{}, "", fmt.Errorf("invalid base URL: %w", perr)
		}
		u, err = url.Parse(rawURL)
		if err != nil {
			return parser.Endpoint{}, "", fmt.Errorf("invalid target path: %w", err)
		}
		resolvedBase = base.Scheme + "://" + base.Host
		// Preserve any base path prefix.
		u.Path = singleJoin(base.Path, u.Path)
	}

	ep := parser.Endpoint{
		Path:   u.Path,
		Method: method,
	}
	for key, vals := range u.Query() {
		example := ""
		if len(vals) > 0 {
			example = vals[0]
		}
		ep.Parameters = append(ep.Parameters, parser.ParamInfo{
			Name:    key,
			In:      "query",
			Type:    "string",
			Example: example,
		})
	}
	for _, f := range bodyParams {
		ep.Parameters = append(ep.Parameters, parser.ParamInfo{Name: f, In: "body", Type: "string"})
	}
	if body != "" || len(bodyParams) > 0 {
		ep.RequestBody = &parser.BodyInfo{ContentType: "application/json", Fields: bodyParams, Example: body}
	}

	return ep, resolvedBase, nil
}

func isAbsoluteURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func singleJoin(a, b string) string {
	a = strings.TrimRight(a, "/")
	if b == "" {
		return a
	}
	if !strings.HasPrefix(b, "/") {
		b = "/" + b
	}
	return a + b
}
