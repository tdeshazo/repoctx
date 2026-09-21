package artifacts

import (
	"reflect"
	"strings"
	"testing"
)

func declaration(id, kind string, scopes []Applicability, inputs []Input) Artifact {
	artifact := minimal().Artifacts[0]
	artifact.ID = id
	artifact.Kind = kind
	artifact.AppliesTo = scopes
	artifact.DeclaredInputs = append([]Input{}, inputs...)
	return artifact
}

func relationship(from, to, kind string) Relationship {
	relationship := edge()
	relationship.From = from
	relationship.To = to
	relationship.Kind = kind
	return relationship
}

func resolve(t *testing.T, catalog *Catalog, authority Authority) *Resolution {
	t.Helper()
	resolution, err := Resolve(catalog, authority)
	if err != nil {
		t.Fatal(err)
	}
	return resolution
}

func diagnosticCodes(resolution *Resolution) []string {
	codes := make([]string, 0, len(resolution.Diagnostics))
	for _, diagnostic := range resolution.Diagnostics {
		codes = append(codes, diagnostic.Code)
	}
	return codes
}

func TestResolveRequiresExplicitCallerAuthority(t *testing.T) {
	catalog := minimal()
	catalog.Artifacts[0].AppliesTo = []Applicability{{Kind: "subtree", Path: "."}}

	resolution := resolve(t, catalog, Authority{})
	if len(resolution.Artifacts) != 0 || len(resolution.Diagnostics) != 0 {
		t.Fatalf("repository claim gained authority: %+v", resolution)
	}

	resolution = resolve(t, catalog, Authority{
		AcceptedIDs: []string{"a:a"},
		Scopes:      []Applicability{{Kind: "subtree", Path: "pkg"}},
	})
	expected := []EffectiveArtifact{{
		ID:               "a:a",
		Kind:             "component",
		LifecycleClaim:   "active",
		DeclarationIndex: 0,
		Scopes:           []Applicability{{Kind: "subtree", Path: "pkg"}},
	}}
	if !reflect.DeepEqual(resolution.Artifacts, expected) || len(resolution.Diagnostics) != 0 {
		t.Fatalf("unexpected resolution: %+v", resolution)
	}

	// A repository lifecycle claim cannot override the caller's explicit choice.
	catalog.Artifacts[0].Lifecycle = "retired"
	resolution = resolve(t, catalog, Authority{
		AcceptedIDs: []string{"a:a"},
		Scopes:      []Applicability{{Kind: "file", Path: "pkg/file.go"}},
	})
	if len(resolution.Artifacts) != 1 || resolution.Artifacts[0].LifecycleClaim != "retired" {
		t.Fatalf("lifecycle claim changed authority: %+v", resolution)
	}
}

func TestResolveDiagnosesDuplicateAndBrokenReferences(t *testing.T) {
	catalog := minimal()
	catalog.Artifacts = append(catalog.Artifacts, catalog.Artifacts[0])
	catalog.Relationships = []Relationship{
		relationship("a:a", "a:missing", "references"),
		relationship("other:external", "a:a", "depends_on"),
	}
	resolution := resolve(t, catalog, Authority{
		AcceptedIDs: []string{"a:a", "a:absent"},
		Scopes:      []Applicability{{Kind: "subtree", Path: "."}},
	})
	if len(resolution.Artifacts) != 0 {
		t.Fatalf("ambiguous declaration became effective: %+v", resolution.Artifacts)
	}
	expected := []string{"accepted_id_missing", "ambiguous_reference", "ambiguous_reference", "broken_reference", "broken_reference", "duplicate_id"}
	if !reflect.DeepEqual(diagnosticCodes(resolution), expected) {
		t.Fatalf("diagnostics = %+v, want %+v", resolution.Diagnostics, expected)
	}
}

func TestResolveIntersectsScopeAndRejectsInvalidEffectiveScope(t *testing.T) {
	catalog := minimal()
	catalog.Artifacts = []Artifact{
		declaration("a:file", "component", []Applicability{{Kind: "file", Path: "pkg/a.go"}}, nil),
		declaration("a:subtree", "component", []Applicability{{Kind: "subtree", Path: "pkg"}}, nil),
		declaration("a:outside", "component", []Applicability{{Kind: "subtree", Path: "cmd"}}, nil),
	}
	resolution := resolve(t, catalog, Authority{
		AcceptedIDs: []string{"a:outside", "a:subtree", "a:file"},
		Scopes: []Applicability{
			{Kind: "subtree", Path: "pkg/internal"},
			{Kind: "file", Path: "pkg/a.go"},
		},
	})
	if got := []string{resolution.Artifacts[0].ID, resolution.Artifacts[1].ID}; !reflect.DeepEqual(got, []string{"a:file", "a:subtree"}) {
		t.Fatalf("effective IDs = %v", got)
	}
	if got := resolution.Artifacts[1].Scopes; !reflect.DeepEqual(got, []Applicability{
		{Kind: "file", Path: "pkg/a.go"},
		{Kind: "subtree", Path: "pkg/internal"},
	}) {
		t.Fatalf("scope intersection = %+v", got)
	}
	if !reflect.DeepEqual(diagnosticCodes(resolution), []string{"invalid_effective_scope"}) {
		t.Fatalf("diagnostics = %+v", resolution.Diagnostics)
	}
}

func TestResolveDiagnosesSupersessionCyclesAndAcceptedConflict(t *testing.T) {
	catalog := minimal()
	scope := []Applicability{{Kind: "subtree", Path: "pkg"}}
	catalog.Artifacts = []Artifact{
		declaration("a:new", "requirement", scope, nil),
		declaration("a:old", "requirement", scope, nil),
		declaration("a:other", "requirement", scope, nil),
	}
	catalog.Relationships = []Relationship{
		relationship("a:new", "a:old", "supersedes"),
		relationship("a:old", "a:other", "supersedes"),
		relationship("a:other", "a:new", "supersedes"),
	}
	resolution := resolve(t, catalog, Authority{
		AcceptedIDs: []string{"a:new", "a:old"},
		Scopes:      scope,
	})
	if len(resolution.Artifacts) != 0 {
		t.Fatalf("conflicting accepted versions became effective: %+v", resolution.Artifacts)
	}
	if !reflect.DeepEqual(diagnosticCodes(resolution), []string{"accepted_supersession_conflict", "supersession_cycle"}) {
		t.Fatalf("diagnostics = %+v", resolution.Diagnostics)
	}

	// Accepting only one side does not let the supersession claim select it; the
	// caller made that choice explicitly.
	resolution = resolve(t, catalog, Authority{AcceptedIDs: []string{"a:new"}, Scopes: scope})
	if len(resolution.Artifacts) != 1 || resolution.Artifacts[0].ID != "a:new" {
		t.Fatalf("explicit acceptance lost: %+v", resolution)
	}
}

func TestResolveDiagnosesConflictingAcceptedRequirements(t *testing.T) {
	zero := strings.Repeat("0", 64)
	one := strings.Repeat("1", 64)
	scope := []Applicability{{Kind: "subtree", Path: "pkg"}}
	catalog := minimal()
	catalog.Artifacts = []Artifact{
		declaration("a:left", "requirement", scope, []Input{{Path: "go.mod", Role: "configuration", Required: true, SHA256: &zero}}),
		declaration("a:right", "requirement", scope, []Input{{Path: "go.mod", Role: "configuration", Required: true, SHA256: &one}}),
		declaration("a:self", "requirement", scope, []Input{
			{Path: "config.json", Role: "source", Required: true},
			{Path: "config.json", Role: "source", Required: false},
		}),
	}
	resolution := resolve(t, catalog, Authority{
		AcceptedIDs: []string{"a:right", "a:self", "a:left"},
		Scopes:      scope,
	})
	if len(resolution.Artifacts) != 0 {
		t.Fatalf("conflicting requirements became effective: %+v", resolution.Artifacts)
	}
	if !reflect.DeepEqual(diagnosticCodes(resolution), []string{
		"conflicting_accepted_requirements",
		"conflicting_accepted_requirements",
	}) {
		t.Fatalf("diagnostics = %+v", resolution.Diagnostics)
	}
	if resolution.Diagnostics[0].Path != "go.mod" || resolution.Diagnostics[1].Path != "config.json" {
		t.Fatalf("conflict paths = %+v", resolution.Diagnostics)
	}
}

func TestResolveRejectsInvalidTrustedAuthority(t *testing.T) {
	catalog := minimal()
	cases := []Authority{
		{AcceptedIDs: []string{"other:a"}},
		{AcceptedIDs: []string{"a:a", "a:a"}},
		{Scopes: []Applicability{{Kind: "subtree", Path: "../outside"}}},
		{Scopes: repeat(Applicability{Kind: "subtree", Path: "."}, 65)},
	}
	for _, authority := range cases {
		resolution, err := Resolve(catalog, authority)
		if err == nil || resolution != nil || len(err.Error()) > 256 {
			t.Fatalf("authority=%+v resolution=%+v err=%v", authority, resolution, err)
		}
	}
	if resolution, err := Resolve(nil, Authority{}); err == nil || resolution != nil {
		t.Fatalf("nil catalog resolution=%+v err=%v", resolution, err)
	}
}

func TestResolveRejectsMutatedCatalog(t *testing.T) {
	cases := []*Catalog{minimal(), minimal(), minimal()}
	cases[0].Relationships = nil
	cases[1].Artifacts[0].AppliesTo = nil
	invalidUTF8 := string([]byte{0xff})
	cases[2].Artifacts[0].Owner = &invalidUTF8
	for _, catalog := range cases {
		resolution, err := Resolve(catalog, Authority{})
		if err == nil || resolution != nil || len(err.Error()) > 256 {
			t.Fatalf("catalog=%+v resolution=%+v err=%v", catalog, resolution, err)
		}
	}
}

func TestResolveIsIndependentOfDeclarationOrder(t *testing.T) {
	scope := []Applicability{{Kind: "subtree", Path: "."}}
	first := minimal()
	first.Artifacts = []Artifact{
		declaration("a:z", "component", scope, nil),
		declaration("a:a", "component", scope, nil),
	}
	second := minimal()
	second.Artifacts = []Artifact{first.Artifacts[1], first.Artifacts[0]}
	authority := Authority{AcceptedIDs: []string{"a:z", "a:a"}, Scopes: scope}
	one := resolve(t, first, authority)
	two := resolve(t, second, authority)
	if len(one.Artifacts) != 2 || len(two.Artifacts) != 2 {
		t.Fatalf("unexpected resolutions: %+v %+v", one, two)
	}
	for i := range one.Artifacts {
		if one.Artifacts[i].ID != two.Artifacts[i].ID || !reflect.DeepEqual(one.Artifacts[i].Scopes, two.Artifacts[i].Scopes) {
			t.Fatalf("declaration order selected authority: %+v %+v", one, two)
		}
	}
}
