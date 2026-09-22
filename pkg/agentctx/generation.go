package agentctx

import (
	"encoding/json"
	"fmt"
	"sort"
	"unsafe"

	"github.com/tdeshazo/repoctx/pkg/ir"
)

const maxGenerationIndexBytes = 128 << 20

// SourceGenerationOptions bounds the one-time verified capture used for
// repeated context requests.
type SourceGenerationOptions struct {
	Root           string
	MaxSourceBytes int64
	MaxReadBytes   int64
	MaxIndexBytes  int64
}

// VerifiedSourceGeneration owns an immutable index copy and exact source bytes
// that passed strict verified-local loading together. Its fields are private so
// callers cannot mutate one request's evidence underneath another request.
type VerifiedSourceGeneration struct {
	repository *ir.Repository
	sources    map[int]*source
	symbols    []int
	names      []int
	snapshotID string
	sourceID   string
	profileID  string
	bytes      int64
}

// VerifySourceGeneration captures a bounded generation after strict local
// inventory and source verification. The caller remains responsible for
// authenticating r and authorizing its recorded scope.
func VerifySourceGeneration(r *ir.Repository, o SourceGenerationOptions) (*VerifiedSourceGeneration, error) {
	if r == nil {
		return nil, fmt.Errorf("repository index is required")
	}
	if o.Root == "" {
		return nil, fmt.Errorf("explicit source root is required")
	}
	if o.MaxSourceBytes == 0 {
		o.MaxSourceBytes = 2 << 20
	}
	if o.MaxReadBytes == 0 {
		o.MaxReadBytes = 256 << 20
	}
	if o.MaxIndexBytes == 0 {
		o.MaxIndexBytes = maxGenerationIndexBytes
	}
	if o.MaxSourceBytes < 1 || o.MaxReadBytes < 1 ||
		o.MaxIndexBytes < 1 || o.MaxIndexBytes > maxGenerationIndexBytes {
		return nil, fmt.Errorf("generation resource limits must be positive and index bytes at most %d", maxGenerationIndexBytes)
	}
	if err := r.Validate(); err != nil {
		return nil, fmt.Errorf("invalid index: %w", err)
	}
	if r.Version != "repoctx.ir/v1alpha4" {
		return nil, fmt.Errorf("source generation requires v1alpha4 compilation-input manifests; recompile this index")
	}

	encoded, err := json.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("copy repository index: %w", err)
	}
	if int64(len(encoded)) > o.MaxIndexBytes {
		return nil, fmt.Errorf("repository index exceeds generation byte limit")
	}
	var owned ir.Repository
	if err := json.Unmarshal(encoded, &owned); err != nil {
		return nil, fmt.Errorf("copy repository index: %w", err)
	}
	if err := owned.Validate(); err != nil {
		return nil, fmt.Errorf("copied index invariant: %w", err)
	}
	snapshotID, err := owned.SnapshotID()
	if err != nil {
		return nil, err
	}
	sourceID, err := owned.Inputs.SourceID()
	if err != nil {
		return nil, err
	}
	profileID, err := owned.Inputs.ProfileID()
	if err != nil {
		return nil, err
	}
	sources, err := loadSources(&owned, Options{
		Root: o.Root, MaxSourceBytes: o.MaxSourceBytes, MaxReadBytes: o.MaxReadBytes,
		Consistency: "verified-local",
	})
	if err != nil {
		return nil, err
	}
	if len(sources) != len(owned.Files) {
		return nil, fmt.Errorf("verified generation omitted indexed sources")
	}
	symbols, names := buildSymbolIndex(&owned)
	retainedBytes := int64(len(encoded)) + estimateSourceBytes(sources) +
		int64(cap(symbols))*int64(unsafe.Sizeof(int(0))) +
		int64(cap(names))*int64(unsafe.Sizeof(int(0)))
	return &VerifiedSourceGeneration{repository: &owned, sources: sources,
		symbols: symbols, names: names, snapshotID: snapshotID, sourceID: sourceID,
		profileID: profileID, bytes: retainedBytes}, nil
}

func buildSymbolIndex(repo *ir.Repository) ([]int, []int) {
	symbols := make([]int, len(repo.Symbols))
	for symbolIndex := range repo.Symbols {
		symbols[symbolIndex] = symbolIndex
	}
	sort.Slice(symbols, func(i, j int) bool {
		return repo.String(repo.Symbols[symbols[i]].ID) < repo.String(repo.Symbols[symbols[j]].ID)
	})
	names := append([]int(nil), symbols...)
	sort.Slice(names, func(i, j int) bool {
		left, right := repo.Symbols[names[i]], repo.Symbols[names[j]]
		leftName, rightName := repo.String(left.Name), repo.String(right.Name)
		if leftName != rightName {
			return leftName < rightName
		}
		return repo.String(left.ID) < repo.String(right.ID)
	})
	return symbols, names
}

func estimateSourceBytes(sources map[int]*source) int64 {
	var total int64
	for _, source := range sources {
		total += int64(cap(source.data))
		total += int64(cap(source.lines)) * int64(unsafe.Sizeof(int(0)))
	}
	return total
}

// SnapshotID returns the canonical index identity callers must authenticate and
// pass as Options.ExpectedSnapshot when reusing the generation.
func (g *VerifiedSourceGeneration) SnapshotID() string {
	if g == nil {
		return ""
	}
	return g.snapshotID
}

// SourceID returns the exact compilation-input identity retained by g.
func (g *VerifiedSourceGeneration) SourceID() string {
	if g == nil {
		return ""
	}
	return g.sourceID
}

// ProfileID returns the compilation/profile identity retained by g.
func (g *VerifiedSourceGeneration) ProfileID() string {
	if g == nil {
		return ""
	}
	return g.profileID
}

// RetainedBytes returns the deterministic byte charge used by GenerationStore.
// It includes canonical index bytes, retained sources/line maps, and compact
// symbol indexes. It is a cache budget metric, not a Go heap or RSS measurement.
func (g *VerifiedSourceGeneration) RetainedBytes() int64 {
	if g == nil {
		return 0
	}
	return g.bytes
}
