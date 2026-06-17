package parser

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

// LoadSpec accepts a file path or an HTTP(S) URL and returns a normalized ParsedSpec.
// It transparently converts Swagger 2.0 documents to OpenAPI 3.
func LoadSpec(source string) (*ParsedSpec, error) {
	data, err := readSource(source)
	if err != nil {
		return nil, err
	}

	doc, err := parseDocument(data, source)
	if err != nil {
		return nil, err
	}

	return extractSpec(doc), nil
}

// readSource fetches the raw bytes of the spec from a URL or local file.
func readSource(source string) ([]byte, error) {
	if isURL(source) {
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Get(source)
		if err != nil {
			return nil, fmt.Errorf("fetch spec from URL: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("fetch spec: unexpected status %d", resp.StatusCode)
		}
		return io.ReadAll(resp.Body)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return nil, fmt.Errorf("read spec file: %w", err)
	}
	return data, nil
}

// parseDocument decodes the spec bytes into an openapi3.T, converting from
// Swagger 2.0 first when necessary.
func parseDocument(data []byte, source string) (*openapi3.T, error) {
	if isSwaggerV2(data) {
		var v2 openapi2.T
		if err := unmarshalAuto(data, &v2); err != nil {
			return nil, fmt.Errorf("parse swagger 2.0: %w", err)
		}
		doc, err := openapi2conv.ToV3(&v2)
		if err != nil {
			return nil, fmt.Errorf("convert swagger 2.0 to openapi 3: %w", err)
		}
		return doc, nil
	}

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	var (
		doc *openapi3.T
		err error
	)
	if isURL(source) {
		u, perr := url.Parse(source)
		if perr != nil {
			return nil, fmt.Errorf("invalid spec URL: %w", perr)
		}
		doc, err = loader.LoadFromDataWithPath(data, u)
	} else {
		doc, err = loader.LoadFromData(data)
	}
	if err != nil {
		return nil, fmt.Errorf("parse openapi 3 spec: %w", err)
	}
	return doc, nil
}

// unmarshalAuto decodes JSON or YAML bytes into v. kin-openapi's JSON
// unmarshaler handles JSON; YAML is converted first.
func unmarshalAuto(data []byte, v interface{}) error {
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "{") {
		// JSON
		if u, ok := v.(*openapi2.T); ok {
			return u.UnmarshalJSON(data)
		}
		return yaml.Unmarshal(data, v)
	}
	// YAML
	return yaml.Unmarshal(data, v)
}

// isSwaggerV2 reports whether the document declares swagger: "2.0".
func isSwaggerV2(data []byte) bool {
	s := string(data)
	return strings.Contains(s, `"swagger"`) && strings.Contains(s, `"2.0"`) ||
		strings.Contains(s, "swagger:") && strings.Contains(s, "2.0")
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
