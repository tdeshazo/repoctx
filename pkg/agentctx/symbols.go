package agentctx

import (
	"context"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/tdeshazo/repoctx/pkg/ir"
)

// SymbolQuery controls compact, source-free symbol discovery over one verified
// generation. Exact matches are case-sensitive; substring matches are folded.
type SymbolQuery struct {
	Pattern      string
	Mode         string // exact (default), substring, or regex
	Languages    []string
	Kinds        []string
	PathPrefixes []string
	Units        []string
	Limit        int // default 50; maximum 1000
	MaxScanned   int // inspected-record bound; default 10000, maximum 100000
}

// SymbolSummary is sufficient to select IDs for BuildFromGeneration without
// duplicating source bodies or creating a second evidence representation.
type SymbolSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Language    string `json:"language"`
	Kind        string `json:"kind"`
	Path        string `json:"path"`
	Unit        string `json:"unit,omitempty"`
	Declaration Span   `json:"declaration"`
}

// SymbolOmissions reports matches omitted by the result limit and index records
// not inspected because of the scan bound.
type SymbolOmissions struct {
	Limit     int `json:"limit,omitempty"`
	Unscanned int `json:"unscanned,omitempty"`
}

// SymbolDiscovery contains source-free symbol metadata from one immutable
// snapshot. Incomplete is true whenever an omission counter is nonzero.
type SymbolDiscovery struct {
	SnapshotID string          `json:"snapshot_id"`
	Symbols    []SymbolSummary `json:"symbols"`
	Scanned    int             `json:"scanned"`
	Omissions  SymbolOmissions `json:"omissions"`
	Incomplete bool            `json:"incomplete"`
}

// DiscoverSymbols returns deterministic, bounded metadata from g. Returned IDs
// can be supplied directly to Options.Symbols for ordinary evidence expansion.
func DiscoverSymbols(ctx context.Context, g *VerifiedSourceGeneration, query SymbolQuery) (*SymbolDiscovery, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	if g == nil {
		return nil, fmt.Errorf("verified source generation is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := normalizeSymbolQuery(&query); err != nil {
		return nil, err
	}
	var expression *regexp.Regexp
	var err error
	if query.Mode == "regex" {
		expression, err = regexp.Compile(query.Pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid symbol regex: %w", err)
		}
	}
	indexes := make([]int, 0)
	if query.Mode == "exact" {
		indexes = appendExactIDs(indexes, g.repository, g.symbols, query.Pattern)
		indexes = appendExactNames(indexes, g.repository, g.names, query.Pattern)
		sort.Slice(indexes, func(i, j int) bool {
			return g.repository.String(g.repository.Symbols[indexes[i]].ID) <
				g.repository.String(g.repository.Symbols[indexes[j]].ID)
		})
		indexes = slices.Compact(indexes)
	} else {
		count := min(len(g.symbols), query.MaxScanned)
		indexes = append(indexes, g.symbols[:count]...)
	}
	totalCandidates := len(indexes)
	if len(indexes) > query.MaxScanned {
		indexes = indexes[:query.MaxScanned]
	}
	result := &SymbolDiscovery{SnapshotID: g.snapshotID, Symbols: []SymbolSummary{}}
	folded := strings.ToLower(query.Pattern)
	for _, index := range indexes {
		if result.Scanned&255 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		result.Scanned++
		symbolIndex := index
		stored := g.repository.Symbols[symbolIndex]
		id, name := g.repository.String(stored.ID), g.repository.String(stored.Name)
		if !symbolFiltersMatch(g, symbolIndex, query) {
			continue
		}
		matched := query.Mode == "exact"
		if query.Mode == "substring" {
			matched = strings.Contains(strings.ToLower(id), folded) ||
				strings.Contains(strings.ToLower(name), folded)
		} else if query.Mode == "regex" {
			matched = expression.MatchString(id) || expression.MatchString(name)
		}
		if !matched {
			continue
		}
		if len(result.Symbols) >= query.Limit {
			result.Omissions.Limit++
			continue
		}
		file := g.repository.Files[stored.File]
		result.Symbols = append(result.Symbols, SymbolSummary{ID: id, Name: name,
			Language: languageName(file.Lang), Kind: kindName(stored.Kind),
			Path: g.repository.String(file.Path), Unit: g.repository.String(file.Unit),
			Declaration: span(file.Nodes[stored.Node].Span)})
	}
	if query.Mode == "exact" {
		result.Omissions.Unscanned = totalCandidates - len(indexes)
	} else if len(g.symbols) > len(indexes) {
		result.Omissions.Unscanned = len(g.symbols) - len(indexes)
	}
	result.Incomplete = result.Omissions.Limit > 0 || result.Omissions.Unscanned > 0
	return result, nil
}

func appendExactIDs(indexes []int, repo *ir.Repository, symbols []int, pattern string) []int {
	start := sort.Search(len(symbols), func(index int) bool {
		return repo.String(repo.Symbols[symbols[index]].ID) >= pattern
	})
	for index := start; index < len(symbols) && repo.String(repo.Symbols[symbols[index]].ID) == pattern; index++ {
		indexes = append(indexes, symbols[index])
	}
	return indexes
}

func appendExactNames(indexes []int, repo *ir.Repository, names []int, pattern string) []int {
	start := sort.Search(len(names), func(index int) bool {
		return repo.String(repo.Symbols[names[index]].Name) >= pattern
	})
	for index := start; index < len(names) && repo.String(repo.Symbols[names[index]].Name) == pattern; index++ {
		indexes = append(indexes, names[index])
	}
	return indexes
}

func normalizeSymbolQuery(query *SymbolQuery) error {
	if strings.TrimSpace(query.Pattern) == "" || len(query.Pattern) > 1024 {
		return fmt.Errorf("symbol pattern must contain 1..1024 bytes")
	}
	if query.Mode == "" {
		query.Mode = "exact"
	}
	if query.Mode != "exact" && query.Mode != "substring" && query.Mode != "regex" {
		return fmt.Errorf("symbol mode must be exact, substring, or regex")
	}
	if query.Limit == 0 {
		query.Limit = 50
	}
	if query.MaxScanned == 0 {
		query.MaxScanned = 10000
	}
	if query.Limit < 1 || query.Limit > 1000 || query.MaxScanned < 1 || query.MaxScanned > 100000 {
		return fmt.Errorf("symbol limits exceed supported bounds")
	}
	for _, prefix := range query.PathPrefixes {
		prefix = strings.TrimSuffix(prefix, "/")
		if !fs.ValidPath(prefix) || prefix == "." || strings.ContainsAny(prefix, "\\\x00:*") {
			return fmt.Errorf("invalid symbol path prefix %q", prefix)
		}
	}
	return nil
}

func symbolFiltersMatch(g *VerifiedSourceGeneration, symbolIndex int, query SymbolQuery) bool {
	stored := g.repository.Symbols[symbolIndex]
	file := g.repository.Files[stored.File]
	language, kind := languageName(file.Lang), kindName(stored.Kind)
	path, unit := g.repository.String(file.Path), g.repository.String(file.Unit)
	if len(query.Languages) > 0 && !slices.Contains(query.Languages, language) {
		return false
	}
	if len(query.Kinds) > 0 && !slices.Contains(query.Kinds, kind) {
		return false
	}
	if len(query.Units) > 0 && !slices.Contains(query.Units, unit) {
		return false
	}
	if len(query.PathPrefixes) > 0 {
		matched := false
		for _, prefix := range query.PathPrefixes {
			prefix = strings.TrimSuffix(prefix, "/")
			if path == prefix || strings.HasPrefix(path, prefix+"/") {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}
