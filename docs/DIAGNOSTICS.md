# Diagnostic report contract

`repoctx.diagnostics/v1alpha1` is the bounded JSON report emitted by compiler
passes. Validate it with [diagnostics.schema.json](diagnostics.schema.json).
Records are untrusted data. A diagnostic does not grant authority, authenticate
its source, execute a check, establish coverage, or assert verification.

Each record has a stable code, producer severity, bounded pass/capability names,
plain-text message, remediation category, and an optional exact source range.
Locations identify claimed bytes and a SHA-256 value; structural validation does
not verify that the current file matches the digest. Callers must remove denied
content before constructing a report.

The default limits are 1,024 records, 4,096 message bytes, 4,096 path bytes,
and a 1 MiB newline-terminated JSON payload. The hard ceilings are 4,096
records, 8,192 message bytes, 4,096 path bytes, and 8 MiB. Go validation also
checks UTF-8, byte lengths, coordinate agreement, and output bounds.

`Report.SARIF` is a data-only interoperability projection. It uses SARIF
`kind=review` and `level=none`; the producer severity remains in
`properties.repoctx`. It does not emit fixes, commands, invocations, execution
results, or coverage claims, and it preserves byte ranges without guessing
SARIF character columns. The SARIF boundary is documented in the package API;
the report schema above covers the repoctx-native input shape.
