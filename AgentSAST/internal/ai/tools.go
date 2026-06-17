package ai

import (
	"encoding/json"

	openai "github.com/sashabaranov/go-openai"
)

// Tool names presented to the model. All operate on the job workspace.
const (
	fnExtractArchive  = "extract_archive"
	fnListFiles       = "list_files"
	fnReadFile        = "read_file"
	fnSearchCode      = "search_code"
	fnGetKnowledge    = "get_knowledge"
	fnWriteArtifact   = "write_artifact"
	fnValidateOpenAPI = "validate_openapi"
)

var extractArchiveParams = json.RawMessage(`{
  "type": "object",
  "required": ["path"],
  "properties": {
    "path": {"type": "string", "description": "Relative path of the archive under the workspace, e.g. raw/source.zip"},
    "dest": {"type": "string", "description": "Relative destination dir (default raw/extracted). Created if needed."}
  }
}`)

var listFilesParams = json.RawMessage(`{
  "type": "object",
  "properties": {
    "glob": {"type": "string", "description": "Optional path glob, e.g. **/*.go or raw/** . Empty lists everything."},
    "max": {"type": "integer", "description": "Max entries to return (default 600)."}
  }
}`)

var readFileParams = json.RawMessage(`{
  "type": "object",
  "required": ["path"],
  "properties": {
    "path": {"type": "string", "description": "Relative file path under the workspace."},
    "max_bytes": {"type": "integer", "description": "Max bytes to read (default 40000, hard cap 120000)."},
    "offset": {"type": "integer", "description": "Byte offset to start reading from (default 0)."}
  }
}`)

var searchCodeParams = json.RawMessage(`{
  "type": "object",
  "required": ["pattern"],
  "properties": {
    "pattern": {"type": "string", "description": "Go (RE2) regular expression to search for in file contents."},
    "glob": {"type": "string", "description": "Optional path glob to restrict the search."},
    "max_results": {"type": "integer", "description": "Max matching lines to return (default 100)."}
  }
}`)

var getKnowledgeParams = json.RawMessage(`{
  "type": "object",
  "required": ["topic"],
  "properties": {
    "topic": {"type": "string", "description": "Vulnerability topic or alias: idor, sqli, xss, payment (bola/sql/etc. accepted). If none exists, rely on your own expertise."}
  }
}`)

var writeArtifactParams = json.RawMessage(`{
  "type": "object",
  "required": ["name", "content"],
  "properties": {
    "name": {"type": "string", "enum": ["openapi.yaml", "report.md", "base_url.txt"], "description": "Which deliverable to write under sast/."},
    "content": {"type": "string", "description": "Full file content. For base_url.txt write only the verified base URL on one line."}
  }
}`)

var validateOpenAPIParams = json.RawMessage(`{
  "type": "object",
  "properties": {
    "name": {"type": "string", "description": "Artifact to validate (default openapi.yaml)."}
  }
}`)

// toolDefinitions returns the OpenAI tool list exposed to the model.
func toolDefinitions() []openai.Tool {
	def := func(name, desc string, params json.RawMessage) openai.Tool {
		return openai.Tool{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{
			Name: name, Description: desc, Parameters: params,
		}}
	}
	return []openai.Tool{
		def(fnExtractArchive, "Extract a .zip/.tar/.tar.gz/.tgz archive found in the workspace into a directory. Use this first if ./raw contains an archive.", extractArchiveParams),
		def(fnListFiles, "List files in the workspace (optionally filtered by a glob). Use it to discover the project layout, routers, controllers, models.", listFilesParams),
		def(fnReadFile, "Read a source file's contents (capped). Open the actual files — do not guess.", readFileParams),
		def(fnSearchCode, "Regex-search file contents across the workspace to locate routes, sinks (SQL, exec, file, template, redirect), and auth checks.", searchCodeParams),
		def(fnGetKnowledge, "Fetch reference material for a vulnerability class (how to detect/confirm, test cases). If no document exists, use your own expertise.", getKnowledgeParams),
		def(fnWriteArtifact, "Write one of the three deliverables under sast/: openapi.yaml, report.md, or base_url.txt. Call once per file (re-call to overwrite/fix).", writeArtifactParams),
		def(fnValidateOpenAPI, "Validate the OpenAPI document you wrote (sast/openapi.yaml). Returns 'valid' or the exact error to fix. Keep fixing and re-validating until valid.", validateOpenAPIParams),
	}
}
