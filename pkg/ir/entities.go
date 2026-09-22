package ir

import (
	"encoding/hex"
	"fmt"
	"io/fs"
	"regexp"
	"strings"
)

var (
	entityIDPattern        = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}:[a-z][a-z0-9._-]{0,127}$`)
	entityNamespacePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)
)

// EntityVersion identifies the typed semantic layer embedded in repository IR.
const EntityVersion = "repoctx.entities/v1alpha2"

// EntityModel contains repository-authored semantic declarations. Repository
// metadata remains evidence: it does not grant authority or activate behavior.
type EntityModel struct {
	Version       string               `json:"version"`
	Namespace     string               `json:"namespace"`
	Entities      []Entity             `json:"entities"`
	Relationships []EntityRelationship `json:"relationships"`
}

// Entity is a stable repository declaration. IDs and references are qualified
// by EntityModel.Namespace; Source identifies the exact authored declaration.
type Entity struct {
	ID          string      `json:"id"`
	Kind        string      `json:"kind"`
	Owners      []string    `json:"owners"`
	Scopes      []Scope     `json:"scopes"`
	Lifecycle   string      `json:"lifecycle"`
	Supersedes  []string    `json:"supersedes"`
	Sensitivity string      `json:"sensitivity"`
	Freshness   Freshness   `json:"freshness"`
	Declaration Declaration `json:"declaration"`
}

// Scope is a declared file or subtree extent and grants no read permission.
type Scope struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}

// Freshness names meaning-bearing repository inputs. Their declaration does
// not assert that the inputs are current or trusted.
type Freshness struct {
	Inputs []string `json:"inputs"`
}

// Declaration records that an entity is an authored claim and where its exact
// bytes came from. Other declaration states are intentionally unsupported.
type Declaration struct {
	Status  string       `json:"status"`
	Sources []SourceSpan `json:"sources"`
}

// EntityRelationship is an authored semantic edge. It remains a declaration;
// presence in R-CIR does not establish authority or runtime causality.
type EntityRelationship struct {
	From        string      `json:"from"`
	To          string      `json:"to"`
	Kind        string      `json:"kind"`
	Declaration Declaration `json:"declaration"`
}

// SourceSpan uses physical byte coordinates. Endpoints are exclusive, lines
// are one-based, columns are zero-based UTF-8 byte offsets, and SHA256 hashes
// the complete source file.
type SourceSpan struct {
	Path            string `json:"path"`
	SHA256          string `json:"sha256"`
	StartByte       int64  `json:"start_byte"`
	EndByte         int64  `json:"end_byte"`
	StartLine       int64  `json:"start_line"`
	EndLine         int64  `json:"end_line"`
	StartByteColumn int64  `json:"start_byte_column"`
	EndByteColumn   int64  `json:"end_byte_column"`
}

func (r *Repository) validateEntities() error {
	if r.Entities == nil {
		return nil
	}
	model := r.Entities
	if model.Version != EntityVersion || !entityNamespacePattern.MatchString(model.Namespace) {
		return fmt.Errorf("invalid entity model identity")
	}
	if model.Entities == nil || model.Relationships == nil || len(model.Entities) > 1024 || len(model.Relationships) > 4096 {
		return fmt.Errorf("invalid entity collection")
	}
	byID := make(map[string]Entity, len(model.Entities))
	for i, entity := range model.Entities {
		if !entityIDPattern.MatchString(entity.ID) || !strings.HasPrefix(entity.ID, model.Namespace+":") ||
			(i > 0 && entity.ID <= model.Entities[i-1].ID) {
			return fmt.Errorf("entity %d: invalid or noncanonical identity", i)
		}
		if !entityKind(entity.Kind) || entity.Owners == nil || entity.Scopes == nil || entity.Supersedes == nil || entity.Freshness.Inputs == nil {
			return fmt.Errorf("entity %d: invalid fields", i)
		}
		if !uniqueBounded(entity.Owners) || !uniqueBounded(entity.Supersedes) || !uniqueBounded(entity.Freshness.Inputs) || len(entity.Scopes) > 256 {
			return fmt.Errorf("entity %d: noncanonical collection", i)
		}
		if !entityLifecycle(entity.Lifecycle) || !oneOfEntity(entity.Sensitivity, "public", "internal", "restricted") {
			return fmt.Errorf("entity %d: invalid lifecycle or sensitivity", i)
		}
		for _, scope := range entity.Scopes {
			if !oneOfEntity(scope.Kind, "file", "subtree") || !entityPath(scope.Path, scope.Kind == "subtree") {
				return fmt.Errorf("entity %d: invalid scope", i)
			}
		}
		for _, input := range entity.Freshness.Inputs {
			if !entityPath(input, false) {
				return fmt.Errorf("entity %d: invalid freshness input", i)
			}
		}
		if !validDeclaration(entity.Declaration) {
			return fmt.Errorf("entity %d: invalid declaration source", i)
		}
		byID[entity.ID] = entity
	}
	for i, entity := range model.Entities {
		for _, owner := range entity.Owners {
			if target, ok := byID[owner]; !ok || target.Kind != "owner" {
				return fmt.Errorf("entity %d: invalid owner", i)
			}
		}
		for _, prior := range entity.Supersedes {
			if target, ok := byID[prior]; !ok || target.Kind != entity.Kind || prior == entity.ID {
				return fmt.Errorf("entity %d: invalid supersession", i)
			}
		}
	}
	for i, relationship := range model.Relationships {
		if !entityIDPattern.MatchString(relationship.From) || !entityIDPattern.MatchString(relationship.To) ||
			!entityRelationshipKind(relationship.Kind) || !validDeclaration(relationship.Declaration) {
			return fmt.Errorf("entity relationship %d: invalid fields", i)
		}
		if i > 0 && compareEntityRelationship(relationship, model.Relationships[i-1]) < 0 {
			return fmt.Errorf("entity relationship %d: noncanonical order", i)
		}
	}
	return nil
}

func entityKind(value string) bool {
	return oneOfEntity(value, "document", "component", "decision", "contract", "requirement", "verification_obligation", "owner")
}

func entityLifecycle(value string) bool {
	return oneOfEntity(value, "draft", "active", "deprecated", "superseded", "retired")
}

func entityRelationshipKind(value string) bool {
	return oneOfEntity(value, "contains", "references", "governed_by", "depends_on", "supersedes", "verifies")
}

func compareEntityRelationship(a, b EntityRelationship) int {
	if result := strings.Compare(a.From, b.From); result != 0 {
		return result
	}
	if result := strings.Compare(a.Kind, b.Kind); result != 0 {
		return result
	}
	return strings.Compare(a.To, b.To)
}

func oneOfEntity(value string, choices ...string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}

func entityPath(value string, root bool) bool {
	if root && value == "." {
		return true
	}
	return fs.ValidPath(value) && value != "." && !strings.ContainsAny(value, "\\\x00:*")
}

func validEntitySource(source SourceSpan) bool {
	hash, err := hex.DecodeString(source.SHA256)
	return err == nil && len(hash) == 32 && entityPath(source.Path, false) &&
		source.StartByte >= 0 && source.EndByte > source.StartByte &&
		source.StartLine > 0 && source.EndLine >= source.StartLine &&
		source.StartByteColumn >= 0 && source.EndByteColumn >= 0
}

func validDeclaration(declaration Declaration) bool {
	if declaration.Status != "declared" || len(declaration.Sources) == 0 || len(declaration.Sources) > 16 {
		return false
	}
	for _, source := range declaration.Sources {
		if !validEntitySource(source) {
			return false
		}
	}
	return true
}

func uniqueBounded(values []string) bool {
	if len(values) > 256 {
		return false
	}
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if seen[value] {
			return false
		}
		seen[value] = true
	}
	return true
}
