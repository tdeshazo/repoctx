package obligation

import (
	"fmt"

	"github.com/tdeshazo/repoctx/pkg/agentctx"
	"github.com/tdeshazo/repoctx/pkg/artifacts"
	"github.com/tdeshazo/repoctx/pkg/ir"
)

// BuildFromRepository builds obligations from the canonical entities in the
// same repository snapshot as context. The caller must authenticate that
// snapshot and supply authority; repository declarations cannot grant it.
//
// Components, decisions, contracts, requirements and verification obligations
// use Build's authority resolution and resource limits. Document and owner
// entities are not planning artifacts. Declaration sources retain Build's
// context_exact or declared_unverified provenance; no extra paths are read,
// freshness is not asserted, and no runner IDs or commands are inferred.
// Freshness paths remain required source-input declarations without digests.
func BuildFromRepository(
	context *agentctx.Result,
	repository *ir.Repository,
	authority artifacts.Authority,
	options Options,
) (*Result, error) {
	if context == nil || context.Bundle == nil {
		return nil, fmt.Errorf("building obligations: nil source context")
	}
	if err := repository.Validate(); err != nil {
		return nil, fmt.Errorf("building obligations: invalid repository: %w", err)
	}
	if repository.Entities == nil {
		return nil, fmt.Errorf("building obligations: repository has no canonical entities")
	}
	snapshot, err := repository.SnapshotID()
	if err != nil {
		return nil, fmt.Errorf("building obligations: repository identity: %w", err)
	}
	if snapshot != context.Bundle.Snapshot.ID {
		return nil, fmt.Errorf("building obligations: repository snapshot does not match source context")
	}
	catalog, err := planningCatalog(repository.Entities, authority.AcceptedIDs)
	if err != nil {
		return nil, err
	}
	return Build(context, catalog, authority, options)
}

func planningCatalog(model *ir.EntityModel, acceptedIDs []string) (*artifacts.Catalog, error) {
	catalog := &artifacts.Catalog{
		Version: artifacts.Version, Namespace: model.Namespace,
		Artifacts: []artifacts.Artifact{}, Relationships: []artifacts.Relationship{},
	}
	excluded := map[string]bool{}
	for _, entity := range model.Entities {
		if entity.Kind == "document" || entity.Kind == "owner" {
			excluded[entity.ID] = true
			continue
		}
		scopes := make([]artifacts.Applicability, 0, len(entity.Scopes))
		for _, scope := range entity.Scopes {
			scopes = append(scopes, artifacts.Applicability{Kind: scope.Kind, Path: scope.Path})
		}
		inputs := make([]artifacts.Input, 0, len(entity.Freshness.Inputs))
		for _, path := range entity.Freshness.Inputs {
			inputs = append(inputs, artifacts.Input{Path: path, Role: "source", Required: true})
		}
		catalog.Artifacts = append(catalog.Artifacts, artifacts.Artifact{
			ID: entity.ID, Kind: entity.Kind, Lifecycle: entity.Lifecycle,
			AppliesTo: scopes, Sources: declarationSources(entity.Declaration),
			DeclaredInputs: inputs,
		})
	}
	for _, id := range acceptedIDs {
		if excluded[id] {
			return nil, fmt.Errorf("building obligations: entity %q is not a planning artifact", id)
		}
	}
	// Preserve unresolved references for the authority resolver's diagnostics,
	// but omit edges to the known document/owner kinds outside this projection.
	type edge struct{ from, to, kind string }
	explicit := map[edge]bool{}
	for _, relationship := range model.Relationships {
		if excluded[relationship.From] || excluded[relationship.To] {
			continue
		}
		explicit[edge{from: relationship.From, to: relationship.To, kind: relationship.Kind}] = true
		catalog.Relationships = append(catalog.Relationships, artifacts.Relationship{
			From: relationship.From, To: relationship.To, Kind: relationship.Kind,
			Resolution: "declared", Sources: declarationSources(relationship.Declaration),
		})
	}
	// Supersedes is also an entity field. Retain it as a declared edge so the
	// resolver blocks overlapping accepted claims instead of choosing a winner.
	for _, entity := range model.Entities {
		if excluded[entity.ID] {
			continue
		}
		for _, prior := range entity.Supersedes {
			if explicit[edge{from: entity.ID, to: prior, kind: "supersedes"}] {
				continue
			}
			if len(catalog.Relationships) >= 4096 {
				return nil, fmt.Errorf("building obligations: canonical planning relationship limit exceeded")
			}
			catalog.Relationships = append(catalog.Relationships, artifacts.Relationship{
				From: entity.ID, To: prior, Kind: "supersedes",
				Resolution: "declared", Sources: declarationSources(entity.Declaration),
			})
		}
	}
	return catalog, nil
}

func declarationSources(declaration ir.Declaration) []artifacts.Span {
	sources := make([]artifacts.Span, 0, len(declaration.Sources))
	for _, source := range declaration.Sources {
		sources = append(sources, artifacts.Span(source))
	}
	return sources
}
