// Package plugins contains the built-in vulnerability scanner plugins.
//
// Each plugin implements core.Plugin. The Plugin interface and ScanContext
// live in the core package to avoid an import cycle; this package provides
// the concrete implementations plus shared detection helpers.
package plugins

import (
	"strings"

	"agentdast/internal/core"
	"agentdast/pkg/types"
)

// logForMode returns the triggering request log to attach to a finding.
// The log is always attached as per-finding evidence; the scan-wide full log
// array is controlled separately by the executor's collection setting.
func logForMode(ctx *core.ScanContext, log types.RequestLog) *types.RequestLog {
	l := log
	return &l
}

// truncate shortens s to at most n runes, appending an ellipsis when cut.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// evidence renders a compact request/response snippet for a finding.
func evidence(log types.RequestLog) string {
	var b strings.Builder
	b.WriteString(log.Method)
	b.WriteString(" ")
	b.WriteString(log.URL)
	if log.RequestBody != "" {
		b.WriteString("\n--- request body ---\n")
		b.WriteString(truncate(log.RequestBody, 256))
	}
	b.WriteString("\n--- response ")
	b.WriteString(itoa(log.StatusCode))
	b.WriteString(" ---\n")
	b.WriteString(truncate(log.ResponseBody, 512))
	return b.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// stripPayload removes case-insensitive occurrences of payload from s. This is
// the key defense against reflection false positives: a server that simply
// echoes the injected value back must not be treated as having "leaked" a
// signature that was only ever present because it was in the payload.
func stripPayload(s, payload string) string {
	lower := strings.ToLower(s)
	if payload == "" {
		return lower
	}
	return strings.ReplaceAll(lower, strings.ToLower(payload), " ")
}

// matchSignal returns the first signature found in the response AFTER removing
// any reflected payload, so echoed input cannot trigger a match.
func matchSignal(responseBody, payload string, signatures []string) (string, bool) {
	hay := stripPayload(responseBody, payload)
	for _, sig := range signatures {
		if strings.Contains(hay, strings.ToLower(sig)) {
			return sig, true
		}
	}
	return "", false
}

// lengthDelta returns the absolute relative difference in length between two
// bodies in [0,1]. Used by boolean-based blind detection where a meaningful
// difference between "true" and "false" conditions indicates injection.
func lengthDelta(a, b string) float64 {
	la, lb := float64(len(a)), float64(len(b))
	if la == 0 && lb == 0 {
		return 0
	}
	max := la
	if lb > max {
		max = lb
	}
	diff := la - lb
	if diff < 0 {
		diff = -diff
	}
	return diff / max
}
