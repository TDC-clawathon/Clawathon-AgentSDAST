package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"agentdast/internal/core"
	"agentdast/internal/output"
	"agentdast/pkg/types"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan a REST API for vulnerabilities",
	RunE:  runScan,
}

func init() {
	f := scanCmd.Flags()
	f.String("url", "", "scan a single endpoint: a full URL or a path (with --target)")
	f.String("method", "GET", "HTTP method for --url")
	f.String("data", "", "raw request body for --url (e.g. JSON for POST)")
	f.StringSlice("body-param", nil, "JSON body field names to fuzz for --url")
	f.String("swagger", "", "path or URL to an OpenAPI/Swagger spec (scans all endpoints)")
	f.String("target", "", "base URL: overrides the spec server, or resolves a path --url")
	f.StringArrayP("header", "H", nil, "custom header 'Name: Value' (repeatable)")
	f.StringArray("param", nil, "extra query param 'name=value' (repeatable)")
	f.StringSlice("plugin", nil, "plugin names to run (default: all)")
	f.StringSlice("insert-point", nil, "where to inject: name, or 'location:name' (query|header|path|cookie|body); repeatable, default all params")
	f.String("output-mode", "results", "result detail: results | full")
	f.String("output", "text", "output format: text | json | markdown")
	f.String("out", "", "write output to this file (default: stdout)")
	f.Int("timeout", 10, "per-request timeout in seconds")
	f.Int("concurrency", 5, "number of concurrent workers")
	f.Bool("insecure", false, "skip TLS certificate verification")
	f.Bool("follow-redirects", false, "follow HTTP redirects")
}

func runScan(cmd *cobra.Command, _ []string) error {
	f := cmd.Flags()
	url, _ := f.GetString("url")
	method, _ := f.GetString("method")
	data, _ := f.GetString("data")
	bodyParams, _ := f.GetStringSlice("body-param")
	swagger, _ := f.GetString("swagger")
	target, _ := f.GetString("target")
	headerList, _ := f.GetStringArray("header")
	paramList, _ := f.GetStringArray("param")
	pluginList, _ := f.GetStringSlice("plugin")
	insertPoints, _ := f.GetStringSlice("insert-point")
	outputMode, _ := f.GetString("output-mode")
	outFormat, _ := f.GetString("output")
	outFile, _ := f.GetString("out")
	timeout, _ := f.GetInt("timeout")
	concurrency, _ := f.GetInt("concurrency")
	insecure, _ := f.GetBool("insecure")
	followRedirects, _ := f.GetBool("follow-redirects")

	if url == "" && swagger == "" {
		return fmt.Errorf("provide --url (single endpoint) or --swagger (whole spec)")
	}

	cfg := types.ScanConfig{
		TargetURL:          url,
		Method:             method,
		Body:               data,
		BodyParams:         bodyParams,
		SwaggerSource:      swagger,
		TargetBaseURL:      target,
		CustomHeaders:      parseHeaders(headerList),
		CustomParams:       parseParams(paramList),
		Plugins:            pluginList,
		InsertPoints:       insertPoints,
		OutputMode:         types.OutputMode(outputMode),
		Timeout:            timeout,
		Concurrency:        concurrency,
		InsecureSkipVerify: insecure,
		FollowRedirects:    followRedirects,
	}

	scanner := core.NewScanner(registry())
	result, err := scanner.Run(context.Background(), cfg)
	if err != nil {
		// A failed scan still produces a result we can render.
		fmt.Fprintf(os.Stderr, "scan error: %v\n", err)
	}

	rendered, ferr := output.New(outFormat).Format(result)
	if ferr != nil {
		return ferr
	}
	if outFile != "" {
		if werr := os.WriteFile(outFile, rendered, 0o644); werr != nil {
			return werr
		}
		fmt.Printf("wrote %s (%d findings)\n", outFile, result.Summary.TotalFindings)
		return nil
	}
	fmt.Println(string(rendered))
	return nil
}

// parseHeaders converts "Name: Value" strings into a map.
func parseHeaders(in []string) map[string]string {
	out := make(map[string]string, len(in))
	for _, h := range in {
		if idx := strings.Index(h, ":"); idx > 0 {
			out[strings.TrimSpace(h[:idx])] = strings.TrimSpace(h[idx+1:])
		}
	}
	return out
}

// parseParams converts "name=value" strings into a map.
func parseParams(in []string) map[string]string {
	out := make(map[string]string, len(in))
	for _, p := range in {
		if idx := strings.Index(p, "="); idx > 0 {
			out[strings.TrimSpace(p[:idx])] = strings.TrimSpace(p[idx+1:])
		}
	}
	return out
}
