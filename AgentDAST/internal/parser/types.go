package parser

import "strings"

// ParsedSpec is the normalized, version-agnostic view of an OpenAPI document.
type ParsedSpec struct {
	Title     string
	Version   string
	BaseURL   string
	Endpoints []Endpoint
}

// Endpoint is a single operation (method + path) extracted from the spec.
type Endpoint struct {
	Path        string // raw path template, e.g. /users/{id}
	Method      string // GET, POST, PUT, DELETE, PATCH, ...
	OperationID string
	Summary     string
	Parameters  []ParamInfo
	RequestBody *BodyInfo
	Tags        []string
	Secured     bool // true if the operation declares a security requirement
}

// ParamInfo describes one testable input surface.
type ParamInfo struct {
	Name     string
	In       string // query | path | header | cookie | body
	Required bool
	Type     string // schema type: string, integer, number, boolean, array, object
	Example  string // example/default value from the spec, if any
}

// BodyInfo describes a request body input surface.
type BodyInfo struct {
	ContentType string // application/json, application/xml, ...
	Required    bool
	Fields      []string // top-level JSON property names, for body-param fuzzing
	Example     string   // JSON-encoded example body if present
}

// knownLocations are the injection locations an insert point may target.
var knownLocations = map[string]bool{
	"query": true, "header": true, "path": true, "cookie": true, "body": true,
}

// ParseInsertPoint splits an insert-point spec into (location, name). A spec may
// be "loc:name" (e.g. "header:Authorization") or a bare "name" (location empty,
// meaning any/derived). Names may themselves contain ':' only via an explicit
// known-location prefix.
func ParseInsertPoint(s string) (location, name string) {
	if i := strings.IndexByte(s, ':'); i > 0 {
		if loc := strings.ToLower(s[:i]); knownLocations[loc] {
			return loc, s[i+1:]
		}
	}
	return "", s
}

// ResolveInsertPoints turns insert-point specs into concrete ParamInfo targets.
// Empty points means "every declared parameter". A point that matches a declared
// parameter reuses its type/example; a point with no match is synthesized so the
// scanner can inject into positions the spec does not declare (e.g. a custom
// header). Multiple points are supported.
func (e *Endpoint) ResolveInsertPoints(points []string) []ParamInfo {
	if len(points) == 0 {
		return e.Parameters
	}
	var out []ParamInfo
	for _, pt := range points {
		loc, name := ParseInsertPoint(pt)
		matched := false
		for _, p := range e.Parameters {
			if p.Name == name && (loc == "" || p.In == loc) {
				out = append(out, p)
				matched = true
			}
		}
		if !matched {
			if loc == "" {
				loc = "query" // default location for an undeclared bare name
			}
			out = append(out, ParamInfo{Name: name, In: loc, Type: "string"})
		}
	}
	return out
}
