package agentctx

import (
	"encoding/json"
	"fmt"

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
	snapshotID string
	sourceID   string
	profileID  string
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
	return &VerifiedSourceGeneration{repository: &owned, sources: sources,
		snapshotID: snapshotID, sourceID: sourceID, profileID: profileID}, nil
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
