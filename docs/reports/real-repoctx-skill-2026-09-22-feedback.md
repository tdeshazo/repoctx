# Real-repository skill evaluation: actionable feedback

Recorded 2026-09-22 from the external
[no-prefill follow-up report](/home/travis/Workspace/evals/reports/real-repoctx-skill-2026-09-22-v2/REPORT.md).
This feedback reviews that report and the current repository guidance; it does
not independently audit the saved trials. The source link requires the local
evaluation workspace. The findings needed for these actions are retained below.

## Evidence and decision

All four cohorts passed the existing grader on all eight frozen Tornado tasks.
The skill cohort used 5,189,787 total input/output tokens versus 4,038,488 for
vanilla: **28.5% more**, including cached input. Its uncached input was 579,380
versus 492,237 (**17.7% more**), and output was 63,335 versus 61,067
(**3.7% more**). These are usage measures, not API billing estimates.
The prefill and targeted no-prefill cohorts used 37.3% and 18.0% more total
input/output tokens than vanilla, respectively.

The agent read the skill and invoked Repoctx in 8/8 tasks. There were 12 skill
read events, 37 command events, and 39 recognized subcommands: 12 `discover`,
22 `read`, and 5 `search`. Eleven command events failed with requested end lines
beyond EOF; a twelfth failed a downstream public check in a combined command.
Do not count that downstream failure as a Repoctx error. Skill usage under this
explicitly advertised setup establishes exposure and use, not natural adoption.

Keep Repoctx optional, particularly for known-file tasks. Prioritize the recurring
read failure and avoidable retrieval work before adding capabilities or claiming
an efficiency benefit. The skill cohort used fewer total tokens on only two
tasks (`real-workflow-02`, 0.932×; `real-navigate-01`, 0.900×); these are candidates
for trace inspection, not established categories of benefit.

Controls were historical, with one attempt per task. Binary/version, skill,
guidance, prefill, cache state, and run order differed. The reported ratios are
descriptive and cannot isolate the skill's effect. These saturated tasks provide
no evidence of a success-rate gain. Trials used `gpt-5.6-terra` at high reasoning,
a 600-second whole-trial limit, and serial execution. The harness compiled an
index and exposed its path, but supplied no retrieved result; the recognized
commands do not demonstrate use of indexed relationships.

## Prioritized follow-ups

### 1. Make recovery from unknown file length reliable

**Problem:** Eleven EOF range failures recur despite the current
[skill](../../skills/repoctx/SKILL.md) and README documenting `PATH:START:0`.
The earlier [workflow verification](m6-07-completion.md) passed a directed EOF
recovery task; this broader observation limits what that pass establishes.

**Intended behavior:** Inspect the saved failed commands and the exact evaluated
skill/binary first. Reproduce an oversized end line, check the resulting error,
and improve its actionable retry if needed. Put an unknown-end example beside
the normal read example in the skill and CLI guidance if the evaluated version
lacks it. Prefer returned exact spans; when the end is unknown, use
`PATH:START:0` with an explicit output budget. Avoid silently changing inclusive
range semantics merely to hide errors.

**Verification:** Cover oversized ends, empty files, and starts beyond EOF with
focused regressions. Valid through-EOF reads must preserve exact spans and
report truncation under the output budget. On fresh agent tasks with unknown
file length, record guessed endpoints, failed reads, retries, and correctness;
require a usable first retry for any remaining range failure. Keep the original
11 failures visible alongside the new results.

### 2. Identify and remove unnecessary retrieval work

**Problem:** Correctness did not improve, and total usage increased. The largest
skill/vanilla ratio was `real-context-01` at 1.938×; `real-local-02` was 1.716×.
Aggregate tokens and command counts alone cannot explain those increases.

**Intended behavior:** Inspect those two traces and both apparent wins. Attribute
extra turns and returned bytes to EOF recovery, repeated skill reads, repeated
excerpts, discovery after a path is already known, or other observed causes.
Treat these as hypotheses until checked. Reinforce the existing stop rule:
use supplied evidence or a direct known-file read when sufficient, and retrieve
again only to answer an unresolved question. Keep indexed work conditional on
a need for symbols or relationships; measure harness compilation separately.

**Verification:** Produce a per-task trace breakdown before selecting a change.
Evaluate the smallest supported change on fresh tasks, retaining correctness,
cached and uncached input, output, retrieval bytes, follow-up calls, compilation
cost, and time. Predeclare the improvement criterion; do not infer savings from
fewer commands or tune repeatedly against these eight tasks as a held-out set.

### 3. Fix usage accounting before the next comparison

**Problem:** The original live counter found only 3 events because it missed
bare `repoctx` commands on PATH. The corrected completed-event count is 37
commands containing 39 recognized subcommands.

**Intended behavior:** In the external evaluation harness, use one counting path
for live and final reports. Recognize qualified executable paths and PATH
invocations, distinguish commands from subcommands, and separate Repoctx failures
from downstream failures in combined commands. Preserve original logs and label
derived corrections.

**Verification:** Replay the saved completed events and reproduce the report's
37/39 totals, 11 EOF failures, and one downstream failure. Add harness fixtures
for PATH, explicit paths, multiple subcommands, and downstream-check failures.

### 4. Run a controlled comparison after the focused fixes

**Problem:** Historical controls and simultaneous treatment changes prevent
causal attribution; 8/8 baseline success leaves no observed success gap to close.

**Intended behavior:** Freeze fresh tasks and graders before running. Match model,
reasoning, permissions, budgets, and workspace state; pin binary and skill
revisions. Interleave or randomize paired conditions and use repeated trials.
Isolate skill guidance from prefill and index provisioning in separate
comparisons, and record cache behavior. Retain known-file controls and add
harder navigation and relationship tasks with headroom for quality improvement.

**Verification:** Publish all attempts, per-task correctness, usage components,
time, setup/compilation cost, failures, and paired uncertainty. Declare the
decision rule before execution. Expand default usage only if the new evidence
meets that rule; an inconclusive result supports keeping usage optional. This
follow-up does not replace or retroactively satisfy the frozen M4 protocol.
