package obligation

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/tdeshazo/repoctx/pkg/agentctx"
	"github.com/tdeshazo/repoctx/pkg/artifacts"
	"github.com/tdeshazo/repoctx/pkg/ir"
)

const (
	defaultMaxArtifacts     = 64
	defaultMaxRelationships = 128
	defaultMaxDiagnostics   = 128
	defaultMaxBytes         = 32768
)

// Build selects caller-authorized declarations whose effective scope contains
// source evidence in context. It performs no source reads and executes no
// runner check. Pass Result.Payload alongside the source context payload.
func Build(
	context *agentctx.Result,
	catalog *artifacts.Catalog,
	authority artifacts.Authority,
	options Options,
) (*Result, error) {
	if context == nil || context.Bundle == nil {
		return nil, fmt.Errorf("building obligations: nil source context")
	}
	if err := context.Bundle.Validate(); err != nil {
		return nil, fmt.Errorf("building obligations: invalid source context: %w", err)
	}
	if err := normalize(&options); err != nil {
		return nil, err
	}
	resolution, err := artifacts.Resolve(catalog, authority)
	if err != nil {
		return nil, fmt.Errorf("building obligations: %w", err)
	}

	evidenceByPath := make(map[string][]agentctx.Evidence, len(context.Bundle.Evidence))
	evidenceIDs := make([]string, 0, len(context.Bundle.Evidence))
	for _, evidence := range context.Bundle.Evidence {
		evidenceByPath[evidence.File] = append(evidenceByPath[evidence.File], evidence)
		evidenceIDs = append(evidenceIDs, evidence.ID)
	}
	sort.Strings(evidenceIDs)

	bundle := &Bundle{
		Version:        Version,
		ID:             "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		SourceTaskID:   context.Bundle.TaskID,
		SourceSnapshot: context.Bundle.Snapshot.ID,
		Evidence:       evidenceIDs,
		Artifacts:      []Artifact{},
		Relationships:  []Relationship{},
		Diagnostics:    []Diagnostic{},
		Capabilities: Capabilities{
			Available:   []string{"applicable_declarations", "opaque_runner_check_ids", "declared_verification_relationships"},
			Unavailable: []string{"runner_registration", "command_execution", "check_results", "passing_status"},
		},
	}
	if ok, err := fits(bundle, options.MaxBytes); err != nil {
		return nil, err
	} else if !ok {
		return nil, fmt.Errorf("building obligations: budget too small for handoff metadata")
	}

	for _, diagnostic := range resolution.Diagnostics {
		if len(bundle.Diagnostics) >= options.MaxDiagnostics {
			bundle.Omissions.Diagnostics++
			continue
		}
		candidate := diagnosticFrom(diagnostic)
		trial := clone(bundle)
		trial.Diagnostics = append(trial.Diagnostics, candidate)
		if ok, err := fits(trial, options.MaxBytes); err != nil {
			return nil, err
		} else if ok {
			bundle = trial
		} else {
			bundle.Omissions.Budget++
		}
	}

	candidates := applicableArtifacts(catalog, resolution, evidenceByPath)
	selected := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		if len(bundle.Artifacts) >= options.MaxArtifacts {
			bundle.Omissions.Artifacts++
			continue
		}
		trial := clone(bundle)
		trial.Artifacts = append(trial.Artifacts, candidate)
		if ok, err := fits(trial, options.MaxBytes); err != nil {
			return nil, err
		} else if ok {
			bundle = trial
			selected[candidate.ID] = true
		} else {
			bundle.Omissions.Budget++
		}
	}

	for _, relationship := range catalog.Relationships {
		if !selected[relationship.From] || !selected[relationship.To] {
			continue
		}
		if len(bundle.Relationships) >= options.MaxRelationships {
			bundle.Omissions.Relationships++
			continue
		}
		candidate := Relationship{
			From: relationship.From, To: relationship.To, Kind: relationship.Kind,
			Resolution: relationship.Resolution,
			Sources:    sourcesFrom(relationship.Sources, evidenceByPath),
		}
		trial := clone(bundle)
		trial.Relationships = append(trial.Relationships, candidate)
		if ok, err := fits(trial, options.MaxBytes); err != nil {
			return nil, err
		} else if ok {
			bundle = trial
		} else {
			bundle.Omissions.Budget++
		}
	}

	bundle.ID, err = ir.ContentID(struct {
		Version, SourceTaskID, SourceSnapshot string
		Artifacts                             []Artifact
		Relationships                         []Relationship
		Diagnostics                           []Diagnostic
		Omissions                             Omissions
	}{
		Version: Version, SourceTaskID: bundle.SourceTaskID,
		SourceSnapshot: bundle.SourceSnapshot,
		Artifacts:      bundle.Artifacts, Relationships: bundle.Relationships,
		Diagnostics: bundle.Diagnostics, Omissions: bundle.Omissions,
	})
	if err != nil {
		return nil, fmt.Errorf("building obligations: identity: %w", err)
	}
	payload, err := render(bundle)
	if err != nil {
		return nil, err
	}
	if len(payload) > options.MaxBytes {
		return nil, fmt.Errorf("building obligations: final payload exceeds budget")
	}
	return &Result{Bundle: bundle, Payload: payload}, nil
}

func normalize(options *Options) error {
	defaults := []struct {
		value    *int
		fallback int
		ceiling  int
		name     string
	}{
		{&options.MaxArtifacts, defaultMaxArtifacts, 1024, "max artifacts"},
		{&options.MaxRelationships, defaultMaxRelationships, 4096, "max relationships"},
		{&options.MaxDiagnostics, defaultMaxDiagnostics, 4096, "max diagnostics"},
		{&options.MaxBytes, defaultMaxBytes, 8 << 20, "max bytes"},
	}
	for _, setting := range defaults {
		if *setting.value == 0 {
			*setting.value = setting.fallback
		}
		if *setting.value < 1 || *setting.value > setting.ceiling {
			return fmt.Errorf("building obligations: %s must be 1..%d", setting.name, setting.ceiling)
		}
	}
	return nil
}

func applicableArtifacts(
	catalog *artifacts.Catalog,
	resolution *artifacts.Resolution,
	evidence map[string][]agentctx.Evidence,
) []Artifact {
	result := []Artifact{}
	for _, effective := range resolution.Artifacts {
		if !appliesToEvidence(effective.Scopes, evidence) {
			continue
		}
		declaration := catalog.Artifacts[effective.DeclarationIndex]
		var runnerCheckID *string
		if declaration.RunnerCheckID != nil {
			value := *declaration.RunnerCheckID
			runnerCheckID = &value
		}
		result = append(result, Artifact{
			ID: effective.ID, Kind: effective.Kind,
			LifecycleClaim: effective.LifecycleClaim,
			Scopes:         append([]artifacts.Applicability{}, effective.Scopes...),
			Sources:        sourcesFrom(declaration.Sources, evidence),
			RunnerCheckID:  runnerCheckID,
		})
	}
	return result
}

func appliesToEvidence(scopes []artifacts.Applicability, evidence map[string][]agentctx.Evidence) bool {
	for _, scope := range scopes {
		for path := range evidence {
			if scopeMatches(scope, path) {
				return true
			}
		}
	}
	return false
}

func scopeMatches(scope artifacts.Applicability, path string) bool {
	if scope.Kind == "file" {
		return path == scope.Path
	}
	return scope.Path == "." || path == scope.Path || strings.HasPrefix(path, scope.Path+"/")
}

func sourcesFrom(spans []artifacts.Span, evidence map[string][]agentctx.Evidence) []Source {
	result := make([]Source, 0, len(spans))
	for _, span := range spans {
		source := Source{Span: span, Verification: "declared_unverified"}
		for _, candidate := range evidence[span.Path] {
			if evidenceCovers(candidate, span) {
				source.EvidenceID = candidate.ID
				source.Verification = "context_exact"
				break
			}
		}
		result = append(result, source)
	}
	return result
}

func evidenceCovers(evidence agentctx.Evidence, span artifacts.Span) bool {
	if evidence.SHA256 != span.SHA256 || evidence.StartByte > int(span.StartByte) || evidence.EndByte < int(span.EndByte) {
		return false
	}
	startLine, startColumn, ok := positionAt(evidence, int(span.StartByte))
	if !ok || int64(startLine) != span.StartLine || int64(startColumn) != span.StartByteColumn {
		return false
	}
	endLine, endColumn, ok := positionAt(evidence, int(span.EndByte))
	return ok && int64(endLine) == span.EndLine && int64(endColumn) == span.EndByteColumn
}

func positionAt(evidence agentctx.Evidence, offset int) (int, int, bool) {
	if offset < evidence.StartByte || offset > evidence.EndByte {
		return 0, 0, false
	}
	line, column := evidence.Span.StartLine, evidence.Span.StartByteColumn
	for _, b := range []byte(evidence.Text[:offset-evidence.StartByte]) {
		if b == '\n' {
			line, column = line+1, 0
		} else {
			column++
		}
	}
	return line, column, true
}

func diagnosticFrom(diagnostic artifacts.Diagnostic) Diagnostic {
	return Diagnostic{
		Code:                diagnostic.Code,
		ArtifactIDs:         append([]string{}, diagnostic.ArtifactIDs...),
		ArtifactIndexes:     append([]int{}, diagnostic.ArtifactIndexes...),
		RelationshipIndexes: append([]int{}, diagnostic.RelationshipIndexes...),
		Path:                diagnostic.Path,
	}
}

func clone(bundle *Bundle) *Bundle {
	result := *bundle
	result.Artifacts = append([]Artifact{}, bundle.Artifacts...)
	result.Relationships = append([]Relationship{}, bundle.Relationships...)
	result.Diagnostics = append([]Diagnostic{}, bundle.Diagnostics...)
	return &result
}

func fits(bundle *Bundle, maxBytes int) (bool, error) {
	payload, err := render(bundle)
	// Reserve room for omission counters finalized after a candidate is rejected.
	return len(payload)+128 <= maxBytes, err
}

func render(bundle *Bundle) ([]byte, error) {
	payload, err := json.Marshal(bundle)
	if err != nil {
		return nil, fmt.Errorf("building obligations: rendering: %w", err)
	}
	return append(payload, '\n'), nil
}
