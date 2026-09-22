package artifacts

import (
	"slices"

	"github.com/tdeshazo/repoctx/pkg/ir"
)

// Entities adapts deterministic artifact authoring into canonical R-CIR
// declarations. The standalone catalog is a derived view of the same input,
// never a second semantic source.
func Entities(root string, data []byte, permit func(string) bool) (*ir.EntityModel, error) {
	compiled, err := Compile(root, data, Limits{}, PathPermit(permit))
	if err != nil {
		return nil, err
	}
	model := &ir.EntityModel{
		Version: ir.EntityVersion, Namespace: compiled.Catalog.Namespace,
		Entities: []ir.Entity{}, Relationships: []ir.EntityRelationship{},
	}
	supersedes := make(map[string][]string)
	for _, relationship := range compiled.Catalog.Relationships {
		if relationship.Kind == "supersedes" {
			supersedes[relationship.From] = append(supersedes[relationship.From], relationship.To)
		}
	}
	for i, artifact := range compiled.Catalog.Artifacts {
		authored := compiled.Authoring.Artifacts[i]
		owners := []string{}
		prior := []string{}
		if authored.Owner != nil {
			owners = append(owners, *authored.Owner)
		}
		prior = append(prior, supersedes[artifact.ID]...)
		inputs := make([]string, 0, len(artifact.DeclaredInputs))
		seenInputs := make(map[string]bool, len(artifact.DeclaredInputs))
		for _, input := range artifact.DeclaredInputs {
			if seenInputs[input.Path] {
				continue
			}
			inputs = append(inputs, input.Path)
			seenInputs[input.Path] = true
		}
		model.Entities = append(model.Entities, ir.Entity{
			ID: artifact.ID, Kind: artifact.Kind, Owners: owners,
			Scopes: scopesToIR(artifact.AppliesTo), Lifecycle: artifact.Lifecycle,
			Supersedes: slices.Clone(prior), Sensitivity: authored.Sensitivity,
			Freshness:   ir.Freshness{Inputs: inputs},
			Declaration: ir.Declaration{Status: "declared", Sources: spansToIR(artifact.Sources)},
		})
	}
	for _, relationship := range compiled.Catalog.Relationships {
		model.Relationships = append(model.Relationships, ir.EntityRelationship{
			From: relationship.From, To: relationship.To, Kind: relationship.Kind,
			Declaration: ir.Declaration{Status: "declared", Sources: spansToIR(relationship.Sources)},
		})
	}
	return model, nil
}

func scopesToIR(scopes []Applicability) []ir.Scope {
	result := make([]ir.Scope, len(scopes))
	for i, scope := range scopes {
		result[i] = ir.Scope{Kind: scope.Kind, Path: scope.Path}
	}
	return result
}

func spansToIR(spans []Span) []ir.SourceSpan {
	result := make([]ir.SourceSpan, len(spans))
	for i, span := range spans {
		result[i] = ir.SourceSpan{
			Path: span.Path, SHA256: span.SHA256,
			StartByte: span.StartByte, EndByte: span.EndByte,
			StartLine: span.StartLine, EndLine: span.EndLine,
			StartByteColumn: span.StartByteColumn, EndByteColumn: span.EndByteColumn,
		}
	}
	return result
}
