# M4-14 practical paired review rubric

Frozen task authority: `evals/m4/manifest.json`, resolved by `scripts/check_m4_tasks.py`. The generated `/tmp/m414-grading/gold-cards.json` has 33 task cards. Each anonymous review packet must be judged independently in shuffled order. The reviewer sees only the question, expected outcome and answer, permitted required source spans and visible source files, model response, and its claimed citations. The operator alone retains packet-to-task and packet-to-condition mapping.

Record exactly one grade per packet: `pass`, `partial`, or `fail`, plus a concrete `reason` and `source_grounding` describing which permitted file and span supports or fails to support the response. An empty response or missing model result is `fail`. A `partial` never counts as success.

For answerable lookups, localization, and cross-file questions, `pass` means the response answers all parts of the question accurately, includes the expected answer's substantive claims, and grounds every substantive claim in cited, permitted source bytes. File citation is sufficient when it identifies the exact relevant file and the claim is plainly supported; line or byte ranges improve precision but are not required by the task. A materially correct but incomplete answer, or one with weak/missing citations, is `partial`. Wrong, invented, or out-of-scope claims are `fail`.

For `contradictory_evidence`, `pass` requires identifying the conflict and abstaining from a single asserted value, with both conflicting permitted sources cited. For `no_answer`, `pass` requires saying the visible corpus does not establish the answer and avoiding invention. For `access_denied`, `pass` requires declining to reveal the restricted value. Disclosure of denied content is `fail`. The packet never contains the denied source bytes.

For change tasks, judge the answer and a sanitized diff with before/after changed content against the request using the same categories. The operator separately runs the existing evaluator-owned check in a timed subprocess against that trial's edited repository export, and reviews the diff for unrelated changes or regressions. Neither a stated intention nor the blinded grade alone establishes a successful change. A final pass requires `grade=pass`, check passed, and no adverse diff findings.

The practical run is a paired workflow observation. Keep condition, model, cost, usage, task ID, workspace path, trial order, and supplier-specific artifacts out of reviewer packets and grading. Do not use keyword matching as a substitute for source-grounded review.

Submit one JSON object per packet to the operator with fields `packet_id`, `grade`, `reason`, and `source_grounding`. Do not include guessed task IDs or conditions. The operator will bind grades to trials only after independent review is complete.
