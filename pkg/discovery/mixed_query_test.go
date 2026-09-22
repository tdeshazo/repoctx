package discovery

import (
	"strings"
	"testing"
)

func TestDiscoverRetainsLexicalWindowsForMixedIdentifierQueries(t *testing.T) {
	tests := []struct {
		name, query, decisive string
		files                 map[string]string
	}{
		{
			name:     "path identifier with body term",
			query:    "README.md timeout",
			decisive: "timeout_seconds: 30\n",
			files: map[string]string{
				"README.md": "# Repository guide\n\nThis file explains the project.\n\ntimeout_seconds: 30\n",
			},
		},
		{
			name:     "missing identifier with relevant prose",
			query:    "M4-99 checkpoint recovery",
			decisive: "Checkpoint recovery retains the saved evidence.\n",
			files: map[string]string{
				"docs/recovery.md": "# Recovery\n\nCheckpoint recovery retains the saved evidence.\n",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := DefaultOptions()
			o.Root = fixture(t, tc.files)
			o.Query = tc.query
			o.ContextLines = 0
			o.MaxResults = 1

			r := run(t, o)
			if len(r.Response.Results) != 1 || r.Response.Results[0].Evidence == nil {
				t.Fatalf("missing discovery evidence: %+v", r.Response.Results)
			}
			if !strings.Contains(r.Response.Results[0].Evidence.Text, tc.decisive) {
				t.Fatalf("decisive lexical evidence was omitted: %+v", r.Response.Results[0].Evidence)
			}
		})
	}
}

func TestDiscoverDoesNotDuplicateOverlappingLexicalWindows(t *testing.T) {
	planning := strings.Join([]string{
		"Planning input.\n",
		"context\n",
		"for a future report.\n",
		"M4-09\n",
		"failure\n",
		"diagnosis\n",
	}, "")
	separator := strings.Repeat("unrelated material\n", 8)
	actionable := "- [ ] **M4-09 — Context failure diagnosis is actionable.**\n"

	o := DefaultOptions()
	o.Root = fixture(t, map[string]string{
		"ROADMAP.md": planning + separator + actionable,
	})
	o.Query = "M4-09 context failure diagnosis"
	o.MaxResults = 2

	r := run(t, o)
	for _, result := range r.Response.Results {
		if result.Evidence != nil && strings.Contains(result.Evidence.Text, actionable) {
			return
		}
	}
	t.Fatalf("overlapping lexical windows crowded out actionable evidence: %+v", r.Response.Results)
}
