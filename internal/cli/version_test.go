package cli

import "testing"

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
	if info.Version != buildInfoVersion || info.Contracts.IR != "repoctx.ir/v1alpha4" {
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
