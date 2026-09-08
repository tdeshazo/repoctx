# Mothership: repoctx versus codex exec comparison

Date: 2026-09-08 (America/New_York)

## Result in brief

The authoritative answer is **Browser Terminal Access** (rank 11). Its next unmet criterion is:

> Browser authentication, account linking, session-cookie security, credential
> recovery/revocation, protocol framing, limits, backpressure, origin/proxy
> trust, frontend builds, and the supported-browser matrix are recorded as
> testable contracts before their schema or gateway code lands.

This answer was established from Mothership planning documents before the trials. After explicit user authorization for the bounded planning evidence to reach OpenAI, three assisted and three baseline codex exec trials completed successfully. All six returned the expected answer and emitted JSONL usage events.

## Environment and controls

- Mothership root: /home/travis/Workspace/Mothership.
- Mothership git status --short --untracked-files=all was clean after the attempts; no Mothership file was written.
- Codex: codex-cli 0.153.4.
- Account configuration was copied read-only from /home/travis/.codex to /tmp/mothership-codex-home for isolated runs. The copied configuration selected model gpt-5.6-terra, reasoning effort high, and service tier default; no model override was supplied to any trial.
- Trial flags: --ephemeral --json -s read-only -C /home/travis/Workspace/Mothership. Prompts and JSONL/stderr artifacts were under /tmp.
- The assisted prompt prohibited filesystem, shell, tool, search, and file inspection and required use of supplied evidence only. The baseline prompt permitted read-only inspection of only docs/planning/README.md and docs/planning/roadmap-11-browser-terminal-access.md.

## Question and ground truth

Question supplied to every trial:

> According to Mothership's authoritative planning docs, which roadmap feature is the highest-ranked incomplete item, and what is its next unmet criterion?

AGENTS.md identifies docs/planning/README.md as authoritative. That index marks items 1–10 complete and item 11 incomplete:

- /home/travis/Workspace/Mothership/docs/planning/README.md:20 — [ ] Browser Terminal Access (roadmap-11-browser-terminal-access.md).
- /home/travis/Workspace/Mothership/docs/planning/roadmap-11-browser-terminal-access.md:248-251 — the first unchecked acceptance criterion quoted in the result above.

The expected answer therefore has two parts: feature Browser Terminal Access; next criterion the first unchecked acceptance criterion in that feature file.

## Repoctx preparation and evidence

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

## Sanctioned trial commands

After explicit user authorization for transmitting the bounded planning evidence to OpenAI, exactly three assisted and three baseline trials were run with the same model/configuration, question, output format, read-only Mothership root, and 300-second timeout. The commands were:

~~~sh
timeout 300s env CODEX_HOME=/tmp/mothership-codex-home codex exec --ephemeral --json -s read-only -C /home/travis/Workspace/Mothership < /tmp/mothership-assisted-prompt.txt > /tmp/mothership-sanctioned-assisted-N.jsonl 2> /tmp/mothership-sanctioned-assisted-N.stderr
timeout 300s env CODEX_HOME=/tmp/mothership-codex-home codex exec --ephemeral --json -s read-only -C /home/travis/Workspace/Mothership < /tmp/mothership-baseline-prompt.txt > /tmp/mothership-sanctioned-baseline-N.jsonl 2> /tmp/mothership-sanctioned-baseline-N.stderr
~~~

### Illustrative: repoctx-assisted agent versus baseline

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

## Sanctioned per-trial raw accounting

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

## Sanctioned condition aggregates and correctness

Sums and means below are over the three successful, comparable trials in each condition. Field names are reported exactly as emitted; total_tokens was absent.

| Condition | Trials | input_tokens sum / mean | output_tokens sum / mean | cached_input_tokens sum / mean | reasoning_output_tokens sum / mean | cache_write_input_tokens sum / mean | total_tokens sum / mean | Correct final answers |
|---|---:|---:|---:|---:|---:|---:|---|---:|
| repoctx-assisted | 3 | 51,474 / 17,158.00 | 197 / 65.67 | 25,856 / 8,618.67 | 0 / 0.00 | 0 / 0.00 | not emitted / not emitted | 3/3 |
| no-repoctx baseline | 3 | 113,381 / 37,793.67 | 779 / 259.67 | 84,480 / 28,160.00 | 242 / 80.67 | 0 / 0.00 | not emitted / not emitted | 3/3 |

Both conditions had 100% answer correctness on the six sanctioned trials. The baseline consumed more input because its permitted shell inspection returned the full planning documents, while the assisted prompt supplied a compact bounded evidence excerpt and bundle metadata. This is an intended condition difference, not a claim that the conditions had equal input-token volume.

## Limitations and fairness

- Baseline agents inspected only the two permitted planning files. Their full read-only output made baseline input-token counts larger than assisted counts, so token volume is descriptive rather than a fair efficiency score.
- The assisted condition used the bounded repoctx bundle plus exact source excerpts because the current Markdown renderer's selected heading spans did not include criterion prose. Both excerpts were from the same pre-established files, bounded, and treated as untrusted evidence. This is a disclosed evidence-packaging limitation.
- The sanctioned commands used equal 300-second limits and all exited normally.
- The read-only sandbox flag prevented Mothership writes. The sanctioned runs used the user-authorized escalated network path; no Mothership write event was observed.

## Conclusion

The planning-doc answer is Browser Terminal Access, with the readiness-contract criterion quoted above as the next unmet criterion. In the comparison, all three repoctx-assisted and all three no-repoctx baseline trials returned that answer correctly. Usage accounting is available for the emitted input, cached-input, cache-write, output, and reasoning fields; total_tokens was not emitted.
