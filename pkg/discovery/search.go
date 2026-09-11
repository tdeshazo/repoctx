package discovery

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"unicode"
)

type window struct{ start, end int }

func queryTerms(query string) []string {
	terms := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	sort.Strings(terms)
	unique := []string{}
	for _, term := range terms {
		if len(unique) == 0 || unique[len(unique)-1] != term {
			unique = append(unique, term)
		}
	}
	return unique
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
	windows := []window{}
	if e.o.Operation == "read" {
		for _, request := range requests {
			start, end := request.StartLine, request.EndLine
			if start == 0 {
				start = 1
			}
			if end == 0 {
				end = len(starts)
			}
			if start > len(starts) || end > len(starts) {
				return invalid("line range exceeds observed file %q (%d lines)", entry.Path, len(starts))
			}
			windows = append(windows, window{start: start - 1, end: end})
		}
	} else {
		for line, start := range starts {
			if err := e.ctx.Err(); err != nil {
				return err
			}
			end := len(b)
			if line+1 < len(starts) {
				end = starts[line+1]
			}
			text := string(b[start:end])
			hit := false
			if e.search != nil {
				hit = e.search.MatchString(strings.TrimSuffix(text, "\n"))
			} else {
				_, s := lexical(e.terms, "", text)
				hit = s.DistinctTerms > 0
			}
			if hit {
				windows = append(windows, window{
					start: line - min(line, e.o.ContextLines),
					end:   line + 1 + min(len(starts)-line-1, e.o.ContextLines),
				})
			}
		}
		if len(windows) == 0 && score.PathTerms > 0 && e.o.Operation == "discover" {
			windows = append(windows, window{end: 1 + min(len(starts)-1, e.o.ContextLines)})
		}
	}
	digest := sha256.Sum256(b)
	hash := hex.EncodeToString(digest[:])
	for _, w := range mergeWindows(windows) {
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
		e.add(Result{
			Entry: entry, MatchedTerms: matched, Score: score,
			Evidence: &Evidence{
				SHA256: hash, StartByte: start, EndByte: end, StartLine: w.start + 1, EndLine: w.end,
				Text: text,
			},
		})
	}
	return nil
}
