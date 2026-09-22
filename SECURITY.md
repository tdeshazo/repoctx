# Security policy

## Supported versions

repoctx is in active pre-1.0 development. Only the current release and current
contracts are supported; historical schemas and development builds do not receive
separate security maintenance. See [the compatibility policy](docs/COMPATIBILITY.md).

## Report a vulnerability

Use GitHub's private security-advisory form for this repository:
<https://github.com/tdeshazo/repoctx/security/advisories/new>.

Do not include exploit details, sensitive repository content, credentials, or
private source in a public issue. Include the affected repoctx release and
contract versions, platform, minimal reproduction, impact, and whether untrusted
repository input is required. If the advisory form is unavailable, contact the
repository owner privately before public disclosure. No response or remediation
deadline is promised in advance.

## Security boundary

repoctx parses untrusted repository content and returns it as evidence. It does
not make that content safe to execute or promote it to instructions. Callers own
executable authentication, filesystem scope, approvals, sandboxing, secrets,
network access, artifact authority, and result verification. A source hash or
valid schema establishes consistency, not authenticity.

Reports about bypassing documented path/symlink controls, cross-scope cache
disclosure, parser/resource exhaustion beyond stated bounds, or unsafe handling
of malformed indexes are in scope. Prompt-injection resistance, complete language
semantics, and isolation supplied by an external agent harness are not claimed
repoctx capabilities, though a repoctx defect that violates its documented data
boundary remains in scope.
