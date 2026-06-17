package ai

import (
	"encoding/json"

	openai "github.com/sashabaranov/go-openai"
)

// Tool names presented to the model. They mirror the MCP tools.
const (
	fnScanAPI      = "scan_api"
	fnListPlugins  = "list_plugins"
	fnHTTPRequest  = "http_request"
	fnGetKnowledge = "get_knowledge"
	fnGetScanLogs  = "get_scan_logs"
)

// scanAPIParams is the JSON schema the model uses to call scan_api. The model
// names a specific endpoint to test; the live base URL is preconfigured.
var scanAPIParams = json.RawMessage(`{
  "type": "object",
  "required": ["target_url"],
  "properties": {
    "target_url": {"type": "string", "description": "The endpoint to scan: a path like /rest/products/search?q=test (resolved against the configured target) or a full URL. Include realistic query values."},
    "method": {"type": "string", "description": "HTTP method (GET, POST, PUT, PATCH, DELETE). Default GET."},
    "body": {"type": "string", "description": "Raw JSON request body for POST/PUT/PATCH endpoints."},
    "body_params": {"type": "array", "items": {"type": "string"}, "description": "JSON body field names to fuzz."},
    "plugins": {"type": "array", "items": {"type": "string"}, "description": "Vulnerability types to test. Usually pass exactly ONE to test a specific hypothesis. Available: sqli, xss, cmdi, path_traversal, idor, auth, sensitive_data, cors, xxe, ssrf, mass_assignment, open_redirect, ssti"},
    "insert_point": {"type": "array", "items": {"type": "string"}, "description": "Where to inject payloads. Each entry is a bare name (any location) or 'location:name' where location is query|header|path|cookie|body. Examples: [\"query:q\"], [\"header:X-Api-Version\"], [\"q\",\"id\"] to test two params. A point not declared in the spec is still injected (e.g. a custom header). Omit to test all declared parameters."},
    "headers": {"type": "object", "additionalProperties": {"type": "string"}, "description": "Request headers. Set Authorization here to scan an endpoint as an authenticated user when it requires a token."},
    "output_mode": {"type": "string", "enum": ["full", "results"]}
  }
}`)

var listPluginsParams = json.RawMessage(`{
  "type": "object",
  "properties": {"category": {"type": "string"}}
}`)

// httpRequestParams lets the model send a fully custom request for cases the
// built-in plugins do not cover (auth flows, business-logic checks, etc.).
var httpRequestParams = json.RawMessage(`{
  "type": "object",
  "required": ["url"],
  "properties": {
    "method": {"type": "string", "description": "HTTP method. Default GET."},
    "url": {"type": "string", "description": "A path (resolved against the configured target) or a full URL."},
    "headers": {"type": "object", "additionalProperties": {"type": "string"}, "description": "Request headers (overrides the default/auth headers for this call)."},
    "body": {"type": "string", "description": "Raw request body."}
  }
}`)

// getKnowledgeParams fetches reference material for a vulnerability class.
var getKnowledgeParams = json.RawMessage(`{
  "type": "object",
  "required": ["vuln"],
  "properties": {
    "vuln": {"type": "string", "description": "Vulnerability name or alias, e.g. idor, sqli, ssrf, xss, bola, \"sql injection\", \"mass assignment\". Returns how to test/confirm and avoid false positives. If no document exists, rely on your own expertise."}
  }
}`)

// getScanLogsParams retrieves the raw request/response exchanges of a prior scan.
var getScanLogsParams = json.RawMessage(`{
  "type": "object",
  "required": ["scan_id"],
  "properties": {
    "scan_id": {"type": "string", "description": "The scan_id reported by a previous scan_api result."},
    "max": {"type": "integer", "description": "Max number of request/response exchanges to return (default 30)."}
  }
}`)

// toolDefinitions returns the OpenAI tool list exposed to the model.
func toolDefinitions() []openai.Tool {
	return []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        fnScanAPI,
				Description: "Run the DAST scanner against ONE specific endpoint to verify or discover a vulnerability. Provide the endpoint path/URL and (optionally) which plugins to use. Returns concrete findings with evidence. Call it repeatedly to check different endpoints.",
				Parameters:  scanAPIParams,
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        fnListPlugins,
				Description: "List available vulnerability scanner plugins and their categories.",
				Parameters:  listPluginsParams,
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        fnHTTPRequest,
				Description: "Send a fully custom HTTP request and get the raw response (status, headers, body). Use this to test cases the plugins do not cover — auth flows, business logic, manual payloads. Returns the exact response so you can judge it yourself.",
				Parameters:  httpRequestParams,
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        fnGetKnowledge,
				Description: "Fetch reference knowledge about a vulnerability class (how to test, how to confirm, false positives, remediation) before testing it. If no document exists, use your own expertise.",
				Parameters:  getKnowledgeParams,
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        fnGetScanLogs,
				Description: "Retrieve the raw request/response exchanges a scan made (by scan_id). Use this when you are unsure whether a scan result is correct — inspect exactly what was sent and returned, then craft your own payloads and re-test.",
				Parameters:  getScanLogsParams,
			},
		},
	}
}
