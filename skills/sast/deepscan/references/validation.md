# validation (deepscan methodology)


# Security Validation

## Objective

Take candidate findings from discovery and produce the strongest evidence-backed validation assessment you can. Prefer targeted, non-interactive reproduction or falsification when it is feasible and proportionate, but use focused code tracing when dynamic execution is blocked by missing services, unavailable infrastructure, or excessive setup relative to the candidate and scan scope.

## Artifact Resolution

The path references in this skill are the default locations for this phase.
If the user explicitly provides a different path for a required input or output, use the user-provided path instead of the corresponding default path referenced in this skill.
If a required input is still missing, stop and ask the user for it before continuing.
Use the shared scan artifact path conventions in `../../references/scan-artifacts.md`.

## Workflow

1. Before starting, create a detailed validation rubric with up to five criteria for the candidate.
2. For each candidate finding, identify the claimed attacker input, vulnerable sink, and preconditions.
3. Choose the validation path using the strongest realistic method available:
   - crash: for crash, memory-corruption, parser-confusion, or denial-of-service candidates, attempt to compile a debug variant and produce a crashing PoC when the project can be built with bounded effort.
   - valgrind or ASan: if a memory-safety or crash candidate does not immediately reproduce and the build supports it, attempt valgrind and/or ASan.
   - debugger: if runtime execution is available but the chain is unclear, attempt a non-interactive debugger trace with gdb/lldb that shows the source-to-sink path.
   - unit or integration test: if the vulnerable path is covered by an existing test harness, add or adapt the smallest focused test that exercises the vulnerable code and asserts the vulnerable behavior.
   - realistic interface reproduction: if the code exposes a real user-reachable interface such as HTTP, CLI, file parser, RPC, message queue, plugin hook, or package API, attempt a minimal end-to-end reproduction through that interface using crafted input that reaches the suspected sink.
   - code understanding: if dynamic reproduction is not feasible or proportionate after bounded attempts, perform focused code tracing from attacker-controlled input to the sink, identify preconditions and guards, and state whether the vulnerability is supported or defeated by the code path.
   - large internal repository mode: for repository-wide or scoped-path scans where runtime reproduction requires unavailable internal services, secrets, cloud accounts, service meshes, or local production data, use static trace plus existing tests and deploy/config evidence once the candidate has a complete source/control/sink/impact tuple. Missing internal runtime setup is not suppression evidence.
4. For non-compiled stacks, attempt to generate PoCs or targeted commands that exercise the vulnerable path and trigger the vulnerability.
5. For compiled stacks, prefer dynamic validation when it is feasible with bounded setup: build a debug variant or targeted test harness when available, reproduce the vulnerable behavior with a small PoC, then use valgrind, ASan, or a non-interactive debugger trace when those tools materially improve confidence.
6. Save any PoC files, inputs, or logs under that finding's validation artifacts path from `../../references/scan-artifacts.md`.
7. If validation is not feasible, document what was tried, what remains uncertain, and the exact proof gap.
8. Return a clear validation assessment per finding grounded in the evidence, proof gaps, and remaining uncertainty.
9. Save that finding's visible validation report to its per-finding validation report path from `../../references/scan-artifacts.md`.
10. Append one validation receipt per candidate id to that finding's candidate ledger path from `../../references/scan-artifacts.md`. The receipt must record the validation method, evidence or exact proof gap, disposition, and validation artifact/report reference for that candidate finding.

## Usage Guidance

- Prefer short, bounded commands (git, grep -nI within changed dirs, build/test runners, minimal PoCs).
- Avoid interactive editors (vi), long-running repo-wide scans, and network access unless essential.
- If you need to use debuggers, invoke them non-interactively (gdb: "-q -batch -ex run -ex bt -ex quit"; lldb: "-b -o run -o bt -o quit").
- When creating PoCs to validate the vulnerability, you should attempt to trigger them against the actual application/library directly. Ideally this shows how an attacker would trigger the bug.

## Validation Guidance

Follow the instance-preserving validation rules, validation checklist, and confidence guidance in `references/validation-guidance.md`.

## Output Contract

For each candidate finding, include:

- finding title
- candidate id, instance key, and ledger row id when provided
- root-control file:line and affected-location labels from discovery when provided
- advisory/source reference and seed anchor file:line when provided, especially when distinct from the root-control line
- confidence level
- validation method used or recommended
- rubric checklist with `- [x]` or `- [ ]` items
- evidence observed
- concise notes on what was tested
- remaining uncertainty
- minimal next step if more proof is needed
- artifact paths when validation files or logs were created
- enough detail that a later reader can tell whether the finding survived validation without relying on a separate status label

For repository-wide and scoped-path scans, also include a validation closure table with columns:

- ledger row id
- instance key
- advisory/source reference when available
- seed anchor file:line when distinct from the root-control
- root-control file:line
- entrypoint/source
- sink/control
- disposition: `reportable`, `suppressed`, `not_applicable`, or `deferred`
- counterevidence or proof gap
- survives: `yes`, `no`, or `uncertain`

## Hard Rules

- Do not imply validation happened when it did not.
- Do not leave candidate coverage implicit. Every candidate finding that enters validation must leave a validation receipt in its candidate-ledger path from `../../references/scan-artifacts.md`, even when the result is suppressed, uncertain, or deferred.
- Prefer realistic local reproduction paths over contrived setups.
- If a finding depends on missing product assumptions, state the question clearly instead of fabricating the answer.
- Keep commands short, bounded, and non-interactive.
- Use stronger validation methods such as crashing PoCs, valgrind, ASan, debugger traces, focused tests, or realistic interface reproduction before falling back to code understanding when the stack and scan scope make that feasible.
- Calibrate confidence from the validation method and evidence, not from how dangerous the bug class sounds.
- Keep validation artifacts and the final visible report in that finding's validation paths from `../../references/scan-artifacts.md` so the full scan bundle lives together.
- Make a serious, bounded effort to get runtime validation working when it would materially change reportability, confidence, or severity. Consult repository guidance such as `AGENTS.md`, `README.md`, setup docs, test docs, build files, and package-manager metadata to identify the required dependencies, generated files, services, and setup steps.
- For scans that should not modify the target tree, use a disposable copy or generated-artifact directory under that finding's validation artifacts path for builds, generated clients, patched test harnesses, and PoC files. A no-edit target rule does not forbid output-only build copies when they are needed to validate the original code.
- For repository-wide and scoped-path scans, update each affected finding's validation report and closure table as each reportable, suppressed, not_applicable, or deferred row is decided. Do not leave validated rows only in transient notes, terminal logs, or validation artifacts; later phases must be able to reconstruct surviving findings from the saved per-finding validation reports if the scan is interrupted.
- For large repository-wide scans, keep setup/build/debug effort proportionate to the candidate and the remaining high-impact coverage ledger. Do not spend the review budget trying to fully reproduce one internal service when static trace, existing tests, and deploy/config evidence are enough to validate or suppress the candidate.
- In repository-wide and scoped-path validation, once one candidate in a repeated high-impact pattern has a strong proof tuple, switch to sibling candidates from the coverage ledger and validate each by checking the same source, closest control, sink, and impact. Only continue deeper runtime work when it would materially change reportability, severity, or confidence.
- If a repository-wide shard has a promoted same-family finding plus unresolved seeded or root-control rows, close those sibling rows next as reportable, suppressed, or deferred before replacing the review with a more dramatic neighboring finding. Representative proof improves confidence, but it does not close sibling root controls without exact counterevidence.
- If the project or code does not compile/build, diagnose the failure enough to know whether a targeted build, existing test, package API harness, or disposable validation copy can still exercise the original code. Prefer validating the original target over a separate reimplementation.
- Do not treat setup errors, compilation errors, or missing dependencies as immediate counterevidence. Record what blocked runtime proof, then use static trace plus existing tests/config/deploy evidence when setup becomes disproportionate.
- Do not abandon a build, test, or validation command just because it takes time when there is output, resource usage, generated artifacts, or other evidence of progress and no hard evidence of failure. If a long-running command appears inconclusive, check process status, recent logs, output file timestamps, resource usage, or test runner status before stopping or weakening validation.

---
### reference: validation-guidance

# Validation Guidance

Use this guidance when validating candidate security findings.

## Instance-Preserving Validation

Validation does not choose whether the scan is diff-scoped or repository-wide. Use the scope and candidate set provided by the user, discovery report, or top-level security scan workflow. When validation is invoked directly for one named bug, validate that bug only unless the user explicitly provides multiple instances or asks for sibling expansion.

When validation is part of a top-level repository-wide security scan, treat discovery notes, coverage ledgers, and repeated source/sink/control patterns as validation input even if they are not yet numbered as final findings. The ledger is a coverage artifact: preserve rows that are not_applicable, suppressed, deferred, or reportable, and continue bounded high-impact sibling checks needed to complete that provided ledger. This is preserving repository-wide scan scope, not independently expanding a standalone validation request.

For large repository-wide scans, validation should preserve coverage as well as proof depth. Once a candidate has a complete source, closest control, sink, and impact tuple, prefer static trace plus existing tests and deploy/config evidence over lengthy environment bring-up when runtime reproduction needs unavailable internal services, secrets, service meshes, cloud accounts, or production data. Missing runtime setup is a proof-gap note, not counterevidence and not a suppression reason.

If discovery, user scope, or an advisory/tag seed names a specific package, class family, or root-control family, validation must close that exact row. A same-family finding in a neighboring route, parser, deserializer, or auth flow can be used as supporting evidence, but it is not counterevidence for the seeded row. If the seeded row survives, preserve its exact root-control file:line into attack-path analysis and final-report inputs.

Advisory-derived candidate files, functions, or hunks are not findings by themselves, but they are validation obligations for advisory-led repository-wide scans. Validate them against the checked-out local code, then close each exact seed row as `reportable`, `suppressed`, `not_applicable`, or `deferred` with local evidence. If the seed file was opened only during search, still give it an explicit closure row rather than leaving it only in logs.

When the checked-out code still contains a seeded construct, keep that seed row alive until validation decides its local proof tuple. Mark it `reportable` only when local repository evidence independently supports the same source, broken control, sink or security impact, and realistic preconditions; otherwise mark it `deferred` or `suppressed` with the exact missing proof or counterevidence. A stricter deployment assumption, missing downstream application configuration, or a neighboring stronger finding is a precondition or proof gap to state, not counterevidence by itself.

When a high-impact candidate is blocked only by a missing downstream consumer, workflow caller, policy exception, or artifact provenance fact, run one bounded adjacency pass over the most likely evidence sources before suppressing it: generated clients/specs, workflow callers, deploy/policy config, storage or ACL definitions, and package importers. If that pass still leaves the proof gap, keep the row `deferred` rather than treating the gap as counterevidence.

When the input contains multiple candidate instances, preserve that instance inventory:

- validate each candidate instance independently enough to decide whether that exact file/line should survive
- do not collapse multiple candidate instances into one validated finding solely because they share a vulnerability family
- for repeated instances provided by discovery or the security scan workflow, validate a representative exploit path once when feasible, then analytically validate each sibling instance by checking the same proof tuple: attacker-controlled source, missing or incomplete closest control, dangerous sink, and impact
- for repeated vulnerable templates, query builders, parser operations, auth/object endpoints, or shared-helper callers, preserve each independently vulnerable file and sink/control line as a surviving instance when the proof tuple applies. Grouping is acceptable for readability only after every affected instance has its own surviving finding entry or explicit suppressed/deferred row.
- if discovery hands validation a broad family candidate with multiple concrete sink, parser, helper, API-mode, or protected-action lines, expand it into child closure rows before deciding survival. One representative PoC may support the family, but each child still needs its own file/line, source or protected boundary, closest control, sink or action, disposition, and counterevidence.
- when one route or helper exposes multiple same-family operations such as `execute`/`executemany`/`executescript`, `pickle.load`/`pickle.loads`/`yaml.load`/`yaml.load_all`, separate path/file helpers, insert/select/delete/update query builders, or unauthenticated create/delete/reset/admin/job actions, validate or suppress each independently triggerable operation rather than carrying only one representative row.
- treat shared or generated wrappers as reachability evidence during validation. Do not let a proved wrapper path replace the child sink/control/protected-action rows that determine whether each concrete instance survives.
- after a repeated high-impact pattern has one strong proof tuple, stop deepening that one proof unless extra runtime work would materially change reportability, severity, or confidence. Spend the next validation effort on sibling ledger rows and suppress only with exact per-instance counterevidence.
- when a sibling is safe, suppress that exact instance with the specific control that makes it safe
- keep a table of `candidate id`, `file:line`, `family`, `validation method`, `closest control`, `survives`, and `confidence`
- prefer exact source/sink line evidence over broad prose, because downstream validation, attack-path analysis, and final report assembly depend on precise affected locations
- for wrapper-to-shared-sink findings, validate reachability through the wrapper while preserving the underlying parser, deserializer, path/archive, expression, or auth/authz sink/control line as an affected location when that line implements the broken security behavior
- for parser, deserializer, and object-construction findings, validate the concrete codec, converter, deserializer, resolver, or container handler that performs recursive parsing, type resolution, conversion, class filtering, or object construction. A broad parser/config proof does not close a concrete registered handler unless the handler's own root-control line is included or exactly suppressed.
- for file-format object-model helper rows, first decide whether malformed or adversarial input plausibly reaches the helper and whether the missing type, size, shape, recursion, numeric, or conversion guard creates crash, denial-of-service, parser-confusion, authorization-bypass, or other concrete security impact. If those pieces are absent, suppress or defer the row with exact evidence; if present, preserve the helper line as the root control even when an edge parser/filter finding also survives.
- when equivalent class filters, allowlists, denylists, blacklists, whitelists, or resolver controls are duplicated across runtime packages, validate or suppress the runtime/exported implementation separately. A transport callsite or nested helper proves reachability, but the reusable resolver line remains affected when it implements the broken security behavior.
- if validation shows one route or helper has multiple distinct high-impact proof tuples, keep them separately addressable instead of replacing one with another. A proved command-injection path does not validate away a separate SSRF, XML parser, path/file, XSS/template, or authz/state-change path in the same flow.
- when suppressing XML/parser/deserializer candidates, name the exact complete control that defeats that parser path. Partial hardening or a safe sibling parser does not suppress a different default factory, converter, validator, transformer, unmarshal, or parse call.
- for XML parser/converter candidates, verify hardening fails closed on the exact parser instance. `FEATURE_SECURE_PROCESSING` by itself, swallowed/logged `setFeature` failures, caller-supplied parser factories/readers, or missing entity/DTD resolver controls leave the row alive unless exact runtime evidence proves external entities and DTDs cannot be processed.
- for command/action runner candidates, validate each argument type and execution mode independently. Treat API/webhook-supplied values as attacker-controlled even when the frontend widget would normally constrain them, and do not suppress shell injection until the exact typecheck/escape path for that argument type is proven safe before template rendering and shell wrapping.
- for SSRF/download/webhook/callback candidates, validate the exact destination source, URL parser, allow/deny/filter config, redirect behavior, and network client sink. Do not suppress because the outbound request is a documented feature, because operators can configure filters, because filters are empty by default, or because validation happens before redirects; those facts are preconditions or partial controls unless the final requested destination is constrained on the exact path.
- for resource-serving/path traversal candidates, validate the exact allowlist, path matcher, URL decoding, canonicalization, and resource-selection control used by that handler. A newer safe resource handler or resolver does not suppress a legacy/deprecated/exported handler with a different control line. For restore/import/export/admin-looking routes, also validate the exact global middleware and decorator semantics before assuming the path is authenticated; optional or conditional login wrappers are not enough when the route is reachable without auth in default or no-password deployments.
- for restore/import/export, backup/restore, archive extraction, file copy/move, download/open, and key/config fetch candidates, validate the exact destination-selection, canonicalization, branch-local transform, and filesystem effect for that operation. A nearby auth bypass, secret leak, or configuration flaw is a separate row, not suppression or replacement for the path/file row.
- for archive extraction and restore/import candidates, suppression requires exact proof that each untrusted member path is normalized and contained before the extraction or write occurs. Do not suppress with a generic claim that a library helper is safe, or with later top-level file filtering, UUID/manifest gates, copytree/import allowlists, recursive promotion into an approved root, or post-extraction scanning after attacker-controlled paths have already been materialized. Reason explicitly about symlink, hardlink, metadata, and imported-subtree behavior whenever extracted content is later copied or promoted. Do not require the write to escape the overall app/datastore root; writes into trusted config, peer-object directories, shared roots, or imported subtrees inside that root still count as arbitrary file impact.
- for archive-member traversal, static evidence can be enough to survive when uploaded package/archive bytes reach a member-name decode/filter/join/write sequence and no exact containment check runs before the write. Missing optional parser libraries or lack of a full archive harness should lower confidence or become an explicit deferred proof gap, not silently suppress or replace the row with an adjacent same-family file traversal.
- deprecation, opt-in registration, or documentation warning that an API can be dangerous is a precondition, not counterevidence, for framework/library runtime code when the instance has a plausible cross-boundary source and runtime/deployment path. Suppress only if repository evidence proves the intended restricted/control mode defeats the exact attack; do not suppress a bypass of the restricted mode because an unrestricted mode is documented as dangerous.
- when suppressing auth/authz candidates, name the exact permission, authentication, tenant/object, or state-transition check on that endpoint. A credential-helper issue elsewhere does not replace a public webhook/status/API endpoint that reads protected data or triggers protected work.
- for stateful authentication protocols, validate the transition from pre-authentication or pre-upgrade state to credentialed identity: principal/credential/token installation, rebind or reauthentication call, issuer/callback assignment, and validated object versus consumed object. Missing rebinding or incomplete state checks remain reportable when attacker-controlled protocol state can authenticate or bind the wrong identity.
- for self-service update candidates, compare the attacker-controlled request object and persisted object field by field for security-sensitive identity, trust-state, tenant, role/group, MFA, and account-recovery properties. Do not suppress because one alias is checked, such as primary email, when a related scalar or collection alias can still be changed.
- when the provided candidate set is repository-wide, validate high-impact candidates first and spend validation effort in this order: command/code execution, unsafe deserialization, SSTI/template execution, SQL/NoSQL/query injection, SSRF/callback/file/network impact, path traversal/arbitrary file read or write, unsafe upload, and authz/tenant/object bypass with privilege or protected-object impact
- do not let low-severity data/config findings consume validation budget before the high-impact queue is exhausted
- do not let one difficult build or service setup consume the budget needed to validate sibling high-impact candidates from the coverage ledger. If setup becomes disproportionate, switch to code trace plus existing tests/config evidence for that candidate and continue the ledger.
- suppressions must close the specific row they suppress. A missing downstream caller, deployment fact, import path, or artifact-provenance fact is a reason to run the bounded adjacency pass or mark the row `deferred`, not proof that the candidate is safe.

Use class-specific proof tuples:

- authz/tenant/object/state change: attacker path + missing/wrong guard + protected object/comparison/state transition
- injection/path traversal/file upload/header/open redirect: attacker-controlled bytes + sanitizer/canonicalization/allowlist result + dangerous sink/context
- XSS/template/SSTI: attacker-controlled value + escaping/template context + browser/server-side template execution sink
- recursive placeholder/template injection: request, tenant/client metadata, stored configuration, or error value + placeholder/template helper that recursively expands, re-parses, or evaluates resolved values + missing escape/non-recursive guard + XSS, expression execution, credential exfiltration, or code execution impact
- deserialization/code execution: attacker-controlled serialized/code/template bytes + unsafe loader/evaluator + execution or object-construction effect
- deserializer wrapper denylist/allowlist control: attacker-controlled, stored, plugin, remoting, import, or persisted-state serialized input + shared wrapper that accepts type tags or default object construction + missing/misordered deny entry, allowlist gap, converter-priority gap, or unsafe class-loader/default-converter behavior + object construction, crash, code execution, or privilege-boundary impact
- concrete deserializer/codec control: attacker-controlled serialized or structured input + registered codec/converter/deserializer/container handler that recursively parses, resolves types, filters classes, converts values, or constructs objects + missing validation, unsafe fallback, fail-open filter, or unbounded traversal + code execution, object construction, parser confusion, denial of service, or privilege-boundary impact
- SSRF/callback: attacker-controlled destination + destination control bypass + network/read/side-effect impact
- SSRF optional-filter/redirect control: attacker-controlled download/webhook/callback URL + optional, empty-by-default, regex-only, pre-request-only, or redirect-following destination control + internal/LAN/cloud metadata/file-backed fetch or server-side callback side effect
- auth/token/assertion/protocol control: attacker-controlled token, assertion, protocol metadata, or version value + exact validator/control semantics + mismatch between validated value and trusted value, incomplete canonicalization/equality, unchecked parsing, or missing binding + authentication, authorization, or protocol-security impact
- stateful auth protocol transition: attacker-controlled credentials, principal, token, issuer, assertion, server response, or protocol metadata + state transition after TLS upgrade, bind, redirect, callback, assertion validation, or identity-provider response + missing rebind/reauthentication, stale identity reuse, incomplete issuer/callback binding, or validated-vs-consumed mismatch + authentication bypass or identity confusion impact
- SAML/XML assertion binding: attacker-controlled response or assertion set + protocol/signature validation of one object + later use, clone, serialization, or storage of a different assertion/document node + authentication/session/token impact. Multi-object preconditions should be stated, but suppression needs exact evidence that the same object is cryptographically and semantically bound to the consumed object.
- SSO/SAML response validator: attacker-controlled SSO response containing one or more assertions + response/assertion validator code that selects, indexes, clones, serializes, or returns an assertion + mismatch between the signed/validated assertion and the assertion later consumed by the session/token path, or missing recipient/audience/destination/ACS binding + authentication or authorization bypass impact. A generic claims-authorizer or service-method authorization finding does not validate or suppress this row.
- found-valid selection mismatch: attacker-controlled list or set of tokens/assertions/identities + validator loop or `foundValid*` flag proves one element while later fixed-index, first/last, clone, serialization, or lookup consumes another element + authentication, authorization, or protocol-state impact. Suppression needs evidence that the consumed object is the same object already validated.
- XML parser/converter hardening: attacker-controlled XML/SVG/XSLT/SAX/DOM/StAX input + parser factory, converter, transformer, or resolver setup + fail-open feature configuration, missing entity/DTD controls, caller-supplied parser path, or secure-processing-only hardening + XXE, SSRF, file read, parser injection, or denial-of-service impact
- query/parser injection: attacker-controlled bytes + query/selector/parser API that receives syntax or operators rather than bound values + semantic change, parser error, row-set change, write amplification, or bypassable post-query guard + read, write, authz, integrity, or availability impact. A later business check limits confidence or impact only after proving it checks the same trusted object and defeats syntax control for that exact instance.
- resource handler path control: attacker-controlled URL/path/resource name + allowlist/path-matcher/decoder/canonicalizer/resource-selection control + mismatch, pre-decode/post-decode gap, legacy handler behavior, or unsafe resolver fallback + arbitrary file read/write, path traversal, or unauthorized resource access impact
- shared deserialization control: attacker-controlled or privilege-bearing serialized/config/import/plugin/remoting input + shared loader or converter/allowlist/denylist behavior + unsafe object construction or incomplete control + affected callsites or unresolved cross-boundary reuse
- protocol/version parser: attacker-controlled protocol metadata + missing complete format enforcement before split/parse/compare + parse exception, wrong ordering, or feature-gate bypass + protocol-security impact
- protocol/version regression seed: CVE/advisory/security-test context points at protocol compatibility, version comparison, negotiation, or feature gating + checked-out utility/comparator parses attacker-controlled protocol metadata with `split`, numeric conversion, regex partial validation, or unchecked component access + malformed or adversarial metadata can crash negotiation, bypass a protocol gate, select the wrong feature path, or downgrade/disable a security-relevant behavior. Missing runtime harness is a confidence limit when the local parser/control/impact tuple is visible.
- file-format object model DoS/corruption: untrusted document/archive/message parsed into first-party object model + low-level array/dictionary/node/helper method that performs unchecked element conversion, recursion, numeric parsing, or unbounded iteration without validating attacker-controlled structure + crash, denial of service, parser confusion, or security-control bypass impact
- file-format primitive helper: untrusted PDF/XML/YAML/archive/message/image/font/protocol structure + helper such as `to*Array`, `toList`, `getObject`, numeric conversion, parser/iterator, size-based allocation, unchecked cast, collection-to-array conversion, or loop over attacker-controlled nodes + missing type/size/shape validation + crash, denial of service, parser confusion, or security-control bypass impact. Central object-model helpers should be validated even when an edge parser/filter finding already survives.
- advisory-seeded parser/file-format DoS: advisory, release note, fix hunk, or security test identifies a malformed-input crash or resource-exhaustion regression + checked-out source shows untrusted file/message parsing reaches the exact helper + the helper performs unchecked cast, size-based allocation, recursive traversal, numeric conversion, or loop over attacker-controlled structure. A runtime harness improves confidence, but checked-out code plus existing tests or deterministic code reasoning can be enough to mark the row `reportable` when no exact countercontrol exists.
- deterministic parser/helper availability: untrusted remote, protocol, document, archive, or package input + missing helper guard produces a deterministic exception, unchecked cast, unchecked numeric parse, recursion, allocation, or loop failure + repeated trigger can abort request processing, parser execution, security negotiation, or service availability. Treat as security-relevant unless exact recovery or equivalent prevalidation defeats this instance.
- branch-specific operation control: request-selected operation or fallback branch + branch-local split/filter/canonicalize/type-resolution/object-binding line transforms attacker-controlled path/value differently from the shared path + shared evaluator, binder, or security-sensitive mutation sink. Validate the branch line, not only the common helper.
- self-service object/profile update authz: authenticated or externally controlled identity + update guard over protected profile/account/tenant fields + missing immutable-field, collection-alias, or subject/object binding check + account takeover, identity confusion, privilege escalation, or protected-object mutation impact
- secret/data exposure/session config: secret or sensitive source + exposure/storage/log/client boundary + missing protection; validate after high-impact classes unless this directly enables code execution, injection, privilege escalation, auth bypass, or sensitive cross-boundary impact
- agent/MCP: untrusted instruction/data source + privileged tool/action boundary + action, code execution, or exfiltration effect

## Validation Checklist

Use this checklist to keep validation close to the prompt contract:

- Build the rubric before validating, using up to five concrete criteria grounded in the candidate and the surrounding code.
- Include a realistic-interface criterion when the code exposes an HTTP, CLI, message, file, or other user-reachable interface.
- Prefer precise, bounded steps over broad scans.
- For non-compiled stacks, prefer minimal targeted code understanding and only the smallest set of files needed.
- For compiled stacks, prefer stronger evidence in this order when feasible:
  - crash
  - valgrind or ASan
  - debugger trace
  - focused unit or integration test
  - realistic interface reproduction
  - code understanding
- If the code exposes a realistic interface, attempt validation through that interface before concluding when feasible.
- Keep commands short, non-interactive, and scoped to the touched files or the minimum referenced paths.
- If validation fails, record what was attempted, why it was inconclusive, and what proof gap remains.
- Save any PoCs, logs, or crafted inputs under that finding's validation artifacts path from `../../../references/scan-artifacts.md`.
- When multiple instances are provided, keep each candidate individually marked as survived, suppressed, or uncertain; do not silently omit candidates from the final validation report.
- For a single standalone validation request, do not infer repository-wide or sibling scope unless the user explicitly asks for expansion or provides a multi-instance candidate list.
- For a top-level repository-wide security scan, do not narrow validation to one representative finding when discovery supplied a coverage ledger or repeated pattern family.
- For repository-wide candidate sets, do not drop low-severity but real instances solely because they are low severity, but validate and report them only after the high-impact queue unless they directly amplify a serious issue.
- Use nearby safe paths as negative controls when feasible, but do not let the existence of a safe sibling suppress vulnerable siblings.
- Along with the PoC and artifacts, include a small readme explaining how to rebuild or use the PoC to test against the real target.
- If ASan, valgrind, debugger, or other logs prove the vulnerability with high certainty, include them as validation artifacts.

## Confidence Guidance

Calibrate confidence from the strongest evidence actually obtained, not the scariness of the bug class.

- `1.0` for a reproduced crashing PoC with a successful validation result
- `0.9+` for valgrind or ASan reproduction with a successful validation result
- `0.8+` for a debugger trace that successfully demonstrates the vulnerability path
- `0.3+` for code understanding with a defensible success or failure conclusion
- `0.0` when counterevidence clearly defeats the suspected vulnerability
