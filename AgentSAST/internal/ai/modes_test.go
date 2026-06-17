package ai

import (
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeModes(t *testing.T) {
	cases := []struct {
		in   []string
		want []string
	}{
		{nil, []string{"quickscan"}},                                                                 // default
		{[]string{}, []string{"quickscan"}},                                                          // default
		{[]string{"quickscan"}, []string{"quickscan"}},                                               // quick only
		{[]string{"deepscan"}, []string{"quickscan", "deepscan"}},                                    // deep includes quick
		{[]string{"deepscan", "quickscan"}, []string{"quickscan", "deepscan"}},                       // both -> deep (incl quick)
		{[]string{"quickscan", "pgwscan"}, []string{"quickscan", "pgwscan"}},                         // pgw on quick
		{[]string{"deepscan", "pgwscan"}, []string{"quickscan", "deepscan", "pgwscan"}},              // pgw on deep
		{[]string{"quickscan", "deepscan", "pgwscan"}, []string{"quickscan", "deepscan", "pgwscan"}}, // all -> deep+pgw
		{[]string{"pgwscan"}, []string{"quickscan", "pgwscan"}},                                      // pgw alone -> quick base
		{[]string{"bogus"}, []string{"quickscan"}},                                                   // unknown -> default
		{[]string{"DeepScan", " pgwscan "}, []string{"quickscan", "deepscan", "pgwscan"}},            // case/space tolerant
	}
	for _, c := range cases {
		got := NormalizeModes(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("NormalizeModes(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

// quickscan topics must NOT leak deepscan topics, and vice-versa — proving the
// engine loads only the skills relevant to the selected modes.
func TestLoadSkillsSelectsOnlyRequestedModes(t *testing.T) {
	dir := ResolveSastSkillsDir()
	if dir == "" {
		t.Skip("skills/sast not found from test working dir")
	}

	contains := func(s []string, v string) bool {
		for _, x := range s {
			if x == v {
				return true
			}
		}
		return false
	}

	// quickscan only
	q := LoadSkills(dir, []string{"quickscan"})
	if !reflect.DeepEqual(q.Modes(), []string{"quickscan"}) {
		t.Errorf("quickscan modes = %v", q.Modes())
	}
	if !strings.Contains(q.skillText(), "SCAN MODE: quickscan") {
		t.Error("quickscan skill text missing mode header")
	}
	for _, want := range []string{"idor", "sqli", "xss", "payment"} {
		if !contains(q.list(), want) {
			t.Errorf("quickscan missing topic %q (got %v)", want, q.list())
		}
	}
	if contains(q.list(), "finding-discovery") {
		t.Error("quickscan must NOT load deepscan topics")
	}

	// deepscan -> includes quickscan (Deep includes Quick) + deepscan methodology
	d := LoadSkills(dir, []string{"deepscan"})
	if !reflect.DeepEqual(d.Modes(), []string{"quickscan", "deepscan"}) {
		t.Errorf("deepscan modes = %v, want [quickscan deepscan]", d.Modes())
	}
	for _, want := range []string{"finding-discovery", "threat-model", "validation", "attack-path-analysis"} {
		if !contains(d.list(), want) {
			t.Errorf("deepscan missing topic %q (got %v)", want, d.list())
		}
	}
	for _, want := range []string{"idor", "sqli", "xss", "payment"} {
		if !contains(d.list(), want) {
			t.Errorf("deepscan must include quickscan topic %q (Deep includes Quick)", want)
		}
	}

	// pgwscan alone -> quickscan base + pgw topics
	p := LoadSkills(dir, []string{"pgwscan"})
	if !reflect.DeepEqual(p.Modes(), []string{"quickscan", "pgwscan"}) {
		t.Errorf("pgwscan-alone modes = %v, want [quickscan pgwscan]", p.Modes())
	}
	if !strings.Contains(p.skillText(), "SCAN MODE: pgwscan") {
		t.Error("pgwscan skill text missing pgw mode header")
	}
	if !contains(p.list(), "02-hmac-bypass-methodology") {
		t.Errorf("pgwscan missing hmac reference (got %v)", p.list())
	}

	// quick+deep+pgw -> all three modes, union of topics
	all := LoadSkills(dir, []string{"quickscan", "deepscan", "pgwscan"})
	if !reflect.DeepEqual(all.Modes(), []string{"quickscan", "deepscan", "pgwscan"}) {
		t.Errorf("all modes = %v", all.Modes())
	}
	for _, want := range []string{"idor", "finding-discovery", "02-hmac-bypass-methodology"} {
		if !contains(all.list(), want) {
			t.Errorf("all-modes missing topic %q", want)
		}
	}
}
