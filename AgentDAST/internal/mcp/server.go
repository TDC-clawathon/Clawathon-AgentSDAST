package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"

	"agentdast/internal/toolexec"
)

// Server is an MCP JSON-RPC 2.0 server exposing the DAST scanner.
type Server struct {
	exec    *toolexec.Executor
	version string
}

// NewServer constructs an MCP server backed by the given tool executor.
func NewServer(exec *toolexec.Executor, version string) *Server {
	return &Server{exec: exec, version: version}
}

// ServeStdio runs the server over stdin/stdout using newline-delimited JSON.
func (s *Server) ServeStdio(ctx context.Context, in io.Reader, out io.Writer) error {
	return s.serve(ctx, in, out)
}

// ServeTCP listens on addr and serves one connection at a time.
func (s *Server) ServeTCP(ctx context.Context, addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go func() {
			defer conn.Close()
			_ = s.serve(ctx, conn, conn)
		}()
	}
}

// serve reads newline-delimited JSON-RPC messages and writes responses.
func (s *Server) serve(ctx context.Context, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	var writeMu sync.Mutex

	write := func(resp *Response) {
		writeMu.Lock()
		defer writeMu.Unlock()
		data, _ := json.Marshal(resp)
		fmt.Fprintf(out, "%s\n", data)
	}

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			write(&Response{JSONRPC: jsonRPCVersion, Error: &RPCError{Code: codeParseError, Message: err.Error()}})
			continue
		}
		resp := s.handle(ctx, &req)
		if resp != nil {
			write(resp)
		}
	}
	return scanner.Err()
}

// handle routes a single request and returns a response (nil for notifications).
func (s *Server) handle(ctx context.Context, req *Request) *Response {
	var (
		result interface{}
		rpcErr *RPCError
	)

	switch req.Method {
	case "initialize":
		result = initializeResult{
			ProtocolVersion: protocolVersion,
			Capabilities:    serverCapabilities{Tools: &toolsCapability{}},
			ServerInfo:      serverInfo{Name: "agentdast", Version: s.version},
		}
	case "notifications/initialized", "initialized":
		return nil // notification, no response
	case "ping":
		result = struct{}{}
	case "tools/list":
		result = toolsListResult{Tools: toolDefinitions()}
	case "tools/call":
		result, rpcErr = s.handleToolCall(ctx, req.Params)
	default:
		rpcErr = &RPCError{Code: codeMethodNotFound, Message: "unknown method: " + req.Method}
	}

	if req.IsNotification() {
		return nil
	}
	return &Response{JSONRPC: jsonRPCVersion, ID: req.ID, Result: result, Error: rpcErr}
}
