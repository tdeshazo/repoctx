package agentctx

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// ErrGenerationUnavailable reports that a scope has no active retained generation.
// It deliberately does not reveal whether another scope contains the snapshot.
var ErrGenerationUnavailable = errors.New("verified generation unavailable")

// GenerationStoreOptions defines hard count and deterministic retained-byte
// bounds for a GenerationStore.
type GenerationStoreOptions struct {
	MaxGenerations int
	MaxBytes       int64
}

// GenerationStoreStats is a point-in-time view of bounded store occupancy and
// completed LRU evictions.
type GenerationStoreStats struct {
	Generations int
	Scopes      int
	Bytes       int64
	Evictions   uint64
}

type generationEntry struct {
	key        string
	scope      string
	generation *VerifiedSourceGeneration
}

// GenerationStore is a bounded, authorization-scoped LRU for immutable source
// generations. It starts empty after every process restart and spawns no
// goroutines. In-flight callers retain a safe immutable pointer after eviction.
type GenerationStore struct {
	mu         sync.Mutex
	maxEntries int
	maxBytes   int64
	bytes      int64
	evictions  uint64
	lru        *list.List
	entries    map[string]*list.Element
	active     map[string]string
}

// NewGenerationStore creates an empty process-local store with mandatory hard
// bounds. It does not restore generations from an earlier process.
func NewGenerationStore(options GenerationStoreOptions) (*GenerationStore, error) {
	if options.MaxGenerations < 1 || options.MaxGenerations > 10000 || options.MaxBytes < 1 {
		return nil, fmt.Errorf("generation store requires positive bounded count and bytes")
	}
	return &GenerationStore{maxEntries: options.MaxGenerations, maxBytes: options.MaxBytes,
		lru: list.New(), entries: map[string]*list.Element{}, active: map[string]string{}}, nil
}

// Publish atomically installs and activates a complete generation for scope.
// Cancelled publication leaves the prior active generation unchanged.
func (store *GenerationStore) Publish(ctx context.Context, scope string, generation *VerifiedSourceGeneration) error {
	if store == nil || ctx == nil {
		return fmt.Errorf("generation store and context are required")
	}
	if err := validateAuthorityScope(scope); err != nil {
		return err
	}
	if generation == nil {
		return fmt.Errorf("verified source generation is required")
	}
	if generation.bytes > store.maxBytes {
		return fmt.Errorf("generation exceeds store byte limit")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	key := generationKey(scope, generation.snapshotID)
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if element, ok := store.entries[key]; ok {
		store.lru.MoveToFront(element)
		store.active[scope] = key
		return nil
	}
	entry := &generationEntry{key: key, scope: scope, generation: generation}
	element := store.lru.PushFront(entry)
	store.entries[key] = element
	store.bytes += generation.bytes
	store.active[scope] = key
	for len(store.entries) > store.maxEntries || store.bytes > store.maxBytes {
		victim := store.lru.Back()
		if victim == nil || victim == element {
			store.removeLocked(element)
			return fmt.Errorf("generation cannot fit bounded store")
		}
		store.removeLocked(victim)
		store.evictions++
	}
	return nil
}

// Build expands evidence from the active generation authorized for scope.
func (store *GenerationStore) Build(ctx context.Context, scope string, options Options) (*Result, error) {
	generation, err := store.lookup(ctx, scope)
	if err != nil {
		return nil, err
	}
	options.authorityScope = scope
	return BuildFromGenerationContext(ctx, generation, options)
}

// Discover returns compact symbol metadata from the active generation
// authorized for scope.
func (store *GenerationStore) Discover(ctx context.Context, scope string, query SymbolQuery) (*SymbolDiscovery, error) {
	generation, err := store.lookup(ctx, scope)
	if err != nil {
		return nil, err
	}
	return DiscoverSymbols(ctx, generation, query)
}

// Purge removes every retained generation associated with scope. Readers that
// already acquired an immutable generation can finish safely.
func (store *GenerationStore) Purge(scope string) error {
	if store == nil {
		return fmt.Errorf("generation store is required")
	}
	if err := validateAuthorityScope(scope); err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, element := range store.entries {
		if element.Value.(*generationEntry).scope == scope {
			store.removeLocked(element)
		}
	}
	delete(store.active, scope)
	return nil
}

// Stats returns current occupancy without exposing scope or snapshot identities.
func (store *GenerationStore) Stats() GenerationStoreStats {
	if store == nil {
		return GenerationStoreStats{}
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	return GenerationStoreStats{Generations: len(store.entries), Scopes: len(store.active),
		Bytes: store.bytes, Evictions: store.evictions}
}

func (store *GenerationStore) lookup(ctx context.Context, scope string) (*VerifiedSourceGeneration, error) {
	if store == nil || ctx == nil {
		return nil, fmt.Errorf("generation store and context are required")
	}
	if err := validateAuthorityScope(scope); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	key, ok := store.active[scope]
	if !ok {
		return nil, ErrGenerationUnavailable
	}
	element, ok := store.entries[key]
	if !ok {
		return nil, ErrGenerationUnavailable
	}
	store.lru.MoveToFront(element)
	return element.Value.(*generationEntry).generation, nil
}

func (store *GenerationStore) removeLocked(element *list.Element) {
	entry := element.Value.(*generationEntry)
	delete(store.entries, entry.key)
	if store.active[entry.scope] == entry.key {
		delete(store.active, entry.scope)
	}
	store.bytes -= entry.generation.bytes
	store.lru.Remove(element)
}

func validateAuthorityScope(scope string) error {
	if strings.TrimSpace(scope) == "" || len(scope) > 256 || strings.ContainsAny(scope, "\x00\r\n") {
		return fmt.Errorf("authority scope must contain 1..256 non-control bytes")
	}
	return nil
}

func generationKey(scope, snapshot string) string { return scope + "\x00" + snapshot }
