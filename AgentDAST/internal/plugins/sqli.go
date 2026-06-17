package plugins

import (
	"agentdast/internal/core"
	"agentdast/internal/parser"
	"agentdast/pkg/types"
)

// SQLInjectionPlugin detects SQL injection with three techniques:
//   - error-based: a database error signature appears in the response
//   - boolean-based blind: a TRUE condition matches the baseline while a FALSE
//     condition diverges significantly
//   - time-based blind: a SLEEP/pg_sleep payload measurably delays the response
type SQLInjectionPlugin struct{}

func (p *SQLInjectionPlugin) Name() string             { return "sqli" }
func (p *SQLInjectionPlugin) Category() string         { return "injection" }
func (p *SQLInjectionPlugin) Severity() types.Severity { return types.SeverityCritical }
func (p *SQLInjectionPlugin) Description() string {
	return "Detects SQL injection via error signatures, boolean-based blind, and time-based blind techniques"
}

func (p *SQLInjectionPlugin) DefaultPayloads() []string {
	return []string{"'", "\"", "' OR '1'='1", "' AND '1'='2", "' AND SLEEP(3)-- -"}
}

// sqlErrors are substrings that strongly indicate a leaked SQL error, drawn from
// the major engines. None appear in the payloads above, and matchSignal strips
// reflected input before matching, so an echoed payload cannot trigger them.
var sqlErrors = []string{
	// MySQL / MariaDB
	"you have an error in your sql syntax",
	"warning: mysql", "mysqli_", "com.mysql.jdbc", "valid mysql result",
	"check the manual that corresponds to your mysql server version",
	// PostgreSQL
	"pg_query", "pg::syntaxerror", "postgresql", "syntax error at or near",
	"unterminated quoted string", "invalid input syntax for",
	// SQLite (juice-shop and many Node/sequelize apps)
	"sqlite_error", "sqlite3::", "sqlitexception", "unrecognized token",
	"sqlite3.operationalerror", "no such column",
	// Oracle
	"ora-00933", "ora-01756", "ora-00921", "ora-00936", "ora-00942",
	"quoted string not properly terminated",
	// MS SQL Server
	"unclosed quotation mark after the character string",
	"incorrect syntax near", "conversion failed when converting",
	"microsoft odbc", "microsoft sql server", "odbc sql server driver", "native client",
	// Generic / framework wrappers
	"sqlstate[", "sql syntax", "syntax error in string in query expression",
	"sequelizedatabaseerror", "querysqlerror", "fatal error: uncaught",
}

const (
	sleepSeconds      = 3
	sleepThresholdMS  = 2500 // delay attributable to the injected SLEEP
	booleanDivergence = 0.30 // relative length difference signalling true/false split
)

func (p *SQLInjectionPlugin) Test(ctx *core.ScanContext) []types.Finding {
	var findings []types.Finding
	for _, param := range ctx.Params() {
		if ctx.Ctx.Err() != nil {
			return findings
		}
		if f, ok := p.testParam(ctx, param); ok {
			findings = append(findings, f)
		}
	}
	return findings
}

func (p *SQLInjectionPlugin) testParam(ctx *core.ScanContext, param parser.ParamInfo) (types.Finding, bool) {
	orig := param.Example
	if orig == "" {
		orig = "1"
	}

	// 1) Error-based — inject meta-characters (incl. WAF-bypass variants), look
	// for a DB error signature.
	for _, pl := range []string{"'", "\"", "')", "\"))", "`", "';", "'/**/", "%27"} {
		log := ctx.Inject(param, orig+pl)
		if log.Error != "" {
			continue
		}
		if sig, ok := matchSignal(log.ResponseBody, orig+pl, sqlErrors); ok {
			return types.Finding{
				Title:    "SQL injection (error-based) in parameter " + param.Name,
				Endpoint: ctx.Endpoint.Path, Method: ctx.Endpoint.Method,
				ParamName: param.Name, ParamIn: param.In, Payload: orig + pl,
				Evidence:    "database error signature: " + sig + "\n" + evidence(log),
				Confidence:  types.ConfidenceConfirmed,
				Description: "Injecting a SQL meta-character produced a database error, proving input reaches a SQL query unsanitized.",
				Remediation: "Use parameterized queries / prepared statements; never concatenate input into SQL.",
				RequestLog:  logForMode(ctx, log),
			}, true
		}
	}

	// 2) Boolean-based blind — TRUE should resemble the baseline; FALSE should differ.
	baseline := ctx.Inject(param, orig)
	if baseline.Error == "" {
		truePayload := orig + "' AND '1'='1"
		falsePayload := orig + "' AND '1'='2"
		tResp := ctx.Inject(param, truePayload)
		fResp := ctx.Inject(param, falsePayload)
		if tResp.Error == "" && fResp.Error == "" &&
			tResp.StatusCode == baseline.StatusCode &&
			lengthDelta(baseline.ResponseBody, tResp.ResponseBody) < 0.10 &&
			lengthDelta(tResp.ResponseBody, fResp.ResponseBody) >= booleanDivergence {
			return types.Finding{
				Title:    "SQL injection (boolean-based blind) in parameter " + param.Name,
				Endpoint: ctx.Endpoint.Path, Method: ctx.Endpoint.Method,
				ParamName: param.Name, ParamIn: param.In, Payload: truePayload,
				Evidence: "TRUE condition matched baseline; FALSE condition diverged\n" +
					"baseline=" + itoa(len(baseline.ResponseBody)) + "B true=" + itoa(len(tResp.ResponseBody)) +
					"B false=" + itoa(len(fResp.ResponseBody)) + "B\n" + evidence(fResp),
				Confidence:  types.ConfidenceConfirmed,
				Description: "Boolean SQL conditions altered the result set, confirming injection without an error message.",
				Remediation: "Use parameterized queries / prepared statements; never concatenate input into SQL.",
				RequestLog:  logForMode(ctx, tResp),
			}, true
		}
	}

	// 3) Time-based blind — a SLEEP payload should delay the response.
	if baseline.Error == "" && baseline.DurationMS < sleepThresholdMS {
		for _, pl := range []string{orig + "' AND SLEEP(3)-- -", orig + "'; SELECT pg_sleep(3)-- -", orig + " AND SLEEP(3)"} {
			log := ctx.Inject(param, pl)
			if log.Error != "" || log.DurationMS-baseline.DurationMS < sleepThresholdMS {
				continue
			}
			// Confirm once to rule out a transient slow response.
			confirm := ctx.Inject(param, pl)
			if confirm.Error == "" && confirm.DurationMS-baseline.DurationMS >= sleepThresholdMS {
				return types.Finding{
					Title:    "SQL injection (time-based blind) in parameter " + param.Name,
					Endpoint: ctx.Endpoint.Path, Method: ctx.Endpoint.Method,
					ParamName: param.Name, ParamIn: param.In, Payload: pl,
					Evidence:    "response delayed ~" + itoa(int(log.DurationMS-baseline.DurationMS)) + "ms by an injected SLEEP (baseline " + itoa(int(baseline.DurationMS)) + "ms)\n" + evidence(log),
					Confidence:  types.ConfidenceConfirmed,
					Description: "An injected SLEEP delayed the response, confirming time-based blind SQL injection.",
					Remediation: "Use parameterized queries / prepared statements; never concatenate input into SQL.",
					RequestLog:  logForMode(ctx, log),
				}, true
			}
		}
	}

	return types.Finding{}, false
}
