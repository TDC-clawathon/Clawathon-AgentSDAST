# security-scan (deepscan methodology)


# Security Scan

Used when a user wants to audit an entire repository or a user-specified path, package, folder, or submodule-like scope for security vulnerabilities. Keep the scan phases separate and produce final HTML and markdown reports.

## Phase Sequence

Keep these phases distinct and run them in linear order:

1. `$threat-model`
2. `$finding-discovery`
3. `$validation`
4. `$attack-path-analysis`
5. Generate final output

Treat this skill as the top-level orchestrator for the four skills plus the final report assembly step. Do not collapse the phases together.

For each phase:
1. Read that phase's skill.
2. Load only the inputs required for that phase.
3. Complete that phase's workflow and checklist.
4. Only then read the next phase's skill.

Do not read ahead into later-phase skills until the current phase has completed.
Do not amortize effort across phases: complete each phase to the full depth expected by that phase before moving on.
If user requests a repo-side scan or a scoped scan, stop and ask for authorization for subagents now before setting up the goal.

## Goal Setup

Before substantive scan work, create a Codex goal for the scan if the runtime exposes goal tools and no active goal already covers this scan. The objective should state that the scan must not stop until the resolved files in scope have been covered and the required coverage artifacts prove that closure.

Use objective wording shaped like:

`Run the Codex Security repository/scoped-path scan for <resolved target>; do not stop until every in-scope file/worklist row has a completion receipt or explicit deferred closure, every candidate has required ledger receipts, and the final report is written.`

If a compatible active goal already exists, continue under it instead of creating a duplicate. If goal tools are unavailable, state the same coverage objective in the first visible scan update and continue.

Do not mark the goal complete until:

- every file or worklist row in the resolved scope has a completion receipt, or an explicit `deferred`, `not_applicable`, or `suppressed` closure with exact reason
- every candidate that reached discovery has the required discovery, validation, and attack-path ledger receipts, or an explicit deferred reason for the missing proof
- the final markdown and HTML reports have been written to the resolved scan paths

## Artifact Resolution

The path references in this skill are the default locations for this phase.
If the user explicitly provides a different path for a required input or output, use the user-provided path instead of the corresponding default path referenced in this skill.
If a required input is still missing, stop and ask the user for it before continuing.
Use the shared scan artifact path conventions in `../../references/scan-artifacts.md`.

## Execution Plan

Follow this plan in order. Do not skip ahead to a later phase until the current phase has produced its intended output.

1. Resolve the scan target, `repo_name`, `security_scans_dir`, `scan_id`, `scan_dir`, and `artifacts_dir` using `../../references/scan-artifacts.md`.
2. Create or adopt the scan goal described in `Goal Setup`.
3. Run `$threat-model` first.
  - Copy the repository-scoped threat model to the per-scan threat model path without alteration for auditability.
  - Treat the per-scan threat model path as the source of truth threat model for later phases.
4. Run `$finding-discovery` as the second step, against the resolved repository or scoped path and using the per-scan threat model as context.
  - Stop at discovery only when the ranked runtime-surface worklist exists and the coverage ledger has closed every applicable high-impact and seeded root-control row as `suppressed`, `not_applicable`, or `deferred` with exact reasons. Open, reportable, or unresolved seeded rows continue to validation even when they are not yet numbered as findings.
5. Run `$validation` as the third step, for each candidate that came out of discovery and each open, reportable, or deferred seeded/root-control ledger row that still needs closure.
  - Pass the resolved scan scope, discovery notes, and candidate inventory to validation. Validation should preserve or suppress the provided instances; it should not independently broaden or narrow the requested repository or scoped-path scan.
  - Each candidate finding's candidate-ledger path from `../../references/scan-artifacts.md` is part of the validation input for every scan scope. Every candidate finding that came out of discovery must have a discovery receipt before validation starts and a validation receipt before the scan can proceed to final reporting.
  - For repository-wide and scoped-path scans, the discovery worklists, work ledger, raw candidates, per-finding candidate ledgers, deduped candidates, and discovery coverage ledger from `../../references/scan-artifacts.md` are part of the validation input; the ledger is a coverage artifact, not just a findings tracker. Raw candidates should already include the discovering file-review subagent's or parent agent's candidate-local validation evidence and attack-path facts before dedupe, and each per-finding candidate ledger should prove that its raw candidate finding received both checks or has an explicit deferred reason. Validation should preserve checked surfaces with not_applicable, suppressed, deferred, and reportable dispositions, reconcile cross-file proof gaps, and continue the ledger's high-impact sibling checks when needed rather than narrowing to one representative finding.
  - When multiple candidates or coverage-ledger rows need validation and subagents are available and approved, divide validation across validation subagents by candidate, deduped candidate, or ledger row. Each validation subagent must receive the candidate or row, discovery evidence, artifact paths, and candidate-ledger path it owns, then write or return the validation report update and validation receipt for that assignment.
  - As coverage-ledger rows are validated, keep the saved per-finding validation reports current enough that reportable, suppressed, not_applicable, and deferred closure rows survive interruption or later phase summarization, including exact root-control file:line and seed-anchor file:line when distinct.
6. Run `$attack-path-analysis` as the fourth step, for findings and validation closure rows that still need reportability, attack-path, and severity analysis after validation.
  - Each candidate finding's candidate-ledger path from `../../references/scan-artifacts.md` is part of the attack-path input for every scan scope. Every candidate finding that reaches attack-path analysis must have an attack-path receipt before final reporting, even when the final decision is `ignore`, suppressed, or deferred.
  - When multiple validated candidates or validation closure rows need attack-path analysis and subagents are available and approved, divide attack-path work across attack-path subagents by candidate or row. Each attack-path subagent must receive the validation evidence, affected root-control and sink lines, artifact paths, and candidate-ledger path it owns, then write or return attack-path facts, severity/policy analysis, and the attack-path receipt for that assignment.
7. Assemble the final output last using `../../references/final-report.md` and the outputs of the earlier phases: finding discovery plus each candidate finding's validation and attack-path reports.

## Scan Scope

- Phase 1 (threat model generation) is repository-scope by default, unless the user explicitly asks for narrower scope or provides an authoritative threat model or sufficiently repository-specific security scan guidance such as `AGENTS.md`.
- Phase 2 onward (finding discovery, validation, attack path analysis) remain within the resolved repository or scoped path. For repository-wide scans, the entire checked-out repository is in scope. For scoped-path scans, the requested path, package, folder, or submodule-like boundary is in scope together with directly supporting files needed to understand concrete findings.
- Before the `$finding-discovery` phase, read `references/repository-wide-scan.md` and every required reference it lists, then use them for finding discovery, validation, and attack path analysis.

## Scan Target

Resolve the requested audit scope before starting:

- repository-wide: scan the entire checked-out repository
- scoped path: scan the user-specified path, package, folder, or submodule-like boundary inside the checked-out repository

Treat the resolved repository or scoped path as the in-scope codebase for the later phases of this workflow.

## Scoped Exhaustive Mode

For repository-wide and scoped-path scans, follow `references/repository-wide-scan.md` and every required reference it lists.

If the user doesn't explicitly authorize subagents, stop and ask for the permission because the use of subagents is vital to the performance of exhaustive repository or scoped-path scans. If you are pursuing a goal and authorization for subagents is needed, block the goal and ask for authorization first, or the scan will not work.

Use the per-scan artifact directory layout from `../../references/scan-artifacts.md`.

## Final Output

Assemble the final markdown report, final HTML report, and Codex app review directives using `../../references/final-report.md`.

## Hard Rules

Read `../../references/shared-hard-rules.md` before applying scan-mode-specific hard rules.

- Create or adopt the scan goal before substantive scan work, and do not complete it until the resolved in-scope files/worklist rows, candidate ledgers, and final reports meet the `Goal Setup` closure criteria.
- For repository-wide and scoped-path scans, do not equate broad sink counts with completed coverage. The coverage ledger must close each applicable high-impact shard row as `reportable`, `suppressed`, `not_applicable`, or `deferred`.
- For every scan scope, candidate-finding coverage is required. Do not finalize a candidate finding until its candidate-ledger path from `../../references/scan-artifacts.md` shows discovery, validation, and attack-path receipts for that exact candidate, or an explicit deferred reason for the missing proof.
- For repository-wide and scoped-path scans, subagent dispatch must have explicit ownership: ranking subagents own one `rank_input.csv` row and return only ranking JSON; file-review subagents own one assessed file or tiny shard and return full-file receipts plus pre-dedupe finding objects with candidate-local validation evidence and attack-path facts; validation subagents own one candidate or ledger row that needs validation closure; attack-path subagents own one validated candidate or validation closure row; the parent agent owns orchestration, ledger reconciliation, aggregation, cross-file dedupe, and final closure.
- For repository-wide and scoped-path scans, candidate-finding coverage is separate from file coverage. Do not dedupe or finalize a raw candidate finding until its candidate-ledger path from `../../references/scan-artifacts.md` shows candidate-local validation and candidate-local attack-path receipts, or an explicit deferred reason for missing proof.
- Candidate ids are optional links from coverage rows to findings; a not_applicable, suppressed, or deferred row is still required when the surface was in scope.
- For repository-wide and scoped-path scans, the ranked runtime-surface worklist must exist before discovery is considered complete, and the coverage ledger must be materially broader than the promoted candidate list.
- For repository-wide and scoped-path scans with CVE, GHSA, advisory, issue, release, or package-version identifiers, `seed_research.md` must exist before discovery is considered complete. It should record authoritative sources searched, candidate files/functions/classes/hunks, and failed lookup attempts. Missing seed research means advisory-led discovery is incomplete unless the scan explicitly states that no network/local-history source was available.
- In large repository-wide scans, checkpoint the ranked runtime-surface worklist and initial coverage ledger to disk before deep sink review or validation. A run that is interrupted after frontier mapping should still leave auditable coverage artifacts.
- In large monorepos, top product/runtime areas by file count or deployment significance must appear as ledger shards or be explicitly excluded with repository evidence; global sink counts and `no top candidate surfaced` do not close coverage.
- User/advisory/tag-seeded packages, class families, or vulnerability families remain open until the exact seeded row is closed as `reportable`, `suppressed`, `not_applicable`, or `deferred`. A neighboring same-family finding does not close the seeded row.
- For large repository-wide scans, make one reachability pass across every applicable high-impact shard before prolonged validation of any single shard. A row becomes a validation candidate only when it has a concrete entrypoint or privileged boundary, closest relevant control, sink or broken control, and plausible impact.
- Discovery is incomplete when a shard has a promoted finding but still has unclosed sibling packages, concrete implementations, or reusable root-control rows that could be independently vulnerable. Finish those rows or mark them explicitly deferred before final reporting.
- Final assembly must start from reportable validation closure rows and surviving candidates. Do not drop a reportable seeded/root-control row because attack-path analysis or discovery spent more prose on a neighboring same-family finding.
- Final reporting is incomplete when a promoted high-impact finding's affected lines omit the concrete root-control file/line discovered or seeded during discovery, such as a codec, converter, parser feature setup, class filter, resource-path control, protocol state transition, or self-service update guard. Add the root-control affected line or explicitly suppress/defer it with exact counterevidence before finalizing.
- In repository-wide and scoped-path scans, preserve independently reachable sibling instances through final reporting. Repeated vulnerable templates, query builders, parser operations, auth/object endpoints, or shared-helper callers need separate finding entries, affected lines, and dispositions; put grouping in summary prose only after the individual instances are emitted.
- For query/parser injection, do not suppress syntax-control evidence solely because a later business check appears to limit impact. Carry the injection candidate until validation proves the exact query API and post-query guard defeat semantic change for that instance.
- If large-repository scope forces deferral, make the final report explicit about which deployed or privileged areas and vulnerability families remain deferred.

---
### reference: repo-wide-artifacts-and-ledger

# Exhaustive Scan Artifacts And Ledger

Use this reference with `repository-wide-scan.md` for exhaustive repository or scoped-path ranking and coverage-ledger rules. Read `scan-artifacts-and-ledger.md` first for shared artifact, seed, subagent, scoped file-review, candidate-ledger, and dedupe rules.

## Exhaustive Scan Artifact Requirements

- Use the artifact paths from `../../../references/scan-artifacts.md` for `rank_input.csv`, `rank_output.csv` when ranking applies, `deep_review_input.csv`, `work_ledger.jsonl`, `raw_candidates.jsonl`, `dedupe_report.md`, `deduped_candidates.jsonl`, and `repository_coverage_ledger.md`.

## Exhaustive Scan Subagent Ownership

- Ranking-subagent ownership: one ranking subagent owns exactly one `rank_input.csv` row and returns only ranking JSON.
- Parent-agent ownership: the parent agent owns `rank_input.csv` generation when an upstream orchestrator did not already provide it, ranking-subagent dispatch when ranking is needed, `deep_review_input.csv` selection when an upstream orchestrator did not already provide it, global frontier/coverage work, and final exhaustive-scan closure.

## Files In Scope

- A parent orchestrator may provide authoritative in-scope worklists at the standard `<discovery_dir>/rank_input.csv` and `<discovery_dir>/deep_review_input.csv` paths before this workflow begins.
  - Treat the parent-provided worklists as authoritative only when the current scan instructions explicitly say they are authoritative and both files are present. A stale or partial artifact pair is not a valid scope contract.
  - When authoritative parent-provided worklists are present, use them exactly as supplied. Do not regenerate `rank_input.csv`, rerun ranking, overwrite `deep_review_input.csv`, or reinterpret `top-percent` inside this scan.
  - The parent orchestrator owns explaining whether its `deep_review_input.csv` is exhaustive or selected. This exhaustive workflow still owns full review receipts, candidate ledgers, coverage-ledger closure, and final closure for the supplied worklist.
- Otherwise, create a deterministic in-scope file worklist before subagent dispatch. Use `<plugin_dir>/scripts/generate_rank_input.py` to create `rank_input.csv`; do not ask the model to invent the file inventory.
  - Command shape: `python3 <plugin_dir>/scripts/generate_rank_input.py make-repo-rank-input --repo <repo_root> --scope <scope> --out <discovery_dir>/rank_input.csv`.
  - The generated CSV is the canonical candidate list for ranking subagents. It contains `path`, `area`, and `preview`.
  - The script only includes source-like text files and default-excludes tests, docs, examples, personal/dev-only trees, vendored trees, generated caches, and build artifacts unless the threat model explicitly makes one of those areas runtime-reachable or privilege-bearing.
  - If excluded content is added back manually, record the reason in the coverage ledger.
  - The Python script does not make the security ranking decision. Ranking is performed by ranking subagents over `rank_input.csv`.
- When authoritative parent-provided worklists are not present, convert the candidate list into the deep-review worklist:
  - Interpret `top-percent` as the percentage of ranked, included files that receive deep full-file audit.
  - If `top-percent` is below 100, dispatch ranking subagents with `spawn_agents_on_csv` on `<discovery_dir>/rank_input.csv`.
    - Ranking-subagent ownership: one ranking subagent owns exactly one `rank_input.csv` row. It decides only whether that file should enter deep review, and returns JSON with `path`, `include` true/false, `score` 1-10, and `reason`.
    - Ranking subagents do not perform deep review, validation, attack-path analysis, dedupe, or ledger closure.
  - Save ranking output to `<discovery_dir>/rank_output.csv`, then create `<discovery_dir>/deep_review_input.csv` with `select-deep-review-input`.
  - If `top-percent` is 100 or higher, skip ranking and copy every `rank_input.csv` row directly into `<discovery_dir>/deep_review_input.csv`.
  - Do not treat deterministic path order or broad grep hits as ranking evidence; the ranking-subagent output is the ranking source of truth.
- Deep-review every file selected into `deep_review_input.csv` using the shared scoped file-review rules in `scan-artifacts-and-ledger.md`.
- When `top-percent` is 100 or higher, or when an authoritative parent-provided worklist declares `deep_review_input.csv` exhaustive over `rank_input.csv`, do not stop until every `rank_input.csv` row has a completion receipt in the shared work ledger.

## Ranking Requirements

- Derive product and privileged surfaces from router declarations, OpenAPI or RPC metadata, public or anonymous endpoints, applied specs, ingress/service config, job/worker definitions, package exports, and privileged local or agent/tool surfaces before free-text sink search.
- Include HTTP, GraphQL, RPC, CLI, job, webhook, file-processing, message, template, package API, and agent/tool entrypoints; authn/authz/session middleware and decorators; database/query builders, ORM raw-query escapes, serializers/deserializers, shell/process/eval/template engines, filesystem APIs, network clients/fetchers, upload/download paths; first-party security/protocol namespaces such as SSO, SAML, OAuth, OIDC/JWT, LDAP, Kerberos, XML security, remoting, config import/export, protocol codecs, parser/converter registries, and version or feature gates.
- Rank files highly when they define, configure, or materially control those runtime/security surfaces; record the concrete surface in `reason`.
- Default-exclude tests, docs, examples, personal workspaces, lockfiles, vendored trees, generated caches, and one-off research tooling from the first pass unless repository evidence shows they are deployed, privilege-bearing, generated into shipped runtime code, or reachable from untrusted input. If excluded content is added back, record the reason in the ledger.

## Coverage Ledger

- Create a high-impact coverage ledger first across the vulnerability families most likely to produce serious bugs: command/code injection and RCE, SQL/NoSQL/LDAP/XPath/template injection, SSTI, unsafe deserialization, SSRF/callback abuse, path traversal/arbitrary file read or write, unsafe file upload, header injection/open redirect with credential or callback impact, and authz/tenant/object isolation bypasses that cross a meaningful privilege or data boundary.
- Build and save the ledger from the ranked runtime/security surfaces and deep-review evidence with one row per applicable boundary and serious vulnerability family before deep validation begins. The ledger must include: ledger row id, seed or root-control file:line when one is known, boundary, shard or area, files checked, applicable family, source or privileged boundary checked, sink/control checked, candidate ids when any were produced, disposition, evidence summary, prune reason or add-back trigger when applicable, and any deferred reason.
- Rows with no candidate are still required, seeded rows must close the exact seeded package/class family, and dominant runtime/product areas must have explicit rows or explicit repository-evidenced exclusions.
- For large repositories, partition the inventory into review shards by deployed or privileged area and vulnerability family before deep validation. A shard is a concrete boundary such as a service, router group, package API, parser family, job/worker family, deployment surface, CI/deploy path, or privileged local/agent tool surface.
- Shard by product module, package namespace, or protocol/security subsystem as well as by bug family. Do not let one reportable finding in a broad family close sibling modules such as separate SSO/SAML/OAuth/JWT, parser, protocol, config-import, or deserialization-wrapper packages.
- In a large monorepo, the coverage ledger must be materially broader than the promoted candidate list. If the ledger only contains candidate rows, only a handful of rows, or only global sink-count rows, the frontier pass is incomplete; add `not_applicable`, `suppressed`, or `deferred` rows for unresolved shards and families before continuing.
- The top product/runtime areas by tracked-file count or deployment significance must appear as shards in the ledger or be explicitly excluded with repository evidence. Global sink counts alone do not close coverage for a dominant area or family, and `no top candidate surfaced` is not a terminal disposition.
- Dominant ambiguous trees must be split by runtime, deployment, package, or privilege evidence before they can be left deferred. A single blob row such as "project", "server", "core", or "plugins" is incomplete unless it cites the concrete entrypoint/control files checked and explains why further subdivision is not possible from repository evidence.
- Treat broad sink searches as seed generation only. They do not count as coverage completion until the relevant files have been tied to an entrypoint or privileged boundary and the ledger row has a final disposition.
- Promote a seed into a reportable finding candidate only after it has a concrete source or privileged boundary, closest relevant guard/control, sink or broken control, and impact. Public or anonymous routes, upload/parser entrypoints, webhooks, build/job triggers, package APIs, and privileged internal workers count as first-class boundaries.

---
### reference: repo-wide-high-impact-families

# Repository-Wide High-Impact Families

Use this reference with `repository-wide-scan.md` for family-specific repository-wide security review.

## General Family Rules

- Run one frontier pass across every applicable high-impact shard before prolonged validation or build work on any single shard.
- Continue fan-out from a validated high-impact sink pattern to sibling routes, templates, handlers, models, and config files until that high-impact surface is exhausted.
- Do not merge distinct high-impact sink or impact families into one finding solely because they share a route, wrapper, or helper. For example, command execution, SSRF, path traversal/file write, XML parser behavior, and authorization bypass in the same flow should remain separate when they have distinct sinks, controls, or impacts.
- Treat data exposure, hardcoded secrets, weak session/cookie flags, CSRF, rate limits, and generic security configuration as secondary unless they directly enable code execution, injection, privilege escalation, meaningful auth bypass, or sensitive cross-boundary impact.
- For each suppressed or safe-looking nearby path, record exact counterevidence. Suppression must be per-instance, not per-family.

## Fan-Out Families

- RCE and injection: shell/process calls, Python/Ruby/JS eval or exec, dynamic imports, template execution, recursive placeholder/template expansion, SQL/NoSQL/LDAP/XPath queries, header injection, and expression-language sinks.
- For command/action runner RCE, enumerate argument type validators and execution modes as separate shards: UI widgets, webhook/API argument ingestion, type-safety maps, unsafe-type denylists, template rendering, shell wrapping, and direct exec. Do not suppress shell injection because one mode uses direct exec or because some unsafe types are blocked; nil/no-op validators for `password`, `checkbox`, `confirmation`, choice-like, or custom argument types remain root controls when their values can render into a shell command.
- Unsafe parsing and execution: pickle/yaml/xml/deserializer sinks, XXE, SSTI, template/source rendering, upload processors, and archive/file extraction.
- Server-side request and file impact: SSRF, callback fetches, path traversal, arbitrary file read/write/delete, unsafe file upload, open redirect with callback or credential impact.
- For SSRF and callback-fetch families, split `downloadFrom`, URL import, webhook, callback, preview/render, redirect-following, and metadata/cloud/LAN destination controls into separate shards. Intended outbound-fetch functionality, optional operator allow/deny lists, empty default filters, and pre-request-only checks are not suppression evidence; preserve the row until the exact destination parser, redirect policy, scheme/host/IP canonicalization, and final network sink are proven to enforce the boundary.
- Privilege and boundary impact: missing authentication, missing authorization, IDOR/BOLA, tenant/object isolation, token/assertion/federation validation, protocol/version gating, and mass assignment when they expose protected objects or privileged state changes.
- Platform/agent impact: reachable vulnerable dependencies, IaC/Kubernetes exposure/RBAC that enables privilege or data-boundary impact, MCP/agent untrusted content to privileged tool/action paths.

## Parser, Protocol, And File Format

- For parser, protocol, and file-format packages, inventory the operation/helper classes in that boundary package before broad keyword sweeps. Searches may seed the review, but the boundary package should be checked as a unit so a shared helper or operation class is not replaced by an adjacent caller-only finding.
- For each parser/helper, deserializer, auth/token/assertion, protocol/version, and polymorphic-operation shard, run targeted control-hazard searches inside the boundary package before finalizing the shard. Use repository-appropriate patterns; examples include subclass/override declarations, concrete codec/converter/input-handler/validator/filter/guard/resolver classes, `super.` calls, `split`, `parse`, `parseInt`, `matches`, `find`, `startsWith`, `equals`, `URL`, `URI`, `get(0)`, `cloneNode`, `registerConverter`, deny/allow lists, and validator/comparator method pairs.
- Every matching root-control line that is in a reachable boundary package must be closed in the ledger as reportable, suppressed with exact counterevidence, or deferred.
- For file-format object models such as PDF/COS, XML/DOM, YAML nodes, archive entries, protobuf/message fields, image/font tables, or binary protocol records, run a primitive-helper sweep over array, dictionary, node, token, numeric, and iterator helpers before closing the parser shard. Search method shapes such as `to*Array`, `get*`, `getObject`, `toFloat`, `toInt`, `parse*`, `iterator`, `size`, unchecked casts, and loops over attacker-controlled collection sizes; these helpers are root controls when they convert, cast, recurse, or allocate from untrusted document structure.
- When ranking or deep review identifies a central first-party object-model package for an untrusted file or message format, the coverage ledger must contain rows for that package's array, dictionary, node, collection, and primitive conversion helpers before the parser shard can close.
- Object-model helper sweeps create mandatory coverage rows first, not automatic findings. Promote a helper row only when malformed or adversarial input can plausibly reach the helper and the missing type, size, shape, recursion, numeric, or conversion guard can cause crash, denial of service, parser confusion, authorization bypass, or another concrete security impact.
- Deterministic malformed-input crashes are security-relevant when untrusted remote, protocol, document, archive, or package input reaches a missing parser/helper guard and can abort a service, request worker, parser pipeline, or security negotiation. Do not suppress these as generic robustness unless exact containment evidence shows caller-side recovery, equivalent prevalidation, or a non-security-only boundary for that instance.
- For advisory, release-note, or security-test seeded parser/file-format DoS rows, a bespoke runtime harness is not always required when checked-out code plus existing tests or deterministic code reasoning make the untrusted source, broken helper control, and impact directly evident. Record a missing harness as a confidence limit; otherwise keep the row deferred rather than substituting seed text for local proof.
- For XML/parser/deserializer review, enumerate each exposed parser factory, converter, validator, transformer, unmarshal, parse, and handler entrypoint separately. A hardened sibling parser is useful negative control, but it does not suppress a different default factory or converter path.
- XML parser hardening is complete only when the parser fails closed or every needed feature/resolver is enforced on the exact parser instance used by the sink. Swallowed, logged, or best-effort `setFeature` failures, custom caller-supplied parser factories/readers, and `FEATURE_SECURE_PROCESSING` without entity/DTD controls are not suppression evidence for XXE or XML parser injection.

## Polymorphic Operations And Deserialization

- For polymorphic handlers, operation classes, converters, filters, validators, or strategy objects, fan out across subclasses and overrides in the boundary package before finalizing. If an override or request-selected concrete operation transforms, validates, canonicalizes, selects, or reinterprets attacker input before a shared sink/control, keep that concrete implementation line addressable alongside the shared sink/control.
- For structured patch/edit/apply APIs such as JSON Patch, Graph Patch, document edits, or config mutations, enumerate concrete operations like add, remove, replace, move, copy, and test; operation-specific path transforms or special cases such as array append or wildcard selection are root controls when they feed the shared evaluator or object binder.
- When a concrete operation has a branch-specific path such as append, wildcard, fallback, copy/move `from`, default-value, or type-resolution handling, preserve the branch predicate and branch-local transform as root-control evidence if that branch bypasses, narrows, or feeds the shared validator differently from the common path.
- After finding a shared parser, deserializer, template, or auth control, run a boundary-package root-control sweep over concrete codecs, converters, input handlers, validators, guards, filters, allow/deny resolvers, class filters, and operation subclasses before closing the shard.
- For deserialization and object-construction review, identify the repository's serializer/deserializer wrapper, converter registry, allow/deny controls, and default loader behavior before scanning scattered callsites. Preserve the wrapper/control line when it implements the broken security behavior.
- For parser/deserializer registries, enumerate concrete codec, converter, deserializer, and container handlers registered for arrays, collections, maps, beans, enums, throwables, and generic object values. Array or container codecs are root controls when they recursively invoke parsing, type resolution, conversion, object construction, or unbounded traversal on attacker-controlled structures.
- Global serializer/deserializer wrappers and deny/allow controls are themselves high-impact control points when they protect multiple config, import, plugin, remoting, or persisted-state paths. Do not suppress a broken shared loader/control solely because one observed caller requires permission; validate whether the wrapper is reused from another attacker-controlled, privilege-bearing, or cross-boundary path, and carry unresolved wrapper risk forward with explicit confidence/preconditions.
- In deserializer wrapper initialization, treat missing or misordered deny/allow entries, converter priorities, default reflection converters, class-loader selection, and type-tag handling as root controls when the wrapper can construct attacker-named or stored cross-boundary types.
- Class-resolution controls such as class filters, allowlists, denylists, blacklists, whitelists, and resolver predicates are primary affected controls when any deserialization, remoting, plugin, import, or stored-state path reaches them. A transport finding may prove reachability, but it should not replace the shared resolver/control line.

## Query, Template, Resource, And Auth Controls

- For query builders and database APIs, treat attacker control over query syntax as the security boundary. Do not suppress SQL, NoSQL, LDAP, XPath, or similar injection solely because the endpoint already accepts some user-controlled data, because the immediate operation is an insert/update, or because a later business check appears to limit the visible effect.
- First test or reason through quote/comment/boolean/union/operator/multi-statement variants, parser errors, row-set changes, write amplification, and whether the later guard is checking the same trusted object. If input can alter query structure or selector semantics, keep the instance as a validation candidate with any limiting preconditions, and promote it to a reportable finding only if validation confirms semantic change or a bypassable post-query guard for that exact instance.
- For template and placeholder engines, trace both the static template string and values resolved into the template. Recursive placeholder expansion, expression evaluation of placeholder names, helper/parser setup that enables recursive expansion, or re-parsing of resolved model/client/error values is a candidate injection sink when those values can come from request parameters, tenant/client metadata, stored configuration, exception messages, or other cross-boundary state.
- In framework or library code, stored client/application/tenant metadata, identity-provider attributes, externally supplied error descriptions, and imported configuration are cross-boundary values when the component later renders, evaluates, binds, or authorizes with them and the instance has a plausible runtime path from an application, tenant, identity provider, import, or other boundary.
- For resource-serving and static-file handlers, treat allowlist, path-matcher, canonicalization, URL decoding, and resource-selection lines as root controls. Do not replace a vulnerable legacy/deprecated handler or package API with a newer safe sibling handler; close each deployed or exported resource handler separately when path traversal or arbitrary file access is in scope.
- For filesystem-impact families, also sweep restore/import/export paths, backup/restore helpers, archive extraction and archive entry selection, file copy/move helpers, key/config download helpers, and temp-file promotion paths. Keep branch-local decode, join, strip-prefix, extension, canonicalization, and destination-selection lines addressable for each exported operation.
- For file-manager, theme/builder/media, asset, plugin-marketplace, and export/download controllers, treat request-supplied filenames, asset paths, theme names, package names, and builder resource ids as independent file-impact shards. Search route clusters around `FileManager`, `Theme`, `Builder`, `Asset`, `Marketplace`, `Package`, `download`, `zip`, `extract`, `filename`, `path`, and `allow_forward_slash`; do not close a seeded file-manager traversal because a neighboring theme/export traversal, SSRF, or package-install finding was reported.
- Shared path or parameter sanitizers that allow slashes, raw filenames, decoded paths, or optional canonicalization are root controls. When one controller calls `sanitize_params(... allow_forward_slash: true)` or equivalent, enumerate sibling services/controllers using the same helper before suppressing path traversal in a different API namespace.
- For base-controller or shared helpers named like sanitize_params, sanitize_path, clean_path, or safe_path, a boolean such as allow_forward_slash, allow_absolute_path, allow_path, or preserve_slashes is itself a root path-control shard. If request parameters later select config, script, target, plugin, storage, import/export, or file-manager resources, report the helper definition plus representative callers as a path traversal or arbitrary file-access finding; do not suppress it merely because the helper name sounds safe or because other storage/authz findings in the same subsystem are louder.
- For archive extraction and restore/import flows, the root control is the member-path containment proof before each filesystem write. If untrusted archive member names reach `extract`, `extractall`, tar extraction, temp-file materialization, or equivalent writes before exact containment validation, keep the row open even when later code only imports selected top-level files, copies into an approved root, or gates on UUID/manifest checks. Do not suppress with generic claims about standard-library safety; exact closure must reason through per-entry containment and any symlink, hardlink, metadata, or recursive-copy behavior that could still materialize attacker-controlled content inside an imported subtree. Escape outside the overall app/datastore root is not required; writes into trusted config files, peer-object directories, shared roots, or imported subtrees inside that root are still high-impact file writes.
- Treat archive-member decode, strip-prefix, and replacement filters such as removing `../` as root controls, not as proof of safety. Absolute paths, drive-prefixed paths, encoded separators, symlink or hardlink entries, and parser-preserved member names need exact containment proof before the write; a static trace from upload/package parsing to member write can keep the row alive with calibrated confidence when optional parser runtime is unavailable.
- If the same subsystem has both a file-impact row and an auth, secret, or config row, preserve both shards. A louder auth bypass or data-exposure issue is not suppression evidence for the path/file row unless it closes the same operation and control.
- In framework or library scans, deprecated, opt-in, or documented-dangerous APIs are still in-scope runtime surfaces when the repository ships them for downstream applications and the instance has a plausible cross-boundary source and runtime/deployment path. Treat deprecation/docs as reachability or deployment preconditions, not suppression evidence.
- For auth, token, assertion, federation, protocol, and version-gating controls, inspect the validator/control package before suppressing nearby candidates. Treat object-binding mistakes, partial string/prefix checks, URL/URI or host canonicalization hazards, regex match-vs-find mistakes, numeric/version parsing without complete validation, and validated-object vs consumed-object mismatches as first-class high-impact control failures when they cross an authentication, authorization, protocol, or trust boundary. For route-level auth, verify the exact global middleware and route-local decorator semantics; login-named, admin-looking, or restore/import endpoints remain anonymous attack surfaces when the wrapper is optional or only enforces auth after password/token config is enabled.
- In Java code, treat `URL.equals` and URL host equality as suspicious in issuer/callback/host security checks until canonicalization behavior is proven safe; prefer exact string/URI comparison rules from the protocol.
- In SSO/SAML/federation packages, keep response/assertion validators, claims authorizers, and generic method-authorization interceptors as separate shards. A generic authz or claims finding does not close a SAML SSO response validator row.
- For response validators, inspect assertion selection, list indexing, cloned DOM nodes, `getDOM`, `cloneNode`, signed-object lookup, subject confirmation, recipient, audience, destination, ACS URL, and issuer binding; preserve the exact line that chooses or returns the consumed assertion.
- If validator code iterates through candidate assertions/tokens/identities and sets a `foundValid*` flag, then later consumes a fixed-index, first/last, cloned, serialized, or separately looked-up object, keep that later selection as a candidate even when the loop itself contains strong checks.
- For auth/authz review, enumerate public or anonymous webhook/status/API endpoints that read protected objects, trigger jobs/builds, or mutate protected state separately from nearby credential-helper or configuration findings. When session, API-key, token, tenant, or identity parsing lets attacker-controlled input choose or misbind the consumed identity/object, carry both the root parser/control line and the protected object endpoint, and label the instance as authz/BOLA/IDOR when it controls object access.
- Keep external login/account-binding, self-service profile or SCIM update guards, invitation/password-reset flows, token/session validators, and protected object authorization as separate auth shards.
- For stateful authentication protocols such as LDAP, Kerberos, SAML, OAuth/OIDC, TLS-upgraded binds, and delegated identity providers, inspect the sequence that transitions from unauthenticated or pre-upgrade state to credentialed identity. Treat credential/principal/token installation, bind/reauthentication calls, issuer assignment, and validated-vs-consumed object selection as root controls when a missing rebinding or incomplete state check can authenticate the wrong identity.
- For self-service user, account, tenant, profile, SCIM, or settings update endpoints, inspect the guard or expression method that authorizes the update as a root control. Search route-adjacent predicates and helpers named like self, update, profile, allowed, guard, or policy, then compare attacker-provided request-body objects against the persisted object. Check immutable or security-sensitive scalar and collection aliases such as primary email versus email list, username, verified, active/disabled, origin/provider, tenant/zone, roles/groups, MFA state, and identifiers.
- For protocol/version gates, look for paired validator and parser/comparator helpers. If the parser/comparator splits or parses attacker-controlled protocol metadata without invoking the complete validator or enforcing equivalent bounds, keep that line as the broken control and validate protocol-security impact before suppressing it as mere input hygiene.
- In protocol-heavy repositories, include low-level version, capability, feature, and negotiation utility classes in the ledger even when the obvious findings are in web upload, REST, or admin surfaces.

---
### reference: repo-wide-instance-expansion

# Repository-Wide Instance Expansion

Use this reference with `repository-wide-scan.md` to avoid representative-only repo-wide findings.

## Instance Awareness

Within the existing scan workflow, keep repository-wide scans instance-aware:

- Discovery should create one candidate per independently vulnerable source/sink/control instance.
- The file-review subagent or parent agent that discovers a candidate should validate and attack-path that candidate instance before it enters cross-file dedupe, then later validation should preserve or suppress each deduped instance independently.
- The final markdown report may add grouped summaries for readability, but only after each surviving instance has its own finding entry with affected location, source, broken control, sink, impact, and counterevidence.
- Include suppressed candidates in the phase artifacts with the exact file/line and counterevidence so false-positive controls remain auditable.

This mode improves recall while preserving precision: breadth comes from systematic enumeration, while false-positive control comes from per-instance proof tuples and exact suppression reasons.

## Child Instance Expansion

- When a broad ledger or candidate row names a whole operation family such as "all SQL trigger variants", "all deserialization variants", "all path traversal helpers", "all SSRF modes", "all generated framework adapters", or "all unauthenticated mutation endpoints", split it into child instances keyed by concrete exported function, route branch, sink statement, API mode, parser/deserializer variant, or protected action before cross-file dedupe, validation closure, and final reporting.
- If one root cause creates multiple vulnerable templates, routes, query builders, parser/deserializer variants, path/file helpers, auth/object endpoints, protected actions, shared-helper callers, or config entries, carry each affected file/line through the phases as its own instance unless the runtime path truly cannot be separated.
- If one route or helper contains multiple same-family sink/control lines, such as `execute`/`executemany`/`executescript`, `pickle.load`/`pickle.loads`/`yaml.load`/`yaml.load_all`, distinct file/path helper calls, insert/select/delete/update query builders, or unauthenticated create/delete/reset/admin/job actions, preserve each operation as a separate instance when attackers can trigger it independently.
- For repeated vulnerable patterns, keep each independently vulnerable file and sink/control line as its own finding entry through discovery, validation, and final reporting. Do not rely on one representative finding with many extra files when those files can be attacked independently.
- Do not stop after a representative example, but do not promote bare sink hits without reachability and control evidence.

## Wrapper And Root-Control Preservation

- Shared or generated wrappers are reachability evidence, not proof that child sink/control variants can be collapsed; the wrapper may be shared affected context, but independently reachable child operations still need separate disposition rows.
- When a high-impact instance flows through a wrapper into a shared parser, deserializer, path/archive helper, expression evaluator, or auth/authz control, carry both the wrapper and the underlying shared sink/control through later phases so the final finding identifies the root vulnerable line as well as reachable entrypoints.
- If the flaw is caused by an unsafe transformation or selection before the sink, record the split/parse/canonicalize/normalize/compare/regex/object-selection line as the broken control.
- If a reportable finding says "all operations", "every converter", or "the shared loader", the concrete classes that make that statement true must have their own ledger rows or affected locations.
- If class-resolution or parser controls are duplicated across core, server, client, remoting, plugin, import, or compatibility packages, close the runtime/exported implementations separately. Equivalent helper names or nested classes are sibling controls, not automatic suppression for a standalone shared resolver.

## Separate Proof Tuples

- Keep distinct high-impact proof tuples separately addressable even when they share a route, wrapper, or helper.
- Split command execution, SSRF, path/file impact, XML/parser behavior, XSS/template execution, and authz/state-change impact when the sink, closest control, or impact differs.
- In XSS/template/client-rendering surfaces, enumerate each independently vulnerable render context and file/line: HTML body, script block, event handler, URL/attribute, server-side template string, recursive placeholder expansion, expression evaluation, and client-side DOM sink are separate instances when they can be triggered separately.
- In auth/authz surfaces, enumerate public webhook, status, callback, and API endpoints that read protected objects, trigger builds/jobs, or mutate protected state independently from nearby credential-helper or configuration bugs. If an auth bypass also lets the attacker select another user's object or identity, preserve that BOLA/IDOR instance instead of only reporting the credential or parser helper.
- For self-service object-update endpoints, inspect the request-body authorization guard against persisted-object fields and collections; close that guard separately from login, token, or protected-object endpoint findings.

---
### reference: repo-wide-validation-closure

# Repository-Wide Validation Closure

Use this reference with `repository-wide-scan.md` to preserve repo-wide coverage through candidate-local validation, candidate-local attack-path analysis, post-dedupe reconciliation, and final reporting.

## Closure Dispositions

- Every applicable high-impact ledger row must finish as `reportable`, `suppressed`, `not_applicable`, or `deferred`. Do not claim full repository coverage while any applicable high-impact row remains `deferred`; instead state exactly what remains deferred and why.
- User/advisory/tag-seeded root-control rows remain validation input even when reachability is incomplete; validation must close them as `reportable`, `suppressed`, `not_applicable`, or `deferred` with the exact proof gap.
- Do not suppress an exact seed row only because a neighboring issue looks more dramatic or because full downstream deployment details are outside the repository; state those details as preconditions or proof gaps.
- Each in-scope row must be recorded even when no candidate is found. Candidate ids are optional links from coverage rows to findings, not the reason the row exists.
- `open_seed` is an interim disposition only; before final reporting, every row must become `reportable`, `suppressed`, `not_applicable`, or `deferred` with exact file:line evidence or a concrete proof-gap reason.

## Pre-Dedupe Candidate Requirements

- Raw repository-wide candidates should already carry the discovering file-review subagent's or parent agent's candidate-local validation evidence and attack-path facts before dedupe.
- Each raw candidate finding's candidate-ledger path from `../../../references/scan-artifacts.md` is its candidate-finding coverage artifact. Every raw candidate finding must have a stable candidate id plus candidate-ledger receipts for candidate-local validation and candidate-local attack-path analysis, or an explicit deferred reason for missing proof, before dedupe or final reporting.
- Post-dedupe validation is for reconciliation, unresolved proof gaps, cross-file confirmation, and final closure; do not let dedupe erase the earlier proof tuple or its candidate-ledger traceability.

## Validation Budget And Coverage

- Before finalizing a high-impact finding in a large repository, check whether the same inventory shard has unreviewed sibling packages, wrappers, or root controls that could be the actual broken control. If so, finish that shard's sibling checks or mark the remaining rows `deferred` explicitly.
- For query injection candidates, parser-control evidence is enough to keep the instance alive when attacker input crosses into query syntax or selector operators. Record later checks as constraints, but do not suppress the root injection until the exact query API, parser behavior, and post-query guard prove attacker input cannot change semantics or create a meaningful read/write/error side effect.
- For template and placeholder injection candidates, preserve the helper line that creates the placeholder/expression engine and the line that resolves model values through it. Recursive expansion or re-parsing of resolved values remains in scope when the resolved values are request, client, tenant, stored configuration, or error data. Suppress only with exact evidence that resolved values are constant/trusted or escaped before any second parse/evaluation.

## Suppression And Final Reporting

- Before final output, reconcile the candidate/report set against the repository coverage ledger. Every advisory-seeded or exact-anchor row must either appear as a reported finding for the same file/source/control/sink/effect tuple or be closed as `suppressed`, `not_applicable`, or `deferred` with exact counterevidence. If an anchored row is missing because a sibling issue looked cleaner, revisit that exact file/control before finalizing.
- Suppress a candidate only with exact counterevidence for that instance, such as a specific sanitizer, permission check, tenant filter, escaping context, safe parser/loader, path canonicalization check, egress allowlist, or deployment constraint that defeats the claimed source/sink path. For route auth preconditions, exact counterevidence must reflect the real deployment-default middleware plus decorator behavior; an optional or conditional login wrapper does not prove the route is authenticated.
- A safe sibling implementation is useful negative control, but it does not suppress a different default factory, resource handler, generated adapter, protected action, parser operation, or shared-helper caller.
- A surviving auth/authz, secret, or configuration issue in the same subsystem does not close a separate resource-serving, path traversal, archive extraction, export/import, backup/restore, or file copy/move row. Each row must close on its own exact control and effect.
- For advisory-seeded path/file rows that name a file-manager, builder/media, theme/archive, export/download controller, or shared `sanitize_params`-style helper, final reconciliation must revisit that exact controller/helper even if adjacent path/file/authz findings survived. A traversal in one API namespace does not close another namespace's filename/path parameter unless the same source, sanitizer, sink, and effect are proven equivalent with file:line evidence.
- Do not close a shared-sanitizer path row as only literal traversal, safe naming, or deferred parser work while the report omits the sanitizer definition and at least one caller that enables slash-preserving behavior. Adjacent cross-scope storage, script-control, or telemetry findings are not evidence that a slash-permissive filename/path parameter was closed.
- For archive extraction rows, do not close the row with generic claims about standard-library safety, UUID/manifest gates, top-level import filtering, or later recursive copies into an approved root. Closure requires exact per-entry member-path containment evidence before the extraction or write point, including any symlink, hardlink, metadata, or subtree-promotion behavior that could still materialize attacker-controlled content. Do not require escape outside the overall app/datastore root; overwriting trusted config, peer-object directories, shared roots, or imported subtrees inside that root is still sufficient file impact.
- When final report budget forces selection among multiple path/file findings, do not prefer a higher-confidence sibling download/read issue over an upload/archive-member write issue that has the exact source, broken member-path control, and write tuple. Include both when they survive; if the archive row only lacks a runtime harness, report it at lower confidence or leave it in deferred rows rather than omitting it.
- Treat lack of an in-repository write endpoint as a precondition to state, not automatic suppression, unless repository evidence proves only trusted operators can set the value in the intended deployment.
- For command/action runner command-injection rows, closure requires exact evidence that every attacker-controllable argument type reaching shell/template execution is safely typed or escaped. A partial unsafe-type denylist, a safe direct-exec sibling, or frontend widget constraints do not suppress API/webhook-controllable argument types with nil/no-op typechecks.
- Include data exposure, hardcoded secrets, weak session/cookie/security config, CSRF, rate limits, and plaintext storage only after the high-impact ledger and file list are exhausted or when they directly enable code execution, injection, privilege escalation, meaningful auth bypass, or sensitive cross-boundary impact.
- The final markdown report may group related findings for readability, but each surviving instance must remain individually addressable with its own affected location, source, broken control, sink, impact, and counterevidence.
- Include suppressed candidates and deferred rows in phase artifacts with exact file/line and counterevidence or proof-gap reasons so false-positive controls and residual coverage gaps remain auditable.

---
### reference: repository-wide-scan

# Exhaustive Review Guidance

Use this guidance when the security scan target is the entire checked-out repository or a user-specified scoped path, package, folder, or submodule-like boundary.

## Required References

Before exhaustive repository or scoped-path discovery or validation, read this file and all of these same-directory references in order. They are mandatory extensions of this workflow, not optional background:

1. `scan-artifacts-and-ledger.md` for shared artifact, seed, subagent, scoped file-review, candidate-ledger, and dedupe rules.
2. `repo-wide-artifacts-and-ledger.md` for rank input, subagent ranking, deep-review selection, and repository coverage-ledger rules.
3. `repo-wide-high-impact-families.md` for high-impact vulnerability family heuristics and exact suppression boundaries.
4. `repo-wide-instance-expansion.md` for child-instance splitting, wrapper/root-control preservation, and per-operation reporting.
5. `repo-wide-validation-closure.md` for validation/report closure, deferred rows, secondary issue ordering, and false-positive controls.

Do not treat `repository-wide-scan.md` alone as the complete exhaustive scan procedure.

## Exhaustive Mode

Use an exhaustive instance-finding workflow rather than the diff-scan workflow's representative-finding bias.

Repository-wide and scoped-path scans must:

- Load the per-scan threat model path from `../../../references/scan-artifacts.md` as the repo-specific threat-model source of truth.
- Build or consume an authoritative parent-provided `rank_input.csv` before validation so the in-scope candidate file inventory covers routes, handlers, templates, serializers, deserializers, query builders, shell/process calls, file/path APIs, network fetches/callbacks, auth/authz middleware, session/cookie config, secret/config sources, IaC or policy resources, and agent/tool boundaries.
- Create `seed_research.md` when seed hints exist, `rank_input.csv`, `rank_output.csv` when ranking applies, `deep_review_input.csv`, `work_ledger.jsonl`, `raw_candidates.jsonl`, per-finding candidate ledgers, `dedupe_report.md`, `deduped_candidates.jsonl`, and `repository_coverage_ledger.md` using the artifact paths from `../../../references/scan-artifacts.md`.
- Create a high-impact coverage ledger before deep validation. The ledger is a coverage artifact, not a list of potential findings, and must include rows without candidates as well as reportable candidates.
- Keep every applicable high-impact, user-seeded, advisory-seeded, or tag-seeded row open until that exact area is closed as `reportable`, `suppressed`, `not_applicable`, or `deferred` with exact evidence or proof-gap reasons.
- When seed research or the prompt provides a concrete advisory id, snapshot URL, file, line, source, sink, or missing-control hint, create an anchored ledger row for that exact tuple. Sibling findings in the same repository, CWE, or subsystem are additional rows; they do not close the anchored row unless they fix the same vulnerable control and effect.
- Enumerate every technically distinct high-impact vulnerable instance discovered under those families, not just one representative example per class.
- Keep file-impact families open independently from auth, secret, or config findings in the same subsystem. A reportable auth bypass, credential issue, or sensitive-data exposure does not close a separate path traversal, archive extraction, export/import, backup/restore, file copy/move, or resource-serving row unless it defeats the exact same path-control proof tuple.
- Preserve the line where the security control actually fails, including unsafe split/parse/canonicalize/normalize/compare/regex/selection/object-binding lines that create a bypass or feed a sink.
- Suppress a candidate only with exact counterevidence for that instance, such as a specific sanitizer, permission check, tenant filter, escaping context, safe parser/loader, path canonicalization check, egress allowlist, or deployment constraint that defeats the claimed source/sink path.

## Discovery Execution

During finding discovery, apply this exhaustive repository or scoped-path workflow instead of the diff-centered discovery workflow. Use `../../finding-discovery/SKILL.md` for the candidate output contract and `../../../references/scan-artifacts.md` for artifact paths.

Run this broader but still bounded workflow:

1. Read the required references listed above.
2. Resolve `rank_input.csv` before subagent dispatch:
   - if an upstream parent orchestrator explicitly provided authoritative in-scope worklists and both `<discovery_dir>/rank_input.csv` and `<discovery_dir>/deep_review_input.csv` already exist, consume that `rank_input.csv` as supplied
   - otherwise generate `rank_input.csv` using `python3 <plugin_dir>/scripts/generate_rank_input.py make-repo-rank-input --repo <repo_root> --scope <scope> --out <discovery_dir>/rank_input.csv`; this is the deterministic candidate file inventory for the resolved repository or scoped path
3. Resolve `deep_review_input.csv`:
   - if an upstream parent orchestrator explicitly provided authoritative in-scope worklists and both standard worklist files already exist, consume that `deep_review_input.csv` as supplied without reranking or overwrite
   - otherwise apply the `top-percent` flow from `repo-wide-artifacts-and-ledger.md`: for `top-percent` below 100, run subagent ranking over `rank_input.csv` using the runtime-surface scoring guidance and select `deep_review_input.csv`; for `top-percent` 100 or higher, copy every candidate row directly into `deep_review_input.csv`
4. Run advisory/seed research when the user or scan context includes CVE, GHSA, advisory, issue, release, package-version, or vulnerability-family identifiers. Save `seed_research.md` and create exact seed-target ledger rows.
5. Build and save `repository_coverage_ledger.md` with one row per applicable boundary and serious vulnerability family before deep validation begins; include any exact anchored rows from seed research as their own rows even if another candidate in the same subsystem already exists.
6. Run one frontier pass across every applicable high-impact shard before prolonged validation or build/debug work on any single shard.
7. Run targeted control-hazard searches for parser/helper, deserializer, auth/token/assertion, protocol/version, and polymorphic-operation shards using `repo-wide-high-impact-families.md`.
8. For path-sensitive filesystem review, enumerate exported or deployed static/resource handlers, download/open helpers, upload/extract/import flows, export flows, backup/restore flows, file copy/move helpers, and archive entry writers/readers before deepening any one hotspot. Give each independently reachable operation its own ledger row.
9. Run high-impact sibling-expansion passes before any secondary review. When one vulnerable pattern is found, the file-review subagent or parent agent that owns that candidate must search sibling files, routes, templates, handlers, models, and config variants before moving on.
10. When a high-impact instance flows through a wrapper into a shared parser, deserializer, path/archive helper, expression evaluator, or auth/authz control, record both the reachable wrapper and the underlying shared sink/control.
11. If a filesystem/path row and an auth/authz/config row both survive in the same product area, carry both forward until the exact control for each row is closed. Do not let the louder or easier-to-explain issue replace the sibling row.
12. Dispatch file-review subagents over `deep_review_input.csv` using the shared ownership rules in `scan-artifacts-and-ledger.md`. Each file-review subagent owns its assigned file or tiny shard, performs full-file review, and returns pre-dedupe finding objects with candidate-local validation evidence and attack-path facts for findings it discovered.
13. Aggregate file-review-subagent outputs into `raw_candidates.jsonl` and append one candidate-ledger row per raw candidate finding.
14. Do not continue until each raw candidate finding's candidate-ledger path from `../../../references/scan-artifacts.md` shows validation and attack-path coverage, or an explicit deferred reason for any missing proof.
15. Split broad families and repeated same-family operations into child instances using `repo-wide-instance-expansion.md` before cross-file dedupe whenever the child instances are already visible in subagent output.
16. Run cross-file dedupe into `dedupe_report.md` and `deduped_candidates.jsonl` without dropping independently reachable sibling instances, and preserve the raw candidate ids absorbed into each deduped candidate.
17. Use post-dedupe validation and attack-path work for exhaustive-scan reconciliation, unresolved proof gaps, and final closure, not as the first review pass for raw findings. When multiple deduped candidates or coverage-ledger rows remain open and subagents are available and approved, divide validation and attack-path work across candidate/row-scoped subagents using `scan-artifacts-and-ledger.md`.
18. Treat data exposure, hardcoded secrets, weak session/cookie/security config, CSRF, rate limits, and plaintext storage as secondary. Include them only after the high-impact ledger and file list are exhausted or when they directly enable code execution, injection, privilege escalation, meaningful auth bypass, or sensitive cross-boundary impact.
19. Preserve each validated or suppressed instance through validation, attack-path analysis, and final reporting using `repo-wide-validation-closure.md`.

---
### reference: scan-artifacts-and-ledger

# Scan Artifacts And Ledger

Use this reference whenever the scan needs auditable candidate coverage or a scoped file-review worklist.

## Artifact Requirements

- Load the per-scan threat model path from `../../../references/scan-artifacts.md` as the repo-specific threat-model source of truth.
- Use the artifact paths from `../../../references/scan-artifacts.md` for `seed_research.md`, `deep_review_input.csv` when a scoped file-review worklist is needed, `work_ledger.jsonl` when a scoped file-review worklist is needed, `raw_candidates.jsonl` when multiple file-review results are aggregated, `dedupe_report.md` and `deduped_candidates.jsonl` when cross-file dedupe is needed, and per-finding `05_findings/<candidate_id>/candidate_ledger.jsonl`.

## Seed Research

- First capture user-provided scope hints such as CVE/GHSA/advisory identifiers, package versions, named vulnerability families, or release/security-test references.
- When the user request or scan context includes CVE, GHSA, advisory, issue, release, package-version, or explicit vulnerability-family identifiers, run an advisory seed pass before deep frontier scanning and save it to the advisory seed research path from `../../../references/scan-artifacts.md`.
- Use authoritative advisory text, project security notes, release notes, fix commits, pull requests, issue trackers, and security tests when network access or local history is available. Record the sources searched, candidate files/functions/classes/hunks, expected vulnerable behavior, and any failed lookup attempts.
- Treat those candidates as seed rows only: validate the vulnerable behavior against the checked-out repository before reporting. Do not let the seed lane replace the scan's primary scope.
- When CVE/advisory context has a generic or unhelpful category, prioritize advisory, fix-commit, release-note, and security-test lookup before broad sink hotspot scanning. If external lookup is unavailable or inconclusive, run a local regression-seed pass over project-specific protocol, parser, validator, and utility names plus the CVE/advisory terms; do not assume obvious REST/upload/XML hotspots are the intended security regression.
- When the seed pass or local search opens a candidate file, class, package, or hunk, create an exact seed-target row for that area before opportunistic same-family scanning. Run a short seed-first triage over that file/package and its immediate shared helper or caller chain, then close the row as `reportable`, `suppressed`, `not_applicable`, or `deferred`. A more obvious neighboring issue can be reported too, but it does not replace the seed-target row.
- Keep every user/advisory/tag-seeded boundary package or class family open until that exact area is closed as `reportable`, `suppressed`, `not_applicable`, or `deferred`. A broader same-family finding in a neighboring parser, auth flow, deserializer, or template engine does not implicitly close the seeded row.
- In advisory-led scans, treat the advisory, fix hunk, release note, or security test as evidence for the intended root cause, not as an exclusivity filter and not as a bare finding. Keep the exact seed row open until checked-out repository evidence independently supports or disproves the same source, broken control, and impact tuple.

## Subagent Requirements

- When a scan uses subagent-dispatch phases and subagents are available in the current tool set, use subagents for those phases.
- If subagents are available and the user has not explicitly allowed subagents, stop before starting subagent dispatch and ask for that approval.
- File-review-subagent ownership: one file-review subagent owns one `deep_review_input.csv` row or one very small tightly coupled shard, max 5 files, and returns full-file receipts plus pre-dedupe finding objects for that assignment.
- File-review subagents are read-only with respect to the target code under review, but they are allowed and expected to write scan artifacts under the resolved numbered artifact directories, including `<discovery_dir>/work_ledger.jsonl`, raw candidate snippets in `<discovery_dir>/raw_candidates.jsonl`, and per-candidate ledger receipts under `<findings_dir>/<candidate_id>/` when those artifact paths are provided in the prompt.
- Validation-subagent ownership: one validation subagent owns one candidate finding, one deduped candidate, or one repository coverage-ledger row that needs validation closure. It writes or returns validation artifacts, the visible validation report update, and the validation candidate-ledger receipt for that assignment.
- Attack-path-subagent ownership: one attack-path subagent owns one validated candidate finding or one reportable/deferred validation closure row. It writes or returns attack-path facts, severity/policy analysis, the visible attack-path report update, and the attack-path candidate-ledger receipt for that assignment.
- Parent-agent ownership: the parent agent owns `deep_review_input.csv` generation, subagent dispatch, work-ledger and candidate-ledger reconciliation, aggregation of subagent outputs, cross-file dedupe when needed, and final scan closure.
- Subagent prompts must carry the exact current scan instructions they are expected to follow. Do not rely on the subagent implicitly inheriting this skill, another phase skill, previous parent context, or a summarized reference name.

### File-Review Subagent Handoff

The parent agent should give each file-review subagent enough concrete context to execute its assigned row without relying on implicit parent context. Keep the prompt concise, but include:

- the assigned `deep_review_input.csv` row or tiny shard
- the scan target, scan mode, `repo_name`, `scan_id`, `artifacts_dir`, the relevant numbered artifact directories, and per-scan threat model path or summary
- the writable artifact paths for `<discovery_dir>/work_ledger.jsonl`, `<discovery_dir>/raw_candidates.jsonl`, and per-candidate ledger receipts under `<findings_dir>/<candidate_id>/`
- any user-provided comparison or seed artifact that should affect coverage, such as an HTML/markdown report, prior security-review output, advisory text, CVE/GHSA, release note, issue, or separate audit directory
- the expectation to read assigned files in full, read only the supporting files needed for concrete findings, and return or write full-file receipts, raw candidates, suppressions/deferred rows, and ledger receipts

The parent agent must reject or re-prompt any file-review subagent result that lacks full-file receipts, omits source/control/sink/impact for candidates, omits candidate-local validation or attack-path facts for reported candidates, or returns only "no bugs found" without closing the assigned row with evidence.

### Validation And Attack-Path Subagent Handoff

After discovery and dedupe, divide validation and attack-path work across subagents when multiple candidates or coverage-ledger rows need closure and subagents are available and approved.

For validation subagents, include the candidate or ledger row, discovery evidence, relevant raw/deduped candidate ids, affected files, threat-model context, validation artifact/report paths, and the candidate-ledger path that needs the validation receipt. The validation subagent should preserve or suppress the assigned instance only; it does not own final report assembly or unrelated candidates.

For attack-path subagents, include the validated candidate or validation closure row, validation report path or summary, affected root-control and sink lines, threat-model context, attack-path report path, and the candidate-ledger path that needs the attack-path receipt. The attack-path subagent should produce reachability, counterevidence, severity/policy analysis, and final reportability facts for the assigned instance only.

The parent agent must reconcile validation and attack-path subagent outputs before final reporting. Do not finalize while any reportable, suppressed, not_applicable, or deferred candidate/ledger row lacks the required receipt or explicit proof-gap reason.

## Scoped Deep Review

- Use `deep_review_input.csv` as the canonical scoped deep-review worklist for every diff-scoped, repository-wide, and scoped-path scan.
- For diff-scoped scans, generate `rank_input.csv` deterministically from changed source-like files with `python3 <plugin_dir>/scripts/generate_rank_input.py make-diff-rank-input --repo <repo_root> --base <base> --mode revisions --head <head> --out <discovery_dir>/rank_input.csv` for PR, commit, and branch diffs, or `python3 <plugin_dir>/scripts/generate_rank_input.py make-diff-rank-input --repo <repo_root> --base <base> --mode local-patch --out <discovery_dir>/rank_input.csv` for a local patch, then copy every row into `deep_review_input.csv` with `python3 <plugin_dir>/scripts/generate_rank_input.py copy-deep-review-input --rank-input <discovery_dir>/rank_input.csv --out <discovery_dir>/deep_review_input.csv`.
- Diff-scoped scans do not rank or drop changed files before deep review. Every row in diff `rank_input.csv` must be copied into `deep_review_input.csv` and receive a full-file review receipt.
- Add directly supporting files required to understand the changed security behavior only when repository evidence shows they are needed; record the add-back reason in the work ledger or per-file result.
- For repository-wide and scoped-path scans, `deep_review_input.csv` is selected from the ranked in-scope inventory.
- Deep-review every file selected into `deep_review_input.csv`.
  - Use `<discovery_dir>/work_ledger.jsonl` as the append-only record of claims and completions, and reconcile it against `deep_review_input.csv` so rows are not skipped or double-counted.
  - Use subagents when available and approved.
    - A file-review subagent must read every assigned file in full, update the ledger receipt for those files, and return the raw finding results for that assignment.
    - When a file-review subagent finds a plausible finding in its assigned file or shard, that same subagent should carry that finding through candidate-local validation and candidate-local attack-path analysis before handing it back. The raw result should include the source or privileged boundary, closest relevant control, sink or broken control, impact, validation method/evidence or exact proof gap, attack-path facts, and disposition.
    - A file-review subagent may read the minimum supporting files needed to validate or explain a finding it discovered, but it does not own unrelated rows or final scan closure.
  - If subagents are not available, iterate through `deep_review_input.csv` yourself with the same full-file standard.
  - A file is not covered because it appeared in searches. It is covered only when the responsible subagent or parent agent returns a receipt showing the file was read in full.
  - Record file-level completion, disposition, and a concise evidence note in `<discovery_dir>/work_ledger.jsonl`; do not create a separate per-file findings directory.
  - Append normalized, pre-dedupe candidate objects to `<discovery_dir>/raw_candidates.jsonl` when multiple file-review results are being aggregated or cross-file dedupe is needed.
  - Do not stop until every `deep_review_input.csv` row has a completion receipt.

## Candidate Finding Coverage

- Track candidate-finding coverage separately from file coverage.
  - Use `<findings_dir>/<candidate_id>/candidate_ledger.jsonl` as the append-only record for each candidate finding.
  - Every candidate finding must have a stable candidate id plus candidate-ledger receipts for discovery, validation, and attack-path analysis before final reporting.
  - When the finding is emitted during a scoped file-review pass, candidate-local validation and candidate-local attack-path analysis should be recorded before the finding is eligible for cross-file dedupe.
  - Validation coverage must record the validation method, evidence or exact proof gap, and disposition for that candidate finding.
  - Attack-path coverage must record the source or privileged boundary, closest relevant control, sink or broken control, impact path, and severity-relevant facts for that candidate finding.
  - A candidate finding is not covered because it appears in a report or `raw_candidates.jsonl`. It is covered only when its candidate ledger shows the required receipts, or an explicit deferred reason for the missing proof.
- When multiple raw candidate streams need cross-file dedupe, dedupe only after the relevant candidate ledgers prove the required coverage. Write `<reconciliation_dir>/dedupe_report.md` and `<reconciliation_dir>/deduped_candidates.jsonl`, preserving independently reachable sibling instances.
- Dedupe must preserve candidate-ledger traceability: every deduped candidate must list the raw candidate ids and per-finding candidate ledgers it absorbed, and independently reachable sibling instances must remain separate.
