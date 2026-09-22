package discovery

import "testing"

func TestIdentifierMatchesUsesIdentifierBoundaries(t *testing.T) {
	tests := []struct {
		name, identifier, path, text string
		want                         int
	}{
		{
			name:       "exact prose identifier",
			identifier: "M4-07",
			text:       "M4-07 requires an atomic checkpoint.",
			want:       1,
		},
		{
			name: "empty identifier does not match",
			text: "M4-07 requires an atomic checkpoint.",
			want: 0,
		},
		{
			name:       "numeric suffix is not exact",
			identifier: "M4-07",
			text:       "M4-070 is a separate checkpoint.",
			want:       0,
		},
		{
			name:       "unicode letter prefix is not exact",
			identifier: "M4-07",
			text:       "αM4-07 is a separate checkpoint.",
			want:       0,
		},
		{
			name:       "unicode digit suffix is not exact",
			identifier: "M4-07",
			text:       "M4-07١ is a separate checkpoint.",
			want:       0,
		},
		{
			name:       "punctuation delimits identifier",
			identifier: "M4-07",
			text:       "[M4-07], then (M4-07).",
			want:       1,
		},
		{
			name:       "identifier in full path",
			identifier: "M4-07",
			path:       "docs/roadmap/M4-07-evidence.md",
			want:       1,
		},
		{
			name:       "identifier in basename path",
			identifier: "M4-07",
			path:       "M4-07-evidence.md",
			want:       1,
		},
		{
			name:       "numeric path suffix is not exact",
			identifier: "M4-07",
			path:       "docs/M4-070-evidence.md",
			want:       0,
		},
		{
			name:       "dotted symbol identifier",
			identifier: "rcx.topic.retry",
			text:       "id: rcx.topic.retry; kind: map",
			want:       1,
		},
		{
			name:       "dotted symbol continuation is not exact",
			identifier: "rcx.topic.retry",
			text:       "id: rcx.topic.retryable",
			want:       0,
		},
		{
			name:       "dotted symbol path",
			identifier: "rcx.topic.retry",
			path:       "docs/rcx.topic.retry.md",
			want:       1,
		},
		{
			name:       "overlapping later identifier is exact",
			identifier: "a.a",
			text:       "xa.a.a",
			want:       1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := identifierMatches([]string{tc.identifier}, tc.path, tc.text)
			if got != tc.want {
				t.Fatalf("identifierMatches(%q, %q, %q) = %d, want %d", tc.identifier, tc.path, tc.text, got, tc.want)
			}
		})
	}
}

func TestIdentifierMatchesCountsEachIdentifierOnce(t *testing.T) {
	got := identifierMatches(
		[]string{"M4-07", "rcx.topic.retry"},
		"docs/M4-07-evidence.md",
		"M4-07 references rcx.topic.retry and M4-07 again.",
	)
	if got != 2 {
		t.Fatalf("identifierMatches count = %d, want 2", got)
	}
}

func TestDiscoverIdentifierBoundariesRankExactEvidenceFirst(t *testing.T) {
	prefixOnly := "M4-070 M4-070 M4-070 M4-070 M4-070 M4-070 M4-070 M4-070 M4-070 M4-070\n"
	o := DefaultOptions()
	o.Root = fixture(t, map[string]string{
		"a-prefix-only.md": prefixOnly,
		"z-exact.md":       "M4-07 is the requested checkpoint.\n",
	})
	o.Query, o.MaxResults = "M4-07", 1
	r := run(t, o)
	if len(r.Response.Results) != 1 || r.Response.Results[0].Path != "z-exact.md" {
		t.Fatalf("exact identifier did not outrank stronger prefix evidence: %+v", r.Response.Results)
	}
	if r.Response.Results[0].identifierMatches != 1 {
		t.Fatalf("exact result identifier matches = %d, want 1", r.Response.Results[0].identifierMatches)
	}

	o.Root = fixture(t, map[string]string{"a-prefix-only.md": prefixOnly})
	r = run(t, o)
	if len(r.Response.Results) != 1 || r.Response.Results[0].Path != "a-prefix-only.md" {
		t.Fatalf("prefix-only lexical evidence was not retained: %+v", r.Response.Results)
	}
	if r.Response.Results[0].identifierMatches != 0 || r.Response.Results[0].Score.Occurrences == 0 {
		t.Fatalf("prefix-only result was not lexical-only evidence: %+v", r.Response.Results[0])
	}
}
