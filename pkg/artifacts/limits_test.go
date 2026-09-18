package artifacts

import (
	"encoding/json"
	"strings"
	"testing"
)

func repeat[T any](v T, n int) []T {
	out := make([]T, n)
	for i := range out {
		out[i] = v
	}
	return out
}

func TestReaderResourceBoundaries(t *testing.T) {
	t.Run("bytes", func(t *testing.T) {
		b := []byte(empty + strings.Repeat(" ", (1<<20)-len(empty)))
		accepted(t, b, Limits{})
		rejected(t, append(b, ' '), Limits{}, "byte limit")
		accepted(t, []byte(empty), Limits{Bytes: len(empty)})
		rejected(t, []byte(empty+" "), Limits{Bytes: len(empty)}, "byte limit")
	})
	cases := []struct {
		name  string
		limit int
		set   func(*Catalog, int)
	}{
		{"artifacts", 1024, func(c *Catalog, n int) { c.Artifacts = repeat(c.Artifacts[0], n) }},
		{"applicability", 64, func(c *Catalog, n int) {
			c.Artifacts[0].AppliesTo = repeat(Applicability{Kind: "subtree", Path: "."}, n)
		}},
		{"inputs", 64, func(c *Catalog, n int) { c.Artifacts[0].DeclaredInputs = repeat(Input{Path: "a", Role: "source"}, n) }},
		{"artifact sources", 16, func(c *Catalog, n int) { c.Artifacts[0].Sources = repeat(c.Artifacts[0].Sources[0], n) }},
		{"relationship sources", 16, func(c *Catalog, n int) {
			r := edge()
			r.Sources = repeat(r.Sources[0], n)
			c.Relationships = []Relationship{r}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := minimal()
			tc.set(c, tc.limit)
			b := wire(t, c)
			accepted(t, b, Limits{})
			t.Logf("reader accepted count=%d bytes=%d", tc.limit, len(b))
			tc.set(c, tc.limit+1)
			rejected(t, wire(t, c), Limits{}, "count limit")
		})
	}
	t.Run("lower relationships", func(t *testing.T) {
		c := minimal()
		c.Relationships = repeat(edge(), 2)
		accepted(t, wire(t, c), Limits{Relationships: 2})
		c.Relationships = repeat(edge(), 3)
		rejected(t, wire(t, c), Limits{Relationships: 2}, "relationships: count limit")
	})
	t.Run("lower aggregate", func(t *testing.T) {
		c := minimal()
		c.Artifacts[0].DeclaredInputs = repeat(Input{Path: "a", Role: "source"}, 1)
		accepted(t, wire(t, c), Limits{NestedEntries: 2})
		c.Artifacts[0].DeclaredInputs = repeat(Input{Path: "a", Role: "source"}, 2)
		rejected(t, wire(t, c), Limits{NestedEntries: 2}, "nested_entries: count limit")
	})
	for _, tc := range []struct {
		l   Limits
		set func(*Catalog)
	}{
		{Limits{Artifacts: 1}, func(c *Catalog) { c.Artifacts = repeat(c.Artifacts[0], 2) }},
		{Limits{Applicability: 1}, func(c *Catalog) { c.Artifacts[0].AppliesTo = repeat(Applicability{Kind: "file", Path: "a"}, 2) }},
		{Limits{Inputs: 1}, func(c *Catalog) { c.Artifacts[0].DeclaredInputs = repeat(Input{Path: "a", Role: "source"}, 2) }},
		{Limits{Sources: 1}, func(c *Catalog) { c.Artifacts[0].Sources = repeat(c.Artifacts[0].Sources[0], 2) }},
	} {
		c := minimal()
		if tc.l.Applicability > 0 {
			c.Artifacts[0].AppliesTo = []Applicability{{Kind: "file", Path: "a"}}
		}
		if tc.l.Inputs > 0 {
			c.Artifacts[0].DeclaredInputs = []Input{{Path: "a", Role: "source"}}
		}
		accepted(t, wire(t, c), tc.l)
		tc.set(c)
		rejected(t, wire(t, c), tc.l, "count limit")
	}
	t.Run("depth", func(t *testing.T) {
		b := wire(t, minimal())
		accepted(t, b, Limits{Depth: 5})
		rejected(t, b, Limits{Depth: 4}, "depth limit")
		for _, n := range []int{16, 17} {
			raw := strings.Repeat("[", n) + "0" + strings.Repeat("]", n)
			d := json.NewDecoder(strings.NewReader(raw))
			d.UseNumber()
			err := scan(d, 0, 16)
			if (err == nil) != (n == 16) {
				t.Fatalf("isolated tokenizer depth=%d error=%v", n, err)
			}
			t.Logf("isolated tokenizer depth=%d error=%v", n, err)
			reason := "expected object"
			if n == 17 {
				reason = "depth limit"
			}
			rejected(t, []byte(raw), Limits{}, reason)
		}
	})
	for _, l := range []Limits{{Bytes: -1}, {Depth: 17}, {Bytes: 1<<20 + 1}, {Artifacts: 1025}, {Relationships: 4097},
		{Applicability: 65}, {Inputs: 65}, {Sources: 17}, {NestedEntries: 16385}} {
		rejected(t, []byte(empty), l, "invalid caller limit")
	}
}

func TestIsolatedProductionCountersAndMaskedReader(t *testing.T) {
	c := minimal()
	c.Artifacts = []Artifact{}
	c.Relationships = repeat(edge(), 4096)
	if err := validate(c, ceilings()); err != nil {
		t.Fatal(err)
	}
	b := wire(t, c)
	if len(b) != 1093721 {
		t.Fatalf("independent relationship size: %d", len(b))
	}
	rejected(t, b, Limits{}, "byte limit")
	c.Relationships = append(c.Relationships, edge())
	if err := validate(c, ceilings()); err == nil || !strings.Contains(err.Error(), "relationships: count limit") {
		t.Fatal(err)
	}
	t.Log("isolated validator: 4096 accepted / 4097 rejected relationships; reader masked by byte cap")
	c = minimal()
	a := c.Artifacts[0]
	a.Sources = repeat(a.Sources[0], 16)
	c.Artifacts = repeat(a, 1024)
	if err := validate(c, ceilings()); err != nil {
		t.Fatal(err)
	}
	rejected(t, wire(t, c), Limits{}, "byte limit")
	c.Artifacts[0].DeclaredInputs = []Input{{Path: "a", Role: "source"}}
	if err := validate(c, ceilings()); err == nil || !strings.Contains(err.Error(), "nested_entries: count limit") {
		t.Fatal(err)
	}
	t.Log("isolated validator: 16384 accepted / 16385 rejected nested entries; reader masked by byte cap")
}

func TestScalarBoundaries(t *testing.T) {
	cases := []struct {
		name string
		n    int
		set  func(*Catalog, int)
	}{
		{"namespace", 63, func(c *Catalog, n int) { c.Namespace = strings.Repeat("a", n); c.Artifacts[0].ID = c.Namespace + ":a" }},
		{"local id", 128, func(c *Catalog, n int) { c.Artifacts[0].ID = "a:" + strings.Repeat("a", n) }},
		{"owner bytes", 256, func(c *Catalog, n int) {
			s := strings.Repeat("é", n/2) + strings.Repeat("a", n%2)
			c.Artifacts[0].Owner = &s
		}},
		{"check id", 256, func(c *Catalog, n int) {
			s := strings.Repeat("a", n)
			c.Artifacts[0].Kind = "verification_obligation"
			c.Artifacts[0].RunnerCheckID = &s
		}},
		{"path bytes", 4096, func(c *Catalog, n int) {
			c.Artifacts[0].Sources[0].Path = strings.Repeat("é", n/2) + strings.Repeat("a", n%2)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := minimal()
			tc.set(c, tc.n)
			accepted(t, wire(t, c), Limits{})
			tc.set(c, tc.n+1)
			rejected(t, wire(t, c), Limits{}, "")
			tc.set(c, 0)
			rejected(t, wire(t, c), Limits{}, "")
		})
	}
	c := minimal()
	c.Artifacts[0].Sources[0].EndByte = 9007199254740991
	c.Artifacts[0].Sources[0].EndLine = 9007199254740991
	c.Artifacts[0].Sources[0].EndByteColumn = 9007199254740991
	accepted(t, wire(t, c), Limits{})
	for _, field := range []string{"end_byte", "end_line", "end_byte_column"} {
		b := strings.Replace(string(wire(t, c)), `"`+field+`":9007199254740991`, `"`+field+`":9007199254740992`, 1)
		rejected(t, []byte(b), Limits{}, "integer range")
	}
	// Unknown names and values must never be echoed, even near the byte cap.
	b := strings.Replace(empty, `"namespace":"a"`, `"namespace":"a","`+strings.Repeat("SECRET", 10000)+`":"SECRET"`, 1)
	rejected(t, []byte(b), Limits{}, "unknown field")
}

func FuzzDecode(f *testing.F) {
	f.Add([]byte(empty))
	f.Add([]byte(`{"a":"\ud800"}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		c, err := Decode(b, Limits{})
		if err != nil {
			if c != nil || len(err.Error()) > 256 {
				t.Fatal("partial result or unbounded error")
			}
			return
		}
		next := accepted(t, wire(t, c), Limits{})
		if string(wire(t, c)) != string(wire(t, next)) {
			t.Fatal("unstable round trip")
		}
	})
}
