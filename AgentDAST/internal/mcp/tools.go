package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"agentdast/internal/output"
)

// handleToolCall dispatches a tools/call request to the appropriate handler.
func (s *Server) handleToolCall(ctx context.Context, params json.RawMessage) (interface{}, *RPCError) {
	var call toolCallParams
	if err := json.Unmarshal(params, &call); err != nil {
		return nil, &RPCError{Code: codeInvalidParams, Message: err.Error()}
	}

	switch call.Name {
	case toolScanAPI:
		return s.toolScanAPI(ctx, call.Arguments), nil
	case toolListPlugins:
		return s.toolListPlugins(call.Arguments), nil
	case toolGetScanResult:
		return s.toolGetScanResult(ctx, call.Arguments), nil
	case toolGetKnowledge:
		return s.toolGetKnowledge(call.Arguments), nil
	default:
		return nil, &RPCError{Code: codeMethodNotFound, Message: "unknown tool: " + call.Name}
	}
}

func (s *Server) toolGetKnowledge(args json.RawMessage) toolCallResult {
	var in struct {
		Vuln string `json:"vuln"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	return textResult(s.exec.Knowledge(in.Vuln))
}

func (s *Server) toolScanAPI(ctx context.Context, args json.RawMessage) toolCallResult {
	result, err := s.exec.Scan(ctx, args)
	if err != nil {
		return errorResult("scan failed: " + err.Error())
	}
	// Return findings as markdown plus the scan ID for later retrieval.
	md := output.Markdown(result)
	return textResult(fmt.Sprintf("scan_id: %s\n\n%s", result.ID, md))
}

func (s *Server) toolListPlugins(args json.RawMessage) toolCallResult {
	var in struct {
		Category string `json:"category"`
	}
	_ = json.Unmarshal(args, &in)
	plugins := s.exec.ListPlugins(in.Category)
	data, _ := json.MarshalIndent(plugins, "", "  ")
	return textResult(string(data))
}

func (s *Server) toolGetScanResult(ctx context.Context, args json.RawMessage) toolCallResult {
	var in struct {
		ScanID      string `json:"scan_id"`
		IncludeLogs bool   `json:"include_logs"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	result, err := s.exec.GetResult(ctx, in.ScanID, in.IncludeLogs)
	if err != nil {
		return errorResult(err.Error())
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return textResult(string(data))
}
