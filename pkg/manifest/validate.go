package manifest

import "fmt"

var supportedCapabilities = map[string]bool{
	"artifact_declarations": true,
	"bounded_context":       true,
	"document_links":        true,
	"go_imports":            true,
	"obligation_handoff":    true,
	"syntax_graph":          true,
}

func validate(doc *Manifest, limits Limits) error {
	if doc.Version != Version {
		return fail("$.version", "unsupported version")
	}
	if !namespacePattern.MatchString(doc.Namespace) {
		return fail("$.namespace", "invalid namespace")
	}
	if doc.SourceRoots == nil || doc.ArtifactSources == nil || doc.Components == nil ||
		doc.ProviderInputs == nil || doc.DerivedViews == nil || doc.Capabilities == nil {
		return fail("$", "required array missing")
	}
	if len(doc.SourceRoots) == 0 {
		return fail("$.source_roots", "at least one source root is required")
	}
	if len(doc.Components) == 0 {
		return fail("$.components", "at least one component is required")
	}
	counts := []struct {
		path string
		got  int
		max  int
	}{
		{"$.source_roots", len(doc.SourceRoots), limits.SourceRoots},
		{"$.artifact_sources", len(doc.ArtifactSources), limits.ArtifactSources},
		{"$.components", len(doc.Components), limits.Components},
		{"$.provider_inputs", len(doc.ProviderInputs), limits.ProviderInputs},
		{"$.derived_views", len(doc.DerivedViews), limits.DerivedViews},
		{"$.capabilities", len(doc.Capabilities), limits.Capabilities},
	}
	for _, count := range counts {
		if count.got > count.max {
			return fail(count.path, "count limit exceeded")
		}
	}

	identities := make(map[string]bool)
	roots := make(map[string]bool)
	artifacts := make(map[string]bool)
	providers := make(map[string]bool)
	components := make(map[string]bool)
	for i, source := range doc.SourceRoots {
		path := fmt.Sprintf("$.source_roots[%d]", i)
		if err := identity(identities, roots, source.ID, path+".id"); err != nil {
			return err
		}
		if !validPath(source.Path, true) {
			return fail(path+".path", "invalid path")
		}
	}
	for i, source := range doc.ArtifactSources {
		path := fmt.Sprintf("$.artifact_sources[%d]", i)
		if err := identity(identities, artifacts, source.ID, path+".id"); err != nil {
			return err
		}
		if !validPath(source.Path, false) {
			return fail(path+".path", "invalid path")
		}
		if source.Format != "repoctx.artifact-authoring/v1alpha1" {
			return fail(path+".format", "unsupported artifact source format")
		}
	}
	for i, input := range doc.ProviderInputs {
		path := fmt.Sprintf("$.provider_inputs[%d]", i)
		if err := identity(identities, providers, input.ID, path+".id"); err != nil {
			return err
		}
		if !providerPattern.MatchString(input.Provider) ||
			(input.Provider != "repoctx.document-links/v1" && input.Provider != "repoctx.go-imports/v1") {
			return fail(path+".provider", "unsupported provider")
		}
		if !validPath(input.Path, false) {
			return fail(path+".path", "invalid path")
		}
	}
	for i, component := range doc.Components {
		path := fmt.Sprintf("$.components[%d]", i)
		if err := identity(identities, components, component.ID, path+".id"); err != nil {
			return err
		}
		if len(component.SourceRoots) == 0 {
			return fail(path+".source_roots", "at least one source root is required")
		}
		if err := references(component.SourceRoots, roots, path+".source_roots", "source root"); err != nil {
			return err
		}
		if err := references(component.ArtifactSources, artifacts, path+".artifact_sources", "artifact source"); err != nil {
			return err
		}
		if err := references(component.ProviderInputs, providers, path+".provider_inputs", "provider input"); err != nil {
			return err
		}
	}
	for i, view := range doc.DerivedViews {
		path := fmt.Sprintf("$.derived_views[%d]", i)
		if err := identity(identities, make(map[string]bool), view.ID, path+".id"); err != nil {
			return err
		}
		if !oneOf(view.Kind, "repository_ir", "artifact_catalog", "agent_context", "obligation_handoff") {
			return fail(path+".kind", "unsupported derived view")
		}
		if len(view.Components) == 0 {
			return fail(path+".components", "at least one component is required")
		}
		if err := references(view.Components, components, path+".components", "component"); err != nil {
			return err
		}
	}
	seenCapabilities := make(map[string]bool, len(doc.Capabilities))
	for i, capability := range doc.Capabilities {
		path := fmt.Sprintf("$.capabilities[%d]", i)
		if !supportedCapabilities[capability] {
			return fail(path, "unsupported capability")
		}
		if seenCapabilities[capability] {
			return fail(path, "duplicate capability")
		}
		seenCapabilities[capability] = true
	}
	if len(doc.ArtifactSources) > 0 && !seenCapabilities["artifact_declarations"] {
		return fail("$.capabilities", "artifact source capability missing")
	}
	for i, input := range doc.ProviderInputs {
		capability := map[string]string{
			"repoctx.document-links/v1": "document_links",
			"repoctx.go-imports/v1":     "go_imports",
		}[input.Provider]
		if !seenCapabilities[capability] {
			return fail(fmt.Sprintf("$.provider_inputs[%d].provider", i), "provider capability missing")
		}
	}
	for i, view := range doc.DerivedViews {
		capability := map[string]string{
			"repository_ir":      "syntax_graph",
			"artifact_catalog":   "artifact_declarations",
			"agent_context":      "bounded_context",
			"obligation_handoff": "obligation_handoff",
		}[view.Kind]
		if !seenCapabilities[capability] {
			return fail(fmt.Sprintf("$.derived_views[%d].kind", i), "derived view capability missing")
		}
	}
	return nil
}

func identity(all, category map[string]bool, id, path string) error {
	if !idPattern.MatchString(id) {
		return fail(path, "invalid identity")
	}
	if all[id] {
		return fail(path, "duplicate identity")
	}
	all[id] = true
	category[id] = true
	return nil
}

func references(values []string, available map[string]bool, path, kind string) error {
	if len(values) > 256 {
		return fail(path, "count limit exceeded")
	}
	seen := make(map[string]bool, len(values))
	for i, value := range values {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		if !available[value] {
			return fail(itemPath, "unknown "+kind)
		}
		if seen[value] {
			return fail(itemPath, "duplicate "+kind)
		}
		seen[value] = true
	}
	return nil
}

func oneOf(value string, choices ...string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}
