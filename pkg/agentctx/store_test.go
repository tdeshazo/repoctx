package agentctx

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestGenerationStoreScopesSwapsEvictsAndRestarts(t *testing.T) {
	rootA, repoA := goFixture(t)
	generationA, err := VerifySourceGeneration(repoA, SourceGenerationOptions{Root: rootA})
	if err != nil {
		t.Fatal(err)
	}
	rootB, repoB := compileFixture(t, map[string]string{"next.go": "package next\nfunc Next() {}\n"})
	generationB, err := VerifySourceGeneration(repoB, SourceGenerationOptions{Root: rootB})
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewGenerationStore(GenerationStoreOptions{MaxGenerations: 1,
		MaxBytes: max(generationA.RetainedBytes(), generationB.RetainedBytes())})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Publish(context.Background(), "tenant-a", generationA); err != nil {
		t.Fatal(err)
	}
	options := Options{Query: "Ping", ExpectedSnapshot: generationA.SnapshotID(), MaxBytes: 12000}
	first, err := store.Build(context.Background(), "tenant-a", options)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Build(context.Background(), "tenant-b", options); !errors.Is(err, ErrGenerationUnavailable) {
		t.Fatal("cross-scope lookup did not fail generically", err)
	}
	held, err := store.lookup(context.Background(), "tenant-a")
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.Publish(cancelled, "tenant-a", generationB); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled publication changed store", err)
	}
	if after, err := store.Build(context.Background(), "tenant-a", options); err != nil || after.Bundle.Snapshot.ID != first.Bundle.Snapshot.ID {
		t.Fatal("cancelled publication replaced active generation", err)
	}
	if err := store.Publish(context.Background(), "tenant-a", generationB); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Build(context.Background(), "tenant-a", options); err == nil {
		t.Fatal("atomic swap served the prior expected snapshot")
	}
	nextOptions := Options{Query: "Next", ExpectedSnapshot: generationB.SnapshotID(), MaxBytes: 12000}
	if next, err := store.Build(context.Background(), "tenant-a", nextOptions); err != nil || next.Bundle.Snapshot.ID != generationB.SnapshotID() {
		t.Fatal("atomic swap did not serve the complete replacement", err)
	}
	if err := store.Publish(context.Background(), "tenant-b", generationB); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Build(context.Background(), "tenant-a", options); !errors.Is(err, ErrGenerationUnavailable) {
		t.Fatal("evicted scope remained visible", err)
	}
	if stats := store.Stats(); stats.Generations != 1 || stats.Scopes != 1 || stats.Evictions != 2 || stats.Bytes > max(generationA.bytes, generationB.bytes) {
		t.Fatalf("unexpected store stats: %+v", stats)
	}
	// A reader that acquired an immutable generation remains valid after eviction.
	if _, err := BuildFromGeneration(held, options); err != nil {
		t.Fatal("eviction invalidated an in-flight generation", err)
	}
	restarted, err := NewGenerationStore(GenerationStoreOptions{MaxGenerations: 1, MaxBytes: generationB.bytes})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.Discover(context.Background(), "tenant-b", SymbolQuery{Pattern: "Next"}); !errors.Is(err, ErrGenerationUnavailable) {
		t.Fatal("new store exposed a prior process generation", err)
	}
}

func TestGenerationStoreBindsAuthorityAndHonorsCancellation(t *testing.T) {
	root, repo := goFixture(t)
	generation, err := VerifySourceGeneration(repo, SourceGenerationOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewGenerationStore(GenerationStoreOptions{MaxGenerations: 2, MaxBytes: generation.bytes * 2})
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range []string{"tenant-a", "tenant-b"} {
		if err := store.Publish(context.Background(), scope, generation); err != nil {
			t.Fatal(err)
		}
	}
	options := Options{Query: "Ping", ExpectedSnapshot: generation.SnapshotID(), MaxBytes: 12000}
	a, err := store.Build(context.Background(), "tenant-a", options)
	if err != nil {
		t.Fatal(err)
	}
	b, err := store.Build(context.Background(), "tenant-b", options)
	if err != nil {
		t.Fatal(err)
	}
	if a.Bundle.TaskID == b.Bundle.TaskID {
		t.Fatal("authority scope omitted from task identity")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.Build(ctx, "tenant-a", options); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled build succeeded", err)
	}
	if _, err := store.Discover(ctx, "tenant-a", SymbolQuery{Pattern: "Ping"}); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled discovery succeeded", err)
	}
	midflight, stop := context.WithCancel(context.Background())
	options.TokenizerID = "test/cancel/v1"
	options.MaxTokens = 12000
	options.CountTokens = func(payload []byte) (int, error) {
		stop()
		return len(payload), nil
	}
	if _, err := store.Build(midflight, "tenant-a", options); !errors.Is(err, context.Canceled) {
		t.Fatal("mid-flight cancellation was not observed", err)
	}
	if err := store.Purge("tenant-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Build(context.Background(), "tenant-a", options); !errors.Is(err, ErrGenerationUnavailable) {
		t.Fatal("purged scope remained visible", err)
	}
}

func TestGenerationStoreConcurrentReadersAndPublication(t *testing.T) {
	root, repo := goFixture(t)
	generation, err := VerifySourceGeneration(repo, SourceGenerationOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewGenerationStore(GenerationStoreOptions{MaxGenerations: 2, MaxBytes: generation.bytes * 2})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Publish(context.Background(), "tenant", generation); err != nil {
		t.Fatal(err)
	}
	options := Options{Query: "Ping", ExpectedSnapshot: generation.SnapshotID(), MaxBytes: 12000}
	var group sync.WaitGroup
	for range 8 {
		group.Add(1)
		go func() {
			defer group.Done()
			for range 10 {
				if _, err := store.Build(context.Background(), "tenant", options); err != nil {
					t.Errorf("concurrent build: %v", err)
				}
				if _, err := store.Discover(context.Background(), "tenant", SymbolQuery{Pattern: "Ping"}); err != nil {
					t.Errorf("concurrent discovery: %v", err)
				}
			}
		}()
	}
	group.Wait()
}

func TestGenerationStoreRejectsUnboundedConfiguration(t *testing.T) {
	if _, err := NewGenerationStore(GenerationStoreOptions{}); err == nil {
		t.Fatal("unbounded store accepted")
	}
	root, repo := goFixture(t)
	generation, err := VerifySourceGeneration(repo, SourceGenerationOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewGenerationStore(GenerationStoreOptions{MaxGenerations: 1, MaxBytes: generation.bytes - 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Publish(context.Background(), "tenant", generation); err == nil {
		t.Fatal("oversized generation accepted")
	}
}
