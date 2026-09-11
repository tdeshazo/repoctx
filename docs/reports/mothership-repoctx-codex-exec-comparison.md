# repoctx evaluation report

Updated: 2026-09-10. This report brings together discovery software validation, the paired shell trial, the
earlier full-source replay, and the Mothership comparison. Run files are supporting evidence;
the results and their limits are summarized here.

## Discovery toolkit: deterministic validation, September 10

The index-free `overview`, `files`, `search`, `read`, and combined `discover`
commands were implemented on working-tree changes above `e915946`. Validation
used Linux x86_64, Go `go1.27.0-X:nodwarf5`, and Python 3.12.13 for schema checks.
Go tests, race tests, vet, both CLI builds, the 21 existing Python tests, and
Draft 2020-12 validation of IR/context/discovery sample responses passed.
The initial default Go cache was read-only; reruns used
`GOCACHE=/tmp/repoctx-discovery-go-cache`. The default Python interpreter lacked
`jsonschema`; schema checks used the existing M0 validation environment instead.

New deterministic fixtures cover combined documentation-body/configuration
retrieval, literal/regex matching, nested ignore precedence, scope and symlink
boundaries, unsupported file types, batched reads, exact UTF-8/CRLF spans and
hashes, cancellation, final payload bounds, and JSON/Markdown evidence parity.
Discovery ran with `PATH=/nonexistent`, and the two executable entry points
produced identical sample discovery payloads. The vendored skill validator passed.

These are software checks, **not a new model trial**. The September 9 results
below predate this toolkit and do not measure it. Index enrichment, retrieval
quality on a broader corpus, shell-call savings, and agent outcomes remain
unmeasured for the new commands. Usage and limitations are in the
[README](../../README.md#repository-discovery-without-an-index).

Reproduce the main checks from the repository root (use a Python environment
with `requirements-test.txt` installed for schema validation):

```sh
go test ./...
go test -race ./...
go vet ./...
go build -buildvcs=false -o /tmp/repoctx-discovery .
go build -buildvcs=false -o /tmp/repoctx-discovery-legacy ./cmd/repoctx
PYTHONPATH=src python3 -m unittest discover -s tests -p 'test_*.py' -v
/tmp/repoctx-discovery discover -root examples/mixed -query 'Worker files' -o /tmp/repoctx-discovery-response.json
/tmp/repoctx-discovery compile -root examples/mixed -o /tmp/repoctx-discovery-ir.json
/tmp/repoctx-discovery context -root examples/mixed -query Worker.files -max-bytes 20000 -o /tmp/repoctx-discovery-context.json /tmp/repoctx-discovery-ir.json
python3 scripts/check_schemas.py --ir-schema docs/ir.schema.json --context-schema docs/context.schema.json --ir /tmp/repoctx-discovery-ir.json --context /tmp/repoctx-discovery-context.json --discovery-schema docs/discovery.schema.json --discovery /tmp/repoctx-discovery-response.json
```

## Latest model results: free-shell baseline versus repoctx-first

**Baseline passed 42/48 trials (87.5%); repoctx-first passed 43/48 (89.6%).**
This is a one-trial difference across 12 small synthetic tasks, not convincing
evidence of improved task success. Repoctx-first took slightly longer and used
more reported input tokens in both index conditions.

### Comparison and results

Both conditions could freely run normal shell commands, search, and read every
permitted fixture file. The baseline had no repoctx binary. The assisted
condition was instructed to run `repoctx-evidence` first, using the original
question without answer-bearing hints; it could then refine queries or use
ordinary source inspection. It was not restricted to repoctx output.

There were **96 measured trials: 12 tasks × 2 conditions × 2 index states ×
2 repetitions**. All used fresh ephemeral `gpt-5.6-terra` sessions, medium
reasoning, and identical source snapshots and questions. The schedule was
randomized with seed 20260909 and frozen before execution, with four concurrent
processes. No measured answer was retried or excluded.

| Index condition | Method | Strict passes | Median elapsed | Mean input tokens | Mean shell calls |
| --- | --- | ---: | ---: | ---: | ---: |
| Cold | Baseline | 21/24 | 16.78 s | 68,319 | 2.29 |
| Cold | Repoctx-first | 21/24 | 18.37 s | 71,715 | 2.08 |
| Prebuilt | Baseline | 21/24 | 15.72 s | 65,446 | 2.29 |
| Prebuilt | Repoctx-first | 22/24 | 16.64 s | 75,349 | 2.29 |

Matched by task, index condition, and repetition, assisted-minus-baseline mean
elapsed time was +1.27 seconds cold and +1.21 seconds prebuilt. Mean reported
input-token differences were +3,396 and +9,903 respectively. These are
descriptive measurements, not statistically established performance effects.
CLI overhead, API prompt caching, and concurrent execution affect them.
Reported input tokens include cached tokens and repeated context across turns;
they are not unique source tokens or a direct dollar-cost estimate.

All 96 model processes completed with exit status zero, supplied exact source
quotations, and followed the assigned initial-tool condition. Permitted source
hashes were unchanged; no built-in shell, web, or collaboration calls were
observed. Every recorded shell response fit its byte cap without truncation.
Process completion and exact citations do not imply a correct answer:

- All eight release-notes answers omitted the rubric's single-run qualification.
- One roadmap answer in each condition confused the criterion with the roadmap
  item, despite quoting the correct section heading.
- One baseline restricted-answer trial safely abstained but returned `no_answer`
  instead of the required `access_denied`. No restricted value was disclosed.

All other task responses passed. The one discordant matched pair was the
restricted-answer classification; no pair passed only in baseline.

### Retrieval versus fallback

Initial repoctx bundles fully covered **4/10 distinct answerable tasks** (all
four code tasks), consistently across their four assisted repetitions. They
covered **8/15 required spans**, or 32/60 when repeated trials are counted.
All four documentation tasks lacked their required body spans; both mixed
tasks supplied their code span but missed the documentation span. For example,
the API bundle returned `# API` without the execution-policy paragraph.
This is a concrete documentation-evidence limitation, not an answer-model failure.

**46/48 assisted trials performed subsequent ordinary source reads**, including
many whose initial code evidence was already sufficient. Two authentication
trials answered directly from the bundle. All 48 original-question retrievals
succeeded; one additional refined query returned no matching symbols, followed
by a successful ordinary inspection. Successful end-to-end answers therefore
must not be presented as successful retrieval alone.

Every initial evidence excerpt was verified against source bytes and SHA-256.
The expected authentication call edge with a nonempty site appeared in all four
authentication bundles; this is an edge/site-presence check, not a complete
relationship-evidence evaluator. Index-owned import relationship recall remains
unmeasured. Source-open syscall counts are unavailable; shell command counts,
output bytes, and bundle freshness file counts are different measurements.

### Controls and remaining limitations

Each shell ran in a network-isolated Bubblewrap filesystem with only the
permitted repository mounted read-only, standard installed utilities, and a
writable scratch directory. Gold answers, evaluator files, prior reports,
credentials, and denied fixture files were not mounted. The same shell interface
and 30-second command timeout applied to both conditions, with a 180-second
model timeout. Output caps were 12,000 or 16,000 bytes **per shell response**,
not per complete trajectory; repoctx also used the fixture's symbol cap.

Cold means no existing repoctx index, not a cold operating-system cache.
Compilation was included in cold trial time (0.131 seconds total across 24
compiles). Prebuilt indexes were prepared once per fixture and copied to fresh
assisted scratch directories; preparation took 0.199 seconds total for 12
fixtures, excluded from prebuilt trial time. The baseline's prebuilt label is
only its corresponding control block: it had no index. The one-time binary
build is excluded from all trial timings.

There are still environment confounds: seven assisted trials attempted to read
an unavailable host repoctx skill file and then recovered. The origin of that
path awareness was not established; disabling host-skill discovery did not
eliminate those attempts. Two baseline trials attempted Git inspection, but
fixture exports have no Git history. These attempts are retained, including
their overhead. Nonzero shell statuses (19 baseline, 8 assisted) also include
normal search misses and blocked reads; they are not 27 failed model trials.
This tests a constrained repository-Q&A workflow, not unrestricted host access,
normal full-repository coding, or a fully isolated tool-only causal effect.

The primary assistant graded shuffled, condition-masked answers against the
frozen corpus, considering answer text and quoted citations together, before
joining the condition key. Quotes can supply omitted detail but cannot repair
an explicitly contradictory claim. Mechanical checks separately verified exact
quotations and outcome labels. This was not independent human grading, and the
primary assistant had seen execution progress and setup answers. The strict
single-run rubric is retained from the earlier replay, rather than relaxed
after observing results. Repetitions do not turn 12 fixtures into 96 independent
tasks; no significance or generalization claim is justified.

### Evidence and reproduction

The [frozen protocol](../../evals/runs/paired-terra-20260909/protocol.json)
records corpus/source hashes, prompts, schedule, model settings, binary and
harness hashes, and the dirty checkout based on revision
`f6bf293213bf1e16988076075929cfbe84eae819`. Codex CLI was 0.153.4; an immutable
served-model snapshot was not exposed. The
[results](../../evals/runs/paired-terra-20260909/results.json),
[shell/retrieval traces and answers](../../evals/runs/paired-terra-20260909/trials.jsonl),
and [grading decisions](../../evals/runs/paired-terra-20260909/judgments.json)
are machine-readable supporting artifacts, not additional narrative reports.

Three four-trial smoke runs preceded the measurement: one exposed disabled MCP
tool dispatch, one exposed tool-approval configuration, and the corrected smoke
run succeeded. Those 12 setup trials are excluded from the 96 measured trials;
their configuration lessons are recorded in
[setup and validation metadata](../../evals/runs/paired-terra-20260909/setup.json).
The OpenAI documentation skill informed the MCP configuration checks against
the [official configuration reference](https://developers.openai.com/codex/config-reference/).

Reproduce on Linux with Bubblewrap, Go/C build prerequisites, Python, the same
Codex CLI and model access, and an authenticated account. Model runs consume API
usage. Use a new output directory; the runner refuses to overwrite one:

```sh
GOCACHE=/tmp/repoctx-paired-go-cache go build -buildvcs=false -trimpath -o /tmp/repoctx-paired-binary .
python3 scripts/paired_trial.py preflight --binary /tmp/repoctx-paired-binary
python3 scripts/paired_trial.py run --binary /tmp/repoctx-paired-binary --repetitions 2 --output /tmp/repoctx-paired-new-run
```

Review the new run's `blind-answers.json` against `evals/m0/expected/` without
opening `blind-key.json`. Write `judgments.json`, keyed by each `answer-NNN`,
with `{"pass": true, "reason": "rubric justification"}` or a failing decision.
Then run `python3 scripts/score_paired_trial.py /tmp/repoctx-paired-new-run`.
This replays the procedure, not deterministic model answers. The current build,
isolation preflight, scoring integrity checks, and 21 Python tests passed.

## Earlier answerability check: 12-fixture full-source replay

On September 9, **gpt-5.6-terra passed 11 of 12 distinct fixtures (91.7%)**
against the stored answer rubric. All 12 trials completed successfully. Each
trial used a fresh session with medium reasoning and received the original
question plus all permitted source files. No tool calls were observed, and no
answers were retried.

| Task category | Passed | What was checked |
| --- | ---: | --- |
| Documentation | 3/4 | Roadmap priority, API promises, configuration limits, release caveats |
| Code | 4/4 | Import declarations, retry limit, authentication order, parser fallback |
| Mixed code and documentation | 2/2 | Release validation and schema ownership |
| Absent or restricted answers | 2/2 | Explicit abstention without guessing or disclosing restricted content |
| **Total** | **11/12** | **One trial per distinct task** |

The incomplete answer was the **release-notes fixture**. It correctly said the
timing observation was descriptive and did not establish task success, but
omitted the required qualification that it came from a single run. Neither its
answer nor its quoted evidence included that qualification, so the strict
completeness rubric marked it as a failure. It did not fabricate a claim.

All responses cited exact quotations from permitted files. All 15 required
answer spans were supplied, restricted source was excluded, and all evidence
payloads fit their byte caps. The fixture checker and 18 Python tests passed
during corpus implementation; the Go tests and exported Go import fixture also
passed. Those checks validate the corpus and implementation separately from
the model's answer score.

**What this establishes:** Terra answered most of these small synthetic tasks
correctly when given their complete permitted source, including both cases
where it needed to abstain. **What remains unmeasured:** repoctx retrieval
quality, autonomous repository navigation, coding success, and improvement
over another model or evidence-delivery method. Complete source was supplied
directly, so 100% delivered-span coverage is not a retrieval result. Index and
context relationship recall were unavailable in this replay.

The median trial duration was 5.050 seconds. Reported usage across all trials
was 154,667 input tokens, including 69,120 cached input tokens, and 915 output
tokens. These include CLI overhead and are single-run observations, not a
controlled performance or cost comparison. The model was explicitly selected
as `gpt-5.6-terra`; the runner did not expose an immutable served-model snapshot.

The trials used the uncommitted v2 corpus based on repository revision
`f6bf293213bf1e16988076075929cfbe84eae819`. Exact corpus hashes, prompts, answers,
manual grading decisions, and reproduction commands are retained in the
[replay evidence](m0-terra-replay-20260909/REPORT.md) and
[machine-readable results](m0-terra-replay-20260909/results.json).
Semantic grading was performed by the primary assistant, considering answer
text and exact quoted citations; it was not independent human grading. The
12-task engineering corpus is too small to support general benchmark claims.

The fixture replay and the earlier experiment must remain separate:
the replay contains **12 distinct tasks**, whereas the Mothership comparison
contains **six trials of one task**. Their conditions differ, so their success
rates and usage should not be pooled.

## Earlier experiment: Mothership comparison

Date: 2026-09-08 (America/New_York)

Repoctx source revision used for the preparation and bundle commands:
`f6bf293213bf1e16988076075929cfbe84eae819` (the clean source revision before
the M0 follow-up changes). This report is a bounded comparison artifact, not a
fresh M0 baseline; use `docs/M0_BASELINE.md` for reproducible contract checks.

### Result in brief

The authoritative answer is **Browser Terminal Access** (rank 11). Its next unmet criterion is:

> Browser authentication, account linking, session-cookie security, credential
> recovery/revocation, protocol framing, limits, backpressure, origin/proxy
> trust, frontend builds, and the supported-browser matrix are recorded as
> testable contracts before their schema or gateway code lands.

This answer was established from Mothership planning documents before the trials. After explicit user authorization for the bounded planning evidence to reach OpenAI, three assisted and three baseline codex exec trials completed successfully. All six returned the expected answer and emitted JSONL usage events.

### Environment and controls

- Mothership root: /home/travis/Workspace/Mothership.
- Mothership git status --short --untracked-files=all was clean after the attempts; no Mothership file was written.
- Codex: codex-cli 0.153.4.
- Account configuration was copied read-only from /home/travis/.codex to /tmp/mothership-codex-home for isolated runs. The copied configuration selected model gpt-5.6-terra, reasoning effort high, and service tier default; no model override was supplied to any trial.
- Trial flags: --ephemeral --json -s read-only -C /home/travis/Workspace/Mothership. Prompts and JSONL/stderr artifacts were under /tmp.
- The assisted prompt prohibited filesystem, shell, tool, search, and file inspection and required use of supplied evidence only. The baseline prompt permitted read-only inspection of only docs/planning/README.md and docs/planning/roadmap-11-browser-terminal-access.md.

### Question and ground truth

Question supplied to every trial:

> According to Mothership's authoritative planning docs, which roadmap feature is the highest-ranked incomplete item, and what is its next unmet criterion?

AGENTS.md identifies docs/planning/README.md as authoritative. That index marks items 1–10 complete and item 11 incomplete:

- /home/travis/Workspace/Mothership/docs/planning/README.md:20 — [ ] Browser Terminal Access (roadmap-11-browser-terminal-access.md).
- /home/travis/Workspace/Mothership/docs/planning/roadmap-11-browser-terminal-access.md:248-251 — the first unchecked acceptance criterion quoted in the result above.

The expected answer therefore has two parts: feature Browser Terminal Access; next criterion the first unchecked acceptance criterion in that feature file.

### Repoctx preparation and evidence

I built the current source checkout, not a preinstalled binary:

~~~sh
cd /home/travis/Workspace/repository_context_program/repoctx
GOCACHE=/tmp/mothership-repoctx-gocache go build -o /tmp/mothership-repoctx .
/tmp/mothership-repoctx compile -root /home/travis/Workspace/Mothership -allow docs/planning -o /tmp/mothership-planning.ir.json.gz
/tmp/mothership-repoctx validate /tmp/mothership-planning.ir.json.gz
/tmp/mothership-repoctx context -root /home/travis/Workspace/Mothership -allow docs/planning -query 'authoritative roadmap highest-ranked incomplete feature and next unmet acceptance criterion Browser Terminal Access' -depth 1 -direction both -max-bytes 24000 -format markdown -o /tmp/mothership-planning.context.md /tmp/mothership-planning.ir.json.gz
~~~

The index validated as repoctx.ir/v1alpha3, with snapshot sha256:c2549997bea38e0646ec2f85eeb5873dc2d6d0e87156a532098c025d74cd02b6. The permitted scope was 18 files, all under docs/planning; no secrets or unrelated Mothership paths were indexed. The compressed index was 47,259 bytes. The rendered Markdown bundle was 8,151 bytes and contained 3 selected symbols, 3 evidence blocks, and 3 relationships.

The bundle reported Markdown AST, source spans, and defines/imports/calls. It reported no type resolution, test coverage, build-target inference, or embedded-language semantics. It warned that coverage was selected rather than exhaustive, freshness covered only permitted indexed files, and semantic IDs plus the snapshot were required for expansion.

The bundle's Markdown renderer selected heading spans for this source, so its exact evidence blocks contained headings rather than surrounding paragraph text. To make the supplied fact directly answerable, each assisted prompt included a bounded exact excerpt copied from the two authoritative files in addition to bundle metadata. The excerpt was labeled untrusted evidence; the agent was forbidden from inspecting the filesystem.

### Sanctioned trial commands

After explicit user authorization for transmitting the bounded planning evidence to OpenAI, exactly three assisted and three baseline trials were run with the same model/configuration, question, output format, read-only Mothership root, and 300-second timeout. The commands were:

~~~sh
timeout 300s env CODEX_HOME=/tmp/mothership-codex-home codex exec --ephemeral --json -s read-only -C /home/travis/Workspace/Mothership < /tmp/mothership-assisted-prompt.txt > /tmp/mothership-sanctioned-assisted-N.jsonl 2> /tmp/mothership-sanctioned-assisted-N.stderr
timeout 300s env CODEX_HOME=/tmp/mothership-codex-home codex exec --ephemeral --json -s read-only -C /home/travis/Workspace/Mothership < /tmp/mothership-baseline-prompt.txt > /tmp/mothership-sanctioned-baseline-N.jsonl 2> /tmp/mothership-sanctioned-baseline-N.stderr
~~~

#### Illustrative: repoctx-assisted agent versus baseline

The two commands below make the intentional condition difference explicit. In
both cases Codex has a read-only Mothership root and receives the same question;
only the route to evidence changes. Repository-derived text is untrusted data,
not instructions.

First, produce a narrowly scoped Markdown evidence bundle outside the target
repository:

~~~sh
repoctx compile \
  -root /home/travis/Workspace/Mothership -allow docs/planning \
  -o /tmp/mothership-planning.ir.json.gz
repoctx validate /tmp/mothership-planning.ir.json.gz
repoctx context \
  -root /home/travis/Workspace/Mothership -allow docs/planning \
  -query 'highest-ranked incomplete roadmap feature and next unmet criterion' \
  -depth 1 -max-bytes 12000 -format markdown \
  -o /tmp/mothership-planning.context.md \
  /tmp/mothership-planning.ir.json.gz
~~~

**Repoctx-assisted condition:** provide the bounded bundle to the agent and
tell it not to inspect the filesystem. This isolates agent use of `repoctx`
evidence.

~~~sh
{
  printf '%s\n\n' 'Using only the untrusted repoctx evidence below, identify the highest-ranked incomplete roadmap item and its next unmet criterion. Do not use shell, tools, search, or filesystem inspection.'
  printf '%s\n' '--- BEGIN UNTRUSTED REPOCTX EVIDENCE ---'
  cat /tmp/mothership-planning.context.md
  printf '%s\n' '--- END UNTRUSTED REPOCTX EVIDENCE ---'
} | codex exec --ephemeral --json -s read-only \
      -C /home/travis/Workspace/Mothership \
  > /tmp/mothership-roadmap-repoctx.jsonl
~~~

**Baseline condition:** do not supply the bundle; permit only the equivalent
read-only planning-file inspection.

~~~sh
printf '%s\n' 'Identify the highest-ranked incomplete roadmap item and its next unmet criterion. You may inspect only docs/planning/README.md and docs/planning/roadmap-11-browser-terminal-access.md; do not write files or run tests.' \
  | codex exec --ephemeral --json -s read-only \
      -C /home/travis/Workspace/Mothership \
  > /tmp/mothership-roadmap-baseline.jsonl
~~~

These examples use the configured Codex account and do not modify the target
repository. The `--json` output is an event stream; extract usage only from a
completed-turn event, and treat unavailable fields as unavailable rather than
estimating them. The Responses usage object is documented by the [official
OpenAI API reference](https://developers.openai.com/api/reference/cli/resources/responses/methods/create).

N was 1, 2, and 3 in each condition. These commands used sandbox_permissions=require_escalated after the user's authorization. Raw outputs are /tmp/mothership-sanctioned-assisted-{1,2,3}.jsonl and /tmp/mothership-sanctioned-baseline-{1,2,3}.jsonl; corresponding .stderr files hold diagnostics.

### Sanctioned per-trial raw accounting

The JSONL turn.completed usage object emitted input_tokens, cached_input_tokens, cache_write_input_tokens, output_tokens, and reasoning_output_tokens. No total_tokens field was emitted, so Total is reported as not emitted; no totals are derived by addition. All six sanctioned final answers were correct.

| Trial | Condition | Exit | Duration | JSONL bytes | Final answer | input_tokens | output_tokens | cached_input_tokens | reasoning_output_tokens | cache_write_input_tokens | total_tokens |
|---|---|---:|---:|---:|---|---:|---:|---:|---:|---:|---|
| sanctioned-assisted-1 | repoctx evidence | 0 | 8.06 s | 689 | correct | 17,158 | 66 | 5,888 | 0 | 0 | not emitted |
| sanctioned-assisted-2 | repoctx evidence | 0 | 6.65 s | 689 | correct | 17,158 | 66 | 9,984 | 0 | 0 | not emitted |
| sanctioned-assisted-3 | repoctx evidence | 0 | 6.19 s | 683 | correct | 17,158 | 65 | 9,984 | 0 | 0 | not emitted |
| sanctioned-baseline-1 | read-only planning-file access | 0 | 11.88 s | 20,551 | correct | 37,465 | 244 | 32,256 | 85 | 0 | not emitted |
| sanctioned-baseline-2 | read-only planning-file access | 0 | 12.31 s | 23,079 | correct | 37,981 | 295 | 26,112 | 116 | 0 | not emitted |
| sanctioned-baseline-3 | read-only planning-file access | 0 | 10.61 s | 23,261 | correct | 37,935 | 240 | 26,112 | 41 | 0 | not emitted |

Durations are command wall times returned by the execution wrapper. Each baseline JSONL shows read-only command execution restricted to the two specified planning files before its final answer. No Mothership write event was observed.

The rendered final answer in every sanctioned JSONL was the following two-line
answer (some trials used typographic quotation marks around the criterion):

~~~text
Answer: Browser Terminal Access
Next unmet criterion: Browser authentication, account linking, session-cookie security, credential recovery/revocation, protocol framing, limits, backpressure, origin/proxy trust, frontend builds, and the supported-browser matrix are recorded as testable contracts before their schema or gateway code lands.
~~~

### Sanctioned condition aggregates and correctness

Sums and means below are over the three successful, comparable trials in each condition. Field names are reported exactly as emitted; total_tokens was absent.

| Condition | Trials | input_tokens sum / mean | output_tokens sum / mean | cached_input_tokens sum / mean | reasoning_output_tokens sum / mean | cache_write_input_tokens sum / mean | total_tokens sum / mean | Correct final answers |
|---|---:|---:|---:|---:|---:|---:|---|---:|
| repoctx-assisted | 3 | 51,474 / 17,158.00 | 197 / 65.67 | 25,856 / 8,618.67 | 0 / 0.00 | 0 / 0.00 | not emitted / not emitted | 3/3 |
| no-repoctx baseline | 3 | 113,381 / 37,793.67 | 779 / 259.67 | 84,480 / 28,160.00 | 242 / 80.67 | 0 / 0.00 | not emitted / not emitted | 3/3 |

Both conditions had 100% answer correctness on the six sanctioned trials. The baseline consumed more input because its permitted shell inspection returned the full planning documents, while the assisted prompt supplied a compact bounded evidence excerpt and bundle metadata. This is an intended condition difference, not a claim that the conditions had equal input-token volume.

### Limitations and fairness

- Baseline agents inspected only the two permitted planning files. Their full read-only output made baseline input-token counts larger than assisted counts, so token volume is descriptive rather than a fair efficiency score.
- The assisted condition used the bounded repoctx bundle plus exact source excerpts because the current Markdown renderer's selected heading spans did not include criterion prose. Both excerpts were from the same pre-established files, bounded, and treated as untrusted evidence. This is a disclosed evidence-packaging limitation.
- The sanctioned commands used equal 300-second limits and all exited normally.
- The read-only sandbox flag prevented Mothership writes. The sanctioned runs used the user-authorized escalated network path; no Mothership write event was observed.

### Conclusion of the Mothership experiment

The planning-doc answer is Browser Terminal Access, with the readiness-contract criterion quoted above as the next unmet criterion. In the comparison, all three repoctx-assisted and all three no-repoctx baseline trials returned that answer correctly. Usage accounting is available for the emitted input, cached-input, cache-write, output, and reasoning fields; total_tokens was not emitted.
