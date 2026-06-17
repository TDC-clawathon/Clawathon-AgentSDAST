package mcp

import "encoding/json"

// Tool names exposed by the MCP server.
const (
	toolScanAPI       = "scan_api"
	toolListPlugins   = "list_plugins"
	toolGetScanResult = "get_scan_result"
	toolGetKnowledge  = "get_knowledge"
)

// scanAPISchema is shared by the MCP scan_api tool and the AI function definition.
// A scan targets a single endpoint (target_url) or a whole spec (swagger_source).
var scanAPISchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "target_url": {"type": "string", "description": "A single endpoint to scan: a full URL (http://host/path?x=y) or a path (/path?x=y) resolved against the configured base URL. Preferred for targeted testing."},
    "method": {"type": "string", "description": "HTTP method for target_url (GET, POST, PUT, PATCH, DELETE). Default GET."},
    "body": {"type": "string", "description": "Raw request body for target_url (e.g. JSON for POST/PUT)."},
    "body_params": {"type": "array", "items": {"type": "string"}, "description": "JSON body field names to fuzz for target_url."},
    "swagger_source": {"type": "string", "description": "Alternative to target_url: file path or URL to an OpenAPI/Swagger spec to scan ALL of its endpoints."},
    "target_base_url": {"type": "string", "description": "Base URL for resolving a path target_url or overriding the spec server."},
    "plugins": {"type": "array", "items": {"type": "string"}, "description": "Plugin names to run; omit to run all. Available: sqli, xss, cmdi, path_traversal, idor, auth, sensitive_data, cors, xxe, ssrf, mass_assignment, open_redirect, ssti"},
    "insert_point": {"type": "array", "items": {"type": "string"}, "description": "Where to inject payloads: a bare name or 'location:name' (query|header|path|cookie|body). Multiple allowed; undeclared points are still injected. Omit for all declared params."},
    "headers": {"type": "object", "additionalProperties": {"type": "string"}, "description": "Headers injected into every request (e.g. Authorization)"},
    "params": {"type": "object", "additionalProperties": {"type": "string"}, "description": "Extra query params added to every request"},
    "output_mode": {"type": "string", "enum": ["full", "results"], "description": "full includes all request logs"},
    "timeout_secs": {"type": "integer", "description": "Per-request timeout in seconds (default 10)"},
    "concurrency": {"type": "integer", "description": "Max concurrent workers (default 5)"}
  }
}`)

var listPluginsSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "category": {"type": "string", "description": "Filter by category: injection, auth, exposure, config"}
  }
}`)

var getScanResultSchema = json.RawMessage(`{
  "type": "object",
  "required": ["scan_id"],
  "properties": {
    "scan_id": {"type": "string"},
    "include_logs": {"type": "boolean", "description": "Include full request logs"}
  }
}`)

var getKnowledgeSchema = json.RawMessage(`{
  "type": "object",
  "required": ["vuln"],
  "properties": {
    "vuln": {"type": "string", "description": "Vulnerability name or alias (e.g. idor, sqli, ssrf, bola, \"sql injection\"). Returns reference knowledge on how to test, confirm, and remediate it."}
  }
}`)

// toolDefinitions returns the static list of tools advertised to MCP clients.
func toolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{Name: toolScanAPI, Description: "Run a DAST scan against a REST API described by an OpenAPI/Swagger spec", InputSchema: scanAPISchema},
		{Name: toolListPlugins, Description: "List available vulnerability scanner plugins", InputSchema: listPluginsSchema},
		{Name: toolGetScanResult, Description: "Retrieve a previously completed scan result by ID (set include_logs for full request/response logs)", InputSchema: getScanResultSchema},
		{Name: toolGetKnowledge, Description: "Get reference knowledge about a vulnerability class (how to test, confirm, avoid false positives, remediate)", InputSchema: getKnowledgeSchema},
	}
}
