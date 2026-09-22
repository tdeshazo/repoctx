package discovery

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type window struct{ start, end int }

type discoveryWindow struct {
	window
	line int
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func readRangeLabel(request ReadRequest) string {
	if request.StartLine == 0 && request.EndLine == 0 {
		return request.Path
	}
	return request.Path + ":" + strconv.Itoa(request.StartLine) + ":" + strconv.Itoa(request.EndLine)
}

func observedLineCount(count int) string {
	if count == 1 {
		return "1 line"
	}
	return strconv.Itoa(count) + " lines"
}

func queryTerms(query string) []string {
	terms := []string{}
	for _, field := range strings.Fields(strings.ToLower(query)) {
		field = strings.TrimFunc(field, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		})
		if isIdentifier(field) {
			terms = append(terms, field)
			continue
		}
		terms = append(terms, strings.FieldsFunc(field, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		})...)
	}
	sort.Strings(terms)
	unique := []string{}
	for _, term := range terms {
		if len(unique) == 0 || unique[len(unique)-1] != term {
			unique = append(unique, term)
		}
	}
	return unique
}

func isIdentifier(field string) bool {
	if field == "" || !strings.ContainsFunc(field, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		return false
	}
	return strings.ContainsFunc(field, unicode.IsDigit) || strings.ContainsAny(field, "._/:#")
}

// queryIdentifiers retains punctuated query fields such as M4-12 as indivisible
// ranking and window-selection signals. Ordinary punctuation at the edge of a
// word is ignored, and plain words continue through the lexical term path.
func queryIdentifiers(query string) []string {
	identifiers := []string{}
	for _, field := range strings.Fields(strings.ToLower(query)) {
		field = strings.TrimFunc(field, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		})
		if !isIdentifier(field) {
			continue
		}
		identifiers = append(identifiers, field)
	}
	sort.Strings(identifiers)
	return identifiers
}

func identifierMatches(identifiers []string, path, text string) int {
	path, text = strings.ToLower(path), strings.ToLower(text)
	matches := 0
	for _, identifier := range identifiers {
		identifier = strings.ToLower(identifier)
		if containsIdentifier(path, identifier) || containsIdentifier(text, identifier) {
			matches++
		}
	}
	return matches
}

// containsIdentifier finds an identifier that is not embedded in a larger
// letter/digit sequence. Punctuation remains a boundary so identifiers retain
// useful matches in prose, dotted symbols, and paths such as docs/M4-07.md.
func containsIdentifier(value, identifier string) bool {
	if identifier == "" {
		return false
	}
	for offset := 0; ; {
		index := strings.Index(value[offset:], identifier)
		if index < 0 {
			return false
		}
		start := offset + index
		end := start + len(identifier)
		if identifierBoundaryBefore(value, start) && identifierBoundaryAfter(value, end) {
			return true
		}
		_, size := utf8.DecodeRuneInString(value[start:])
		offset = start + size
	}
}

func identifierBoundaryBefore(value string, offset int) bool {
	if offset == 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(value[:offset])
	return !unicode.IsLetter(r) && !unicode.IsDigit(r)
}

func identifierBoundaryAfter(value string, offset int) bool {
	if offset == len(value) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(value[offset:])
	return !unicode.IsLetter(r) && !unicode.IsDigit(r)
}

func lexical(terms []string, p, text string) ([]string, Score) {
	matched := []string{}
	score := Score{}
	p, text = strings.ToLower(p), strings.ToLower(text)
	for _, term := range terms {
		inPath := strings.Contains(p, term)
		count := strings.Count(text, term)
		if inPath || count > 0 {
			matched = append(matched, term)
			score.DistinctTerms++
		}
		if inPath {
			score.PathTerms++
		}
		score.Occurrences += min(count, 10)
	}
	return matched, score
}

func lineStarts(b []byte) []int {
	starts := []int{0}
	for i, c := range b {
		if c == '\n' && i+1 < len(b) {
			starts = append(starts, i+1)
		}
	}
	return starts
}

func mergeWindows(windows []window) []window {
	sort.Slice(windows, func(i, j int) bool { return windows[i].start < windows[j].start })
	merged := []window{}
	for _, w := range windows {
		if len(merged) > 0 && w.start <= merged[len(merged)-1].end {
			merged[len(merged)-1].end = max(merged[len(merged)-1].end, w.end)
		} else {
			merged = append(merged, w)
		}
	}
	return merged
}

// mergeDiscoveryWindows prevents nearby weak term matches from chaining a
// query-centered excerpt into most of a file. Overlapping context still merges
// when the union fits the size of one ordinary context window.
func mergeDiscoveryWindows(windows []window, maxLines int) []window {
	sort.Slice(windows, func(i, j int) bool {
		if windows[i].start != windows[j].start {
			return windows[i].start < windows[j].start
		}
		return windows[i].end < windows[j].end
	})
	merged := []window{}
	for _, candidate := range windows {
		if len(merged) == 0 {
			merged = append(merged, candidate)
			continue
		}
		last := &merged[len(merged)-1]
		if candidate.start == last.start && candidate.end == last.end {
			continue
		}
		if candidate.start <= last.end && max(last.end, candidate.end)-last.start <= maxLines {
			last.end = max(last.end, candidate.end)
			continue
		}
		merged = append(merged, candidate)
	}
	return merged
}

// lexicalWindowsOutside returns lexical candidates whose hit lines are not
// covered by sorted identifier windows. Both inputs are produced in line order.
func lexicalWindowsOutside(candidates []discoveryWindow, identifiers []window) []window {
	windows := []window{}
	identifier := 0
	for _, candidate := range candidates {
		for identifier < len(identifiers) && identifiers[identifier].end <= candidate.line {
			identifier++
		}
		if identifier == len(identifiers) || candidate.line < identifiers[identifier].start {
			windows = append(windows, candidate.window)
		}
	}
	return windows
}

func (e *engine) inspect(entry Entry) error {
	requests := e.requested[entry.Path]
	if e.o.Operation == "read" && len(requests) == 0 {
		return nil
	}
	if e.o.Operation == "files" || e.o.Operation == "overview" {
		return nil
	}
	if e.o.Operation == "discover" && e.o.Query == "" {
		return nil
	}
	if !pathMatches(entry.Path, e.glob, e.o.Glob) {
		return nil
	}
	e.found[entry.Path] = true
	matched, score := lexical(e.terms, entry.Path, "")
	b, reason := e.read(entry.Path)
	if reason != "" {
		e.omit(entry.Path, reason)
		if e.o.Operation == "discover" && score.PathTerms > 0 {
			e.add(Result{Entry: entry, MatchedTerms: matched, Score: score})
		}
		return nil
	}
	starts := lineStarts(b)
	metadataRankValue := 0
	if e.o.Operation == "discover" && e.o.Query != "" {
		metadataRankValue = metadataRank(b)
	}
	windows := []window{}
	if e.o.Operation == "read" {
		invalidRange := false
		for _, request := range requests {
			start, end := request.StartLine, request.EndLine
			if start == 0 {
				start = 1
			}
			if end == 0 {
				end = len(starts)
			}
			if start > len(starts) {
				e.rangeErrors[request] = "requested start line " + strconv.Itoa(start) +
					" for " + shellQuote(readRangeLabel(request)) +
					" exceeds observed file " + strconv.Quote(entry.Path) +
					" (" + observedLineCount(len(starts)) + "); choose a start line from 1 through " +
					strconv.Itoa(len(starts))
				invalidRange = true
				continue
			}
			if end > len(starts) {
				followUp := entry.Path + ":" + strconv.Itoa(start) + ":0"
				e.rangeErrors[request] = "requested end line " + strconv.Itoa(end) +
					" for " + shellQuote(readRangeLabel(request)) +
					" exceeds observed file " + strconv.Quote(entry.Path) +
					" (" + observedLineCount(len(starts)) + "); retry with -file " +
					shellQuote(followUp) + " to read through EOF"
				invalidRange = true
				continue
			}
			windows = append(windows, window{start: start - 1, end: end})
		}
		if invalidRange {
			return nil
		}
	} else {
		identifierFocused := e.o.Operation == "discover" && len(e.identifiers) > 0
		identifierWindows := []discoveryWindow{}
		lexicalWindows := []discoveryWindow{}
		for line, start := range starts {
			if err := e.ctx.Err(); err != nil {
				return err
			}
			end := len(b)
			if line+1 < len(starts) {
				end = starts[line+1]
			}
			text := string(b[start:end])
			lexicalHit := false
			identifierHit := false
			if e.search != nil {
				lexicalHit = e.search.MatchString(strings.TrimSuffix(text, "\n"))
			} else {
				_, s := lexical(e.terms, "", text)
				lexicalHit = s.DistinctTerms > 0
				if identifierFocused {
					identifierHit = identifierMatches(e.identifiers, "", text) > 0
				}
			}
			if lexicalHit || identifierHit {
				candidate := discoveryWindow{window: window{
					start: line - min(line, e.o.ContextLines),
					end:   line + 1 + min(len(starts)-line-1, e.o.ContextLines),
				}, line: line}
				if identifierHit {
					identifierWindows = append(identifierWindows, candidate)
				} else {
					lexicalWindows = append(lexicalWindows, candidate)
				}
			}
		}
		if e.o.Operation == "discover" {
			for _, candidate := range identifierWindows {
				windows = append(windows, candidate.window)
			}
			selectedIdentifiers := mergeDiscoveryWindows(windows, 2*e.o.ContextLines+1)
			windows = append(windows, lexicalWindowsOutside(lexicalWindows, selectedIdentifiers)...)
		} else {
			for _, candidate := range lexicalWindows {
				windows = append(windows, candidate.window)
			}
		}
		if len(windows) == 0 && score.PathTerms > 0 && e.o.Operation == "discover" {
			windows = append(windows, window{end: 1 + min(len(starts)-1, e.o.ContextLines)})
		}
	}
	digest := sha256.Sum256(b)
	hash := hex.EncodeToString(digest[:])
	selectedWindows := mergeWindows(windows)
	if e.o.Operation == "discover" {
		selectedWindows = mergeDiscoveryWindows(windows, 2*e.o.ContextLines+1)
	}
	for _, w := range selectedWindows {
		start, end := starts[w.start], len(b)
		if w.end < len(starts) {
			end = starts[w.end]
		}
		text := string(b[start:end])
		matched, score := lexical(e.terms, entry.Path, text)
		if e.o.Operation == "search" {
			matched = []string{e.o.Query}
			score = Score{}
		}
		if e.o.Operation == "read" {
			matched = []string{}
			score = Score{}
		}
		result := Result{
			Entry: entry, MatchedTerms: matched, Score: score,
			Evidence: &Evidence{
				SHA256: hash, StartByte: start, EndByte: end, StartLine: w.start + 1, EndLine: w.end,
				Text: text,
			},
		}
		if e.o.Operation == "discover" && e.o.Query != "" {
			result.metadataRank = metadataRankValue
			result.identifierMatches = identifierMatches(e.identifiers, entry.Path, text)
		}
		e.add(result)
	}
	return nil
}
