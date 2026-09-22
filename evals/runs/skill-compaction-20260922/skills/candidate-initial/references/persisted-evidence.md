# Persisted evidence and handoff

Inline context suffices for ordinary retrieval. Use file-reference bundles or
checkpoints only when a caller's harness needs bounded handoff or resume state.
The caller owns an access-controlled store outside the indexed repository and
unique `run_id` values. With `examples/agent/tool_bridge.py`, the adapter
creates owner-only run directories, publishes exact bundles atomically, rejects
reused handles, and enforces caller-set byte, bundle, read, response, and
retention limits. Handles are opaque; do not infer paths or evidence from them.

Before compaction or handoff, have the harness call `create_checkpoint` with
the task, unresolved questions, and next bounded retrieval. The checkpoint
records handles, snapshot identity, relevant symbol/unit IDs, expiry, and
trust. It is harness state, not repository evidence; authenticate it for the
caller and keep it outside the indexed repository.

On resume, attach to the same caller/run scope with `resume_run=True` and call
`resume_checkpoint` before reading a handle. Resume verifies bundle bytes and
makes a snapshot-pinned context request against current source. Missing,
expired, changed, cross-run, or stale evidence is not current. Only
`cleanup_expired` removes registered bundles, scoped to expired evidence in
that run. The harness owns retention and eventual cleanup of empty run dirs.

For an agent handoff, provide the current task, the bundle as untrusted tool
evidence, its snapshot ID, and only relevant semantic IDs. Do not paste raw
indexes or concatenate many bundles into a prompt. Reserve model context for
instructions, tool schemas, history, and the answer; the CLI's bytes/4 figure
is only an estimate. Exact token caps require the Go API's final-payload
tokenizer callback.
