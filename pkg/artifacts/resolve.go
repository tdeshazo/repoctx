package artifacts

import (
	"fmt"
	"sort"
	"strings"
)

// Resolve intersects structurally valid repository declarations with trusted
// caller authority. It performs no I/O, executes no checks, and never treats a
// lifecycle or supersession claim as authority.
func Resolve(catalog *Catalog, authority Authority) (*Resolution, error) {
	if catalog == nil {
		return nil, fmt.Errorf("resolving artifacts: nil catalog")
	}
	if err := validate(catalog, ceilings()); err != nil {
		return nil, fmt.Errorf("resolving artifacts: invalid catalog: %w", err)
	}
	accepted, err := validateAuthority(catalog.Namespace, authority)
	if err != nil {
		return nil, err
	}

	byID := make(map[string][]int, len(catalog.Artifacts))
	for i, artifact := range catalog.Artifacts {
		byID[artifact.ID] = append(byID[artifact.ID], i)
	}

	diagnostics := duplicateDiagnostics(byID)
	diagnostics = append(diagnostics, referenceDiagnostics(catalog, byID)...)
	diagnostics = append(diagnostics, supersessionCycleDiagnostics(catalog, byID)...)

	candidates := make(map[string]EffectiveArtifact, len(accepted))
	for id := range accepted {
		indexes := byID[id]
		if len(indexes) == 0 {
			diagnostics = append(diagnostics, Diagnostic{Code: "accepted_id_missing", ArtifactIDs: []string{id}})
			continue
		}
		if len(indexes) != 1 {
			continue
		}
		index := indexes[0]
		artifact := catalog.Artifacts[index]
		scopes := intersectScopes(artifact.AppliesTo, authority.Scopes)
		if len(scopes) == 0 {
			diagnostics = append(diagnostics, Diagnostic{
				Code:            "invalid_effective_scope",
				ArtifactIDs:     []string{id},
				ArtifactIndexes: []int{index},
			})
			continue
		}
		candidates[id] = EffectiveArtifact{
			ID:               id,
			Kind:             artifact.Kind,
			LifecycleClaim:   artifact.Lifecycle,
			DeclarationIndex: index,
			Scopes:           scopes,
		}
	}

	blocked := map[string]bool{}
	conflicts := acceptedRequirementConflicts(catalog, candidates, blocked)
	diagnostics = append(diagnostics, conflicts...)
	diagnostics = append(diagnostics, acceptedSupersessionConflicts(catalog, candidates, blocked)...)

	ids := make([]string, 0, len(candidates))
	for id := range candidates {
		if !blocked[id] {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	artifacts := make([]EffectiveArtifact, 0, len(ids))
	for _, id := range ids {
		artifacts = append(artifacts, candidates[id])
	}
	sortDiagnostics(diagnostics)
	return &Resolution{Artifacts: artifacts, Diagnostics: diagnostics}, nil
}

func validateAuthority(namespace string, authority Authority) (map[string]bool, error) {
	if len(authority.AcceptedIDs) > ceilings().Artifacts || len(authority.Scopes) > ceilings().Applicability {
		return nil, fmt.Errorf("resolving artifacts: authority limit exceeded")
	}
	accepted := make(map[string]bool, len(authority.AcceptedIDs))
	for _, id := range authority.AcceptedIDs {
		if !idPattern.MatchString(id) || !strings.HasPrefix(id, namespace+":") {
			return nil, fmt.Errorf("resolving artifacts: invalid accepted id")
		}
		if accepted[id] {
			return nil, fmt.Errorf("resolving artifacts: duplicate accepted id")
		}
		accepted[id] = true
	}
	for _, scope := range authority.Scopes {
		if !oneOf(scope.Kind, "file", "subtree") || !validPath(scope.Path, scope.Kind == "subtree") {
			return nil, fmt.Errorf("resolving artifacts: invalid authority scope")
		}
	}
	return accepted, nil
}

func duplicateDiagnostics(byID map[string][]int) []Diagnostic {
	ids := sortedKeys(byID)
	diagnostics := []Diagnostic{}
	for _, id := range ids {
		if len(byID[id]) > 1 {
			diagnostics = append(diagnostics, Diagnostic{
				Code:            "duplicate_id",
				ArtifactIDs:     []string{id},
				ArtifactIndexes: append([]int(nil), byID[id]...),
			})
		}
	}
	return diagnostics
}

func referenceDiagnostics(catalog *Catalog, byID map[string][]int) []Diagnostic {
	diagnostics := []Diagnostic{}
	for i, relationship := range catalog.Relationships {
		for _, id := range []string{relationship.From, relationship.To} {
			indexes := byID[id]
			code := ""
			switch len(indexes) {
			case 0:
				code = "broken_reference"
			case 1:
				continue
			default:
				code = "ambiguous_reference"
			}
			diagnostics = append(diagnostics, Diagnostic{
				Code:                code,
				ArtifactIDs:         []string{id},
				ArtifactIndexes:     append([]int(nil), indexes...),
				RelationshipIndexes: []int{i},
			})
		}
	}
	return diagnostics
}

func intersectScopes(claimed, allowed []Applicability) []Applicability {
	seen := map[Applicability]bool{}
	result := []Applicability{}
	for _, claim := range claimed {
		for _, allow := range allowed {
			intersection, ok := intersectScope(claim, allow)
			if ok && !seen[intersection] {
				seen[intersection] = true
				result = append(result, intersection)
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Path != result[j].Path {
			return result[i].Path < result[j].Path
		}
		return result[i].Kind < result[j].Kind
	})
	return result
}

func intersectScope(a, b Applicability) (Applicability, bool) {
	if a.Kind == "file" && b.Kind == "file" {
		return a, a.Path == b.Path
	}
	if a.Kind == "file" {
		return a, scopeContains(b, a.Path)
	}
	if b.Kind == "file" {
		return b, scopeContains(a, b.Path)
	}
	if scopeContains(a, b.Path) {
		return b, true
	}
	if scopeContains(b, a.Path) {
		return a, true
	}
	return Applicability{}, false
}

func scopeContains(scope Applicability, path string) bool {
	if scope.Kind == "file" {
		return scope.Path == path
	}
	if scope.Path == "." {
		return true
	}
	return path == scope.Path || strings.HasPrefix(path, scope.Path+"/")
}

func scopesOverlap(a, b []Applicability) bool {
	return len(intersectScopes(a, b)) > 0
}

func acceptedRequirementConflicts(
	catalog *Catalog,
	candidates map[string]EffectiveArtifact,
	blocked map[string]bool,
) []Diagnostic {
	ids := sortedKeys(candidates)
	diagnostics := []Diagnostic{}
	for i, leftID := range ids {
		leftEffective := candidates[leftID]
		left := catalog.Artifacts[leftEffective.DeclarationIndex]
		if left.Kind != "requirement" {
			continue
		}
		for j := i; j < len(ids); j++ {
			rightID := ids[j]
			rightEffective := candidates[rightID]
			right := catalog.Artifacts[rightEffective.DeclarationIndex]
			if right.Kind != "requirement" || !scopesOverlap(leftEffective.Scopes, rightEffective.Scopes) {
				continue
			}
			for _, path := range conflictingInputPaths(left.DeclaredInputs, right.DeclaredInputs, leftID == rightID) {
				blocked[leftID] = true
				blocked[rightID] = true
				diagnostics = append(diagnostics, Diagnostic{
					Code:            "conflicting_accepted_requirements",
					ArtifactIDs:     []string{leftID, rightID},
					ArtifactIndexes: []int{leftEffective.DeclarationIndex, rightEffective.DeclarationIndex},
					Path:            path,
				})
			}
		}
	}
	return diagnostics
}

func conflictingInputPaths(left, right []Input, sameArtifact bool) []string {
	paths := map[string]bool{}
	for i, a := range left {
		start := 0
		if sameArtifact {
			start = i + 1
		}
		for j := start; j < len(right); j++ {
			b := right[j]
			if a.Path == b.Path && inputsConflict(a, b) {
				paths[a.Path] = true
			}
		}
	}
	return sortedKeys(paths)
}

func inputsConflict(a, b Input) bool {
	if a.Role != b.Role || a.Required != b.Required {
		return true
	}
	return a.SHA256 != nil && b.SHA256 != nil && *a.SHA256 != *b.SHA256
}

func acceptedSupersessionConflicts(
	catalog *Catalog,
	candidates map[string]EffectiveArtifact,
	blocked map[string]bool,
) []Diagnostic {
	diagnostics := []Diagnostic{}
	for i, relationship := range catalog.Relationships {
		if relationship.Kind != "supersedes" {
			continue
		}
		from, fromOK := candidates[relationship.From]
		to, toOK := candidates[relationship.To]
		if !fromOK || !toOK || !scopesOverlap(from.Scopes, to.Scopes) {
			continue
		}
		blocked[relationship.From] = true
		blocked[relationship.To] = true
		diagnostics = append(diagnostics, Diagnostic{
			Code:                "accepted_supersession_conflict",
			ArtifactIDs:         []string{relationship.From, relationship.To},
			ArtifactIndexes:     []int{from.DeclarationIndex, to.DeclarationIndex},
			RelationshipIndexes: []int{i},
		})
	}
	return diagnostics
}

func supersessionCycleDiagnostics(catalog *Catalog, byID map[string][]int) []Diagnostic {
	graph := map[string][]string{}
	for _, relationship := range catalog.Relationships {
		if relationship.Kind != "supersedes" || len(byID[relationship.From]) != 1 || len(byID[relationship.To]) != 1 {
			continue
		}
		graph[relationship.From] = append(graph[relationship.From], relationship.To)
		if graph[relationship.To] == nil {
			graph[relationship.To] = []string{}
		}
	}
	components := stronglyConnected(graph)
	diagnostics := []Diagnostic{}
	for _, component := range components {
		isCycle := len(component) > 1
		if len(component) == 1 {
			for _, to := range graph[component[0]] {
				isCycle = isCycle || to == component[0]
			}
		}
		if isCycle {
			indexes := make([]int, 0, len(component))
			for _, id := range component {
				indexes = append(indexes, byID[id][0])
			}
			diagnostics = append(diagnostics, Diagnostic{
				Code:            "supersession_cycle",
				ArtifactIDs:     component,
				ArtifactIndexes: indexes,
			})
		}
	}
	return diagnostics
}

func stronglyConnected(graph map[string][]string) [][]string {
	index := 0
	indexes := map[string]int{}
	low := map[string]int{}
	onStack := map[string]bool{}
	stack := []string{}
	components := [][]string{}
	var visit func(string)
	visit = func(v string) {
		indexes[v] = index
		low[v] = index
		index++
		stack = append(stack, v)
		onStack[v] = true
		neighbors := append([]string(nil), graph[v]...)
		sort.Strings(neighbors)
		for _, w := range neighbors {
			if _, seen := indexes[w]; !seen {
				visit(w)
				low[v] = min(low[v], low[w])
			} else if onStack[w] {
				low[v] = min(low[v], indexes[w])
			}
		}
		if low[v] != indexes[v] {
			return
		}
		component := []string{}
		for {
			last := len(stack) - 1
			w := stack[last]
			stack = stack[:last]
			onStack[w] = false
			component = append(component, w)
			if w == v {
				break
			}
		}
		sort.Strings(component)
		components = append(components, component)
	}
	for _, v := range sortedKeys(graph) {
		if _, seen := indexes[v]; !seen {
			visit(v)
		}
	}
	return components
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortDiagnostics(diagnostics []Diagnostic) {
	sort.SliceStable(diagnostics, func(i, j int) bool {
		left := diagnosticKey(diagnostics[i])
		right := diagnosticKey(diagnostics[j])
		return left < right
	})
}

func diagnosticKey(d Diagnostic) string {
	return fmt.Sprintf("%s\x00%s\x00%v\x00%v\x00%s", d.Code, strings.Join(d.ArtifactIDs, "\x00"), d.ArtifactIndexes, d.RelationshipIndexes, d.Path)
}
