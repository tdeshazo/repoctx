package cli

import (
	"testing"

	"github.com/tdeshazo/repoctx/pkg/compat"
	"github.com/tdeshazo/repoctx/pkg/ir"
	"github.com/tdeshazo/repoctx/pkg/manifest"
)

func TestCurrentBuildInfoUsesInjectedProvenance(t *testing.T) {
	originalRelease := buildRelease
	originalRevision := buildRevision
	originalModified := buildModified
	originalDistribution := buildDistribution
	t.Cleanup(func() {
		buildRelease = originalRelease
		buildRevision = originalRevision
		buildModified = originalModified
		buildDistribution = originalDistribution
	})

	buildRelease = "v1.2.3"
	buildRevision = "0123456789abcdef"
	buildModified = "false"
	buildDistribution = "test-package"
	info := currentBuildInfo()
	if info.Release != buildRelease || info.Revision != buildRevision ||
		info.Modified != buildModified || info.Distribution != buildDistribution {
		t.Fatalf("injected provenance lost: %+v", info)
	}
	if buildInfoVersion != "repoctx.build/v1alpha4" {
		t.Fatalf("build info version = %q", buildInfoVersion)
	}
	if info.Version != buildInfoVersion || info.Contracts.GoAPI != compat.GoAPI ||
		info.Contracts.IR != "repoctx.ir/v1alpha4" ||
		info.Contracts.ArtifactAuthoring != "repoctx.artifact-authoring/v1alpha1" ||
		info.Contracts.DocumentLinks != "repoctx.document-links/v1" ||
		info.Contracts.GoImports != "repoctx.go-imports/v1" ||
		info.Contracts.Obligations != "repoctx.obligations/v1alpha1" ||
		info.Contracts.Manifest != manifest.Version || info.Contracts.Entities != ir.EntityVersion ||
		info.Contracts.Frontmatter != manifest.FrontmatterVersion {
		t.Fatalf("build contract incomplete: %+v", info)
	}
}

func TestNormalizedModifiedRejectsUnrecognizedValues(t *testing.T) {
	for _, value := range []string{"", "yes", "TRUE"} {
		if got := normalizedModified(value); got != "unknown" {
			t.Fatalf("normalizedModified(%q) = %q", value, got)
		}
	}
}
