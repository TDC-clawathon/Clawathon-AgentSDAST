package parser

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// extractSpec walks an openapi3.T and produces a normalized ParsedSpec.
func extractSpec(doc *openapi3.T) *ParsedSpec {
	spec := &ParsedSpec{}
	if doc.Info != nil {
		spec.Title = doc.Info.Title
		spec.Version = doc.Info.Version
	}
	spec.BaseURL = firstServerURL(doc)

	if doc.Paths == nil {
		return spec
	}

	paths := doc.Paths.Map()
	// Deterministic ordering for reproducible scans.
	pathKeys := make([]string, 0, len(paths))
	for p := range paths {
		pathKeys = append(pathKeys, p)
	}
	sort.Strings(pathKeys)

	for _, path := range pathKeys {
		item := paths[path]
		if item == nil {
			continue
		}
		// Path-level parameters apply to every operation on the path.
		pathParams := convertParams(item.Parameters)

		for method, op := range item.Operations() {
			if op == nil {
				continue
			}
			ep := Endpoint{
				Path:        path,
				Method:      strings.ToUpper(method),
				OperationID: op.OperationID,
				Summary:     op.Summary,
				Tags:        op.Tags,
				Secured:     op.Security != nil && len(*op.Security) > 0,
			}
			ep.Parameters = append(ep.Parameters, pathParams...)
			ep.Parameters = append(ep.Parameters, convertParams(op.Parameters)...)
			ep.RequestBody = convertBody(op.RequestBody)
			ep.Parameters = append(ep.Parameters, bodyParams(ep.RequestBody)...)
			spec.Endpoints = append(spec.Endpoints, ep)
		}
	}
	return spec
}

func firstServerURL(doc *openapi3.T) string {
	if len(doc.Servers) > 0 {
		return doc.Servers[0].URL
	}
	return ""
}

// convertParams maps kin-openapi parameters to our ParamInfo.
func convertParams(refs openapi3.Parameters) []ParamInfo {
	var out []ParamInfo
	for _, ref := range refs {
		if ref == nil || ref.Value == nil {
			continue
		}
		p := ref.Value
		out = append(out, ParamInfo{
			Name:     p.Name,
			In:       p.In,
			Required: p.Required,
			Type:     schemaType(p.Schema),
			Example:  exampleString(p.Example),
		})
	}
	return out
}

// convertBody maps a request body ref to BodyInfo, preferring JSON content.
func convertBody(ref *openapi3.RequestBodyRef) *BodyInfo {
	if ref == nil || ref.Value == nil {
		return nil
	}
	body := ref.Value
	for _, ct := range []string{"application/json", "application/xml"} {
		if mt := body.Content.Get(ct); mt != nil {
			info := &BodyInfo{
				ContentType: ct,
				Required:    body.Required,
				Fields:      schemaFields(mt.Schema),
			}
			if mt.Example != nil {
				info.Example = exampleString(mt.Example)
			}
			return info
		}
	}
	// Fall back to any available content type.
	for ct, mt := range body.Content {
		return &BodyInfo{
			ContentType: ct,
			Required:    body.Required,
			Fields:      schemaFields(mt.Schema),
		}
	}
	return nil
}

// bodyParams turns top-level JSON body fields into testable ParamInfo entries.
func bodyParams(body *BodyInfo) []ParamInfo {
	if body == nil {
		return nil
	}
	var out []ParamInfo
	for _, f := range body.Fields {
		out = append(out, ParamInfo{Name: f, In: "body", Type: "string"})
	}
	return out
}

func schemaType(ref *openapi3.SchemaRef) string {
	if ref == nil || ref.Value == nil {
		return "string"
	}
	if t := ref.Value.Type; t != nil && len(*t) > 0 {
		return (*t)[0]
	}
	return "string"
}

func schemaFields(ref *openapi3.SchemaRef) []string {
	if ref == nil || ref.Value == nil {
		return nil
	}
	var fields []string
	for name := range ref.Value.Properties {
		fields = append(fields, name)
	}
	sort.Strings(fields)
	return fields
}

func exampleString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}
}
