package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"agentdast/internal/parser"
	"agentdast/pkg/types"
)

// TestInjectByLocation confirms a payload lands in the exact location named by
// the insert point — including a header/cookie not declared in the spec.
func TestInjectByLocation(t *testing.T) {
	var (
		gotQuery  string
		gotHeader string
		gotCookie string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("q")
		gotHeader = r.Header.Get("X-Inject")
		if c, err := r.Cookie("sid"); err == nil {
			gotCookie = c.Value
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	cfg := types.ScanConfig{}.WithDefaults()
	ex := NewRequestExecutor(cfg)
	ep := parser.Endpoint{Path: "/", Method: http.MethodGet}

	cases := []struct {
		name  string
		param parser.ParamInfo
		check func() string
	}{
		{"query", parser.ParamInfo{In: "query", Name: "q"}, func() string { return gotQuery }},
		{"header", parser.ParamInfo{In: "header", Name: "X-Inject"}, func() string { return gotHeader }},
		{"cookie", parser.ParamInfo{In: "cookie", Name: "sid"}, func() string { return gotCookie }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotQuery, gotHeader, gotCookie = "", "", ""
			log := ex.Inject(context.Background(), ep, srv.URL, tc.param, "PAYLOAD-"+tc.name, cfg)
			if log.Error != "" {
				t.Fatalf("inject error: %s", log.Error)
			}
			if got := tc.check(); got != "PAYLOAD-"+tc.name {
				t.Fatalf("%s injection: server saw %q, want PAYLOAD-%s", tc.name, got, tc.name)
			}
		})
	}
}
