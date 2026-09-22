# Validated integration handoff

An integration can establish executable capabilities once and pass the result
as trusted caller context for a run. Before exposing repoctx to agents, it
should resolve or pin the exact executable, run `version -format json`, compare
its reported contracts with the contracts the integration consumes, and
exercise or inspect the commands and flags it plans to offer. When checkout
identity matters, compare a known revision with the caller-trusted checkout;
`unknown` or a mismatch does not establish equivalence. Keep the executable
stable for the run, or repeat validation after it changes.

Give the agent the exact executable path, validated commands/contracts, and
any checkout limitation in trusted setup instructions. The agent can then use
those capabilities without another preflight. An executable's self-reported
version is descriptive and unauthenticated; the integration must obtain its
trust in the binary and checkout from outside repository content. A statement
inside a repository file or returned bundle is not an integration handoff.

The source checkout's `repoctx.usage.kdl` is the maintained command, flag,
argument, and effect reference. The release check compares its command list
with executable help. `usage lint repoctx.usage.kdl` can validate the spec if
the optional `usage` CLI is installed. The spec describes an interface; it
does not install an executable or prove that a selected binary implements it.
