package compat_test

import (
	"testing"

	"github.com/tdeshazo/repoctx/pkg/agentctx"
	"github.com/tdeshazo/repoctx/pkg/artifacts"
	"github.com/tdeshazo/repoctx/pkg/compat"
	"github.com/tdeshazo/repoctx/pkg/compiler"
	"github.com/tdeshazo/repoctx/pkg/discovery"
	"github.com/tdeshazo/repoctx/pkg/ir"
	"github.com/tdeshazo/repoctx/pkg/manifest"
	"github.com/tdeshazo/repoctx/pkg/obligation"
)

func TestPublishedContractVersions(t *testing.T) {
	versions := map[string]string{
		"Go API":                  compat.GoAPI,
		"IR":                      compiler.IRVersion,
		"agent context":           agentctx.Version,
		"discovery":               discovery.Version,
		"artifacts":               artifacts.Version,
		"artifact authoring":      artifacts.AuthoringVersion,
		"document-links provider": artifacts.DocumentLinkProviderVersion,
		"Go-imports provider":     artifacts.GoImportProviderVersion,
		"obligation handoff":      obligation.Version,
		"repository entities":     ir.EntityVersion,
		"repository manifest":     manifest.Version,
		"document frontmatter":    manifest.FrontmatterVersion,
	}
	want := map[string]string{
		"Go API":                  "repoctx.go-api/v1alpha1",
		"IR":                      "repoctx.ir/v1alpha4",
		"agent context":           "repoctx.context/v1alpha3",
		"discovery":               "repoctx.discovery/v1alpha1",
		"artifacts":               "repoctx.artifacts/v1alpha1",
		"artifact authoring":      "repoctx.artifact-authoring/v1alpha1",
		"document-links provider": "repoctx.document-links/v1",
		"Go-imports provider":     "repoctx.go-imports/v1",
		"obligation handoff":      "repoctx.obligations/v1alpha1",
		"repository entities":     "repoctx.entities/v1alpha1",
		"repository manifest":     "repoctx.manifest/v1alpha2",
		"document frontmatter":    "repoctx.frontmatter/v1alpha1",
	}
	for name, version := range versions {
		if version != want[name] {
			t.Errorf("%s version = %q, want %q", name, version, want[name])
		}
	}
}

func TestPublishedProviderNames(t *testing.T) {
	if artifacts.DocumentLinkProviderName != "repoctx.document-links" {
		t.Fatalf("document-link provider name = %q", artifacts.DocumentLinkProviderName)
	}
	if artifacts.GoImportProviderName != "repoctx.go-imports" {
		t.Fatalf("Go-import provider name = %q", artifacts.GoImportProviderName)
	}
}
