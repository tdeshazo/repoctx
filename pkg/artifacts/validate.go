package artifacts

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	namespacePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)
	idPattern        = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}:[a-z][a-z0-9._-]{0,127}$`)
	hashPattern      = regexp.MustCompile(`^[0-9a-f]{64}$`)
	checkPattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]*$`)
)

func oneOf(s string, values ...string) bool {
	for _, v := range values {
		if s == v {
			return true
		}
	}
	return false
}

func validPath(s string, root bool) bool {
	if len(s) == 0 || len(s) > 4096 || !utf8.ValidString(s) {
		return false
	}
	if root && s == "." {
		return true
	}
	for _, c := range s {
		if c < 32 || c == 127 || strings.ContainsRune(`\:*?[]{}!`, c) {
			return false
		}
	}
	for _, part := range strings.Split(s, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func count(path string, n, max int) error {
	if n > max {
		return failure(path, "count limit exceeded")
	}
	return nil
}

// validate is private: callers cannot bypass the byte, syntax or shape guards.
// Its counters also run against isolated in-memory production-ceiling fixtures.
func validate(c *Catalog, l Limits) error {
	if c.Version != Version {
		return failure("$.version", "unsupported version")
	}
	if !namespacePattern.MatchString(c.Namespace) {
		return failure("$.namespace", "invalid namespace")
	}
	if err := count("$.artifacts", len(c.Artifacts), l.Artifacts); err != nil {
		return err
	}
	if err := count("$.relationships", len(c.Relationships), l.Relationships); err != nil {
		return err
	}
	nested := 0
	for i, a := range c.Artifacts {
		p := fmt.Sprintf("$.artifacts[%d]", i)
		if !idPattern.MatchString(a.ID) || !strings.HasPrefix(a.ID, c.Namespace+":") {
			return failure(p+".id", "invalid local identity")
		}
		if !oneOf(a.Kind, "component", "decision", "contract", "requirement", "verification_obligation") {
			return failure(p+".kind", "unknown kind")
		}
		if a.Owner != nil && (len(*a.Owner) == 0 || len(*a.Owner) > 256) {
			return failure(p+".owner", "invalid byte length")
		}
		if !oneOf(a.Lifecycle, "draft", "active", "deprecated", "superseded", "retired") {
			return failure(p+".lifecycle", "unknown lifecycle")
		}
		if a.RunnerCheckID != nil {
			if a.Kind != "verification_obligation" {
				return failure(p+".runner_check_id", "only allowed for obligations")
			}
			if len(*a.RunnerCheckID) > 256 || !checkPattern.MatchString(*a.RunnerCheckID) {
				return failure(p+".runner_check_id", "invalid check ID")
			}
		}
		if err := count(p+".applies_to", len(a.AppliesTo), l.Applicability); err != nil {
			return err
		}
		if err := count(p+".declared_inputs", len(a.DeclaredInputs), l.Inputs); err != nil {
			return err
		}
		if err := spans(a.Sources, p+".sources", l.Sources); err != nil {
			return err
		}
		nested += len(a.AppliesTo) + len(a.DeclaredInputs) + len(a.Sources)
		if err := count("$.nested_entries", nested, l.NestedEntries); err != nil {
			return err
		}
		for j, scope := range a.AppliesTo {
			q := fmt.Sprintf("%s.applies_to[%d]", p, j)
			if !oneOf(scope.Kind, "file", "subtree") {
				return failure(q+".kind", "unknown applicability")
			}
			if !validPath(scope.Path, scope.Kind == "subtree") {
				return failure(q+".path", "invalid path")
			}
		}
		for j, input := range a.DeclaredInputs {
			q := fmt.Sprintf("%s.declared_inputs[%d]", p, j)
			if !validPath(input.Path, false) {
				return failure(q+".path", "invalid path")
			}
			if !oneOf(input.Role, "source", "configuration") {
				return failure(q+".role", "unknown input role")
			}
			if input.SHA256 != nil && !hashPattern.MatchString(*input.SHA256) {
				return failure(q+".sha256", "invalid digest")
			}
		}
	}
	for i, r := range c.Relationships {
		p := fmt.Sprintf("$.relationships[%d]", i)
		if !idPattern.MatchString(r.From) {
			return failure(p+".from", "invalid identity")
		}
		if !idPattern.MatchString(r.To) {
			return failure(p+".to", "invalid identity")
		}
		if !oneOf(r.Kind, "contains", "references", "governed_by", "depends_on", "supersedes", "verifies") {
			return failure(p+".kind", "unknown relationship")
		}
		if r.Resolution != "declared" {
			return failure(p+".resolution", "unsupported resolution")
		}
		if err := spans(r.Sources, p+".sources", l.Sources); err != nil {
			return err
		}
		nested += len(r.Sources)
		if err := count("$.nested_entries", nested, l.NestedEntries); err != nil {
			return err
		}
	}
	return nil
}

func spans(ss []Span, path string, max int) error {
	if len(ss) == 0 {
		return failure(path, "at least one source required")
	}
	if err := count(path, len(ss), max); err != nil {
		return err
	}
	for i, s := range ss {
		p := fmt.Sprintf("%s[%d]", path, i)
		if !validPath(s.Path, false) {
			return failure(p+".path", "invalid path")
		}
		if !hashPattern.MatchString(s.SHA256) {
			return failure(p+".sha256", "invalid digest")
		}
		for _, n := range []int64{s.StartByte, s.EndByte, s.StartLine, s.EndLine, s.StartByteColumn, s.EndByteColumn} {
			if n < 0 || n > 9007199254740991 {
				return failure(p, "integer range exceeded")
			}
		}
		if s.StartByte >= s.EndByte {
			return failure(p, "byte offsets must increase")
		}
		if s.StartLine < 1 || s.EndLine < 1 {
			return failure(p, "lines must be positive")
		}
		if s.StartLine > s.EndLine || (s.StartLine == s.EndLine && s.StartByteColumn >= s.EndByteColumn) {
			return failure(p, "source positions must increase")
		}
	}
	return nil
}
