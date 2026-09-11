package discovery

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

type rule struct {
	base      string
	pattern   *regexp.Regexp
	negate    bool
	directory bool
}

// globRegexp implements slash-separated wildmatch: ** as a whole component
// crosses directories; ordinary * and ? never cross a separator.
func globRegexp(pattern string) (*regexp.Regexp, error) {
	var out strings.Builder
	out.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch c := pattern[i]; c {
		case '\\':
			i++
			if i == len(pattern) {
				return nil, fmt.Errorf("trailing glob escape")
			}
			out.WriteString(regexp.QuoteMeta(pattern[i : i+1]))
		case '*':
			whole := i == 0 || pattern[i-1] == '/'
			if i+1 < len(pattern) && pattern[i+1] == '*' && whole {
				if i+2 == len(pattern) {
					out.WriteString(".*")
					i++
					continue
				}
				if pattern[i+2] == '/' {
					out.WriteString("(?:.*/)?")
					i += 2
					continue
				}
			}
			out.WriteString("[^/]*")
		case '?':
			out.WriteString("[^/]")
		case '[':
			end := i + 1
			if end < len(pattern) && (pattern[end] == '!' || pattern[end] == '^') {
				end++
			}
			for end < len(pattern) && pattern[end] != ']' {
				end++
			}
			if end == len(pattern) {
				return nil, fmt.Errorf("unclosed glob class")
			}
			class := pattern[i+1 : end]
			if strings.Contains(class, "/") {
				return nil, fmt.Errorf("slash in glob class")
			}
			if strings.HasPrefix(class, "!") {
				class = "^" + class[1:]
			}
			if strings.HasPrefix(class, "^") {
				class = "^/" + class[1:]
			}
			out.WriteString("[" + class + "]")
			i = end
		default:
			out.WriteString(regexp.QuoteMeta(pattern[i : i+1]))
		}
	}
	out.WriteString("$")
	return regexp.Compile(out.String())
}

func parseRules(base string, data []byte) ([]rule, error) {
	rules := []rule{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSuffix(line, "\r")
		// Ignore unescaped trailing spaces, preserving escaped literal spaces.
		for strings.HasSuffix(line, " ") {
			backslashes := 0
			for i := len(line) - 2; i >= 0 && line[i] == '\\'; i-- {
				backslashes++
			}
			if backslashes%2 == 1 {
				break
			}
			line = strings.TrimSuffix(line, " ")
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		r := rule{base: base}
		if strings.HasPrefix(line, "!") {
			r.negate = true
			line = line[1:]
		}
		if line == "" {
			continue
		}
		r.directory = strings.HasSuffix(line, "/")
		line = strings.TrimSuffix(line, "/")
		anchored := strings.HasPrefix(line, "/")
		line = strings.TrimPrefix(line, "/")
		if line == "" {
			continue
		}
		if !anchored && !strings.Contains(line, "/") {
			line = "**/" + line
		}
		re, err := globRegexp(line)
		if err != nil {
			return nil, err
		}
		r.pattern = re
		rules = append(rules, r)
	}
	return rules, nil
}

func ignored(p string, directory bool, rules []rule) bool {
	matched := false
	for _, r := range rules {
		rel := p
		if r.base != "." {
			if !strings.HasPrefix(p, r.base+"/") {
				continue
			}
			rel = strings.TrimPrefix(p, r.base+"/")
		}
		if r.directory && !directory {
			continue
		}
		if r.pattern.MatchString(rel) {
			matched = !r.negate
		}
	}
	return matched
}

func prefixMatch(p, prefix string) bool {
	return p == prefix || strings.HasPrefix(p, prefix+"/")
}

func permitted(p string, o Options) bool {
	for _, deny := range o.DenyPaths {
		if prefixMatch(p, deny) {
			return false
		}
	}
	if len(o.AllowPaths) == 0 {
		return true
	}
	for _, allow := range o.AllowPaths {
		if prefixMatch(p, allow) {
			return true
		}
	}
	return false
}

func traversable(p string, o Options) bool {
	if p == "." {
		return true
	}
	for _, deny := range o.DenyPaths {
		if prefixMatch(p, deny) {
			return false
		}
	}
	if permitted(p, o) {
		return true
	}
	for _, allow := range o.AllowPaths {
		if prefixMatch(allow, p) {
			return true
		}
	}
	return false
}

func pathMatches(p string, re *regexp.Regexp, pattern string) bool {
	if re == nil {
		return true
	}
	if !strings.Contains(pattern, "/") {
		p = path.Base(p)
	}
	return re.MatchString(p)
}
