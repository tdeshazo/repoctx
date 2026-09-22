package agentctx

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

func TestDiscoverSymbolsIsBoundedDeterministicAndExpandable(t *testing.T) {
	root, repo := goFixture(t)
	generation, err := VerifySourceGeneration(repo, SourceGenerationOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	exact, err := DiscoverSymbols(context.Background(), generation, SymbolQuery{Pattern: "Ping"})
	if err != nil {
		t.Fatal(err)
	}
	if len(exact.Symbols) != 1 || exact.Symbols[0].Name != "Ping" || exact.Symbols[0].Path != "main.go" {
		t.Fatalf("exact symbols = %+v", exact.Symbols)
	}
	repeated, err := DiscoverSymbols(context.Background(), generation, SymbolQuery{Pattern: "Ping"})
	if err != nil || !reflect.DeepEqual(exact, repeated) {
		t.Fatal("symbol discovery is not deterministic", err)
	}
	substring, err := DiscoverSymbols(context.Background(), generation, SymbolQuery{
		Pattern: "ping", Mode: "substring", Languages: []string{"go"},
		Kinds: []string{"function"}, PathPrefixes: []string{"main.go"},
	})
	if err != nil || len(substring.Symbols) != 1 || substring.Symbols[0].ID != exact.Symbols[0].ID {
		t.Fatalf("filtered substring = %+v, %v", substring, err)
	}
	regex, err := DiscoverSymbols(context.Background(), generation, SymbolQuery{
		Pattern: "^go:.*#P", Mode: "regex", Limit: 1,
	})
	if err != nil || len(regex.Symbols) != 1 || regex.Omissions.Limit == 0 || !regex.Incomplete {
		t.Fatalf("bounded regex = %+v, %v", regex, err)
	}
	unscanned, err := DiscoverSymbols(context.Background(), generation, SymbolQuery{
		Pattern: ".*", Mode: "regex", Limit: 10, MaxScanned: 1,
	})
	if err != nil || unscanned.Omissions.Unscanned == 0 || !unscanned.Incomplete {
		t.Fatalf("scan bound = %+v, %v", unscanned, err)
	}
	wire, err := json.Marshal(exact)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(wire, []byte(`"text"`)) || bytes.Contains(wire, []byte(`"evidence"`)) {
		t.Fatalf("symbol discovery exposed source payload: %s", wire)
	}
	result, err := BuildFromGeneration(generation, Options{Query: "unused",
		Symbols: []string{exact.Symbols[0].ID}, ExpectedSnapshot: generation.SnapshotID(), MaxBytes: 12000})
	if err != nil || len(result.Bundle.Symbols) == 0 || len(result.Bundle.Evidence) == 0 {
		t.Fatal("discovered ID did not expand through context evidence", err)
	}
}

func TestDiscoverSymbolsRejectsInvalidOrCancelledQueries(t *testing.T) {
	root, repo := goFixture(t)
	generation, err := VerifySourceGeneration(repo, SourceGenerationOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := DiscoverSymbols(ctx, generation, SymbolQuery{Pattern: "Ping"}); err == nil {
		t.Fatal("cancelled discovery succeeded")
	}
	for _, query := range []SymbolQuery{
		{}, {Pattern: "x", Mode: "glob"}, {Pattern: "[", Mode: "regex"},
		{Pattern: "x", Limit: 1001}, {Pattern: "x", MaxScanned: 100001},
		{Pattern: "x", PathPrefixes: []string{"../private"}},
	} {
		if _, err := DiscoverSymbols(context.Background(), generation, query); err == nil {
			t.Fatalf("invalid query accepted: %+v", query)
		}
	}
}

func TestDiscoverSymbolsBoundsExactNameMatches(t *testing.T) {
	root, repo := compileFixture(t, map[string]string{
		"a.go": "package fixture\nfunc Shared() {}\n",
		"b.go": "package fixture\nfunc Shared() {}\n",
	})
	generation, err := VerifySourceGeneration(repo, SourceGenerationOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	result, err := DiscoverSymbols(context.Background(), generation, SymbolQuery{
		Pattern: "Shared", Limit: 10, MaxScanned: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Scanned != 1 || result.Omissions.Unscanned == 0 || !result.Incomplete {
		t.Fatalf("exact scan bound = %+v", result)
	}
}
