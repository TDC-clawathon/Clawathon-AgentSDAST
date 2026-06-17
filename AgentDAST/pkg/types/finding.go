package types

// Severity ranks the impact of a finding.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info"
)

// Rank returns a numeric weight for sorting (higher = more severe).
func (s Severity) Rank() int {
	switch s {
	case SeverityCritical:
		return 5
	case SeverityHigh:
		return 4
	case SeverityMedium:
		return 3
	case SeverityLow:
		return 2
	case SeverityInfo:
		return 1
	default:
		return 0
	}
}

// Confidence levels describe how certain the scanner is about a finding.
const (
	ConfidenceConfirmed = "confirmed"
	ConfidenceProbable  = "probable"
	ConfidencePossible  = "possible"
)

// Finding is one confirmed or suspected vulnerability instance.
type Finding struct {
	ID          string      `json:"id"`
	Plugin      string      `json:"plugin"`
	Category    string      `json:"category"`
	Severity    Severity    `json:"severity"`
	Title       string      `json:"title"`
	Endpoint    string      `json:"endpoint"`
	Method      string      `json:"method"`
	ParamName   string      `json:"param_name,omitempty"`
	ParamIn     string      `json:"param_in,omitempty"` // query|body|header|path|cookie
	Payload     string      `json:"payload,omitempty"`
	Evidence    string      `json:"evidence,omitempty"` // truncated request+response snippet
	Description string      `json:"description,omitempty"`
	Remediation string      `json:"remediation,omitempty"`
	Confidence  string      `json:"confidence"` // confirmed|probable|possible
	RequestLog  *RequestLog `json:"request_log,omitempty"`
}
