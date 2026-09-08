package ir

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
)

// String resolves an interned reference. Validate must precede graph traversal.
func (r *Repository) String(ref int) string {
	if ref <= 0 || ref > len(r.Strings) {
		return ""
	}
	return r.Strings[ref-1]
}

// SnapshotID identifies the complete normalized index, not a Git revision.
// It binds ASTs, file hashes, symbols, graph and diagnostics. It is not a signature.
func (r *Repository) SnapshotID() (string, error) {
	b, e := json.Marshal(r)
	if e != nil {
		return "", e
	}
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:]), nil
}

// Validate rejects unsafe references, malformed source paths and inconsistent
// graph tables. It is intentionally structural, not language type checking.
func (r *Repository) Validate() error {
	if r == nil {
		return fmt.Errorf("nil repository")
	}
	if r.Version != "repoctx.ir/v1alpha2" && r.Version != "repoctx.ir/v1alpha3" {
		return fmt.Errorf("unsupported IR version %q", r.Version)
	}
	str := func(n int, optional bool) bool { return (optional && n == 0) || n > 0 && n <= len(r.Strings) }
	ref := func(f, n int) bool { return f >= 0 && f < len(r.Files) && n >= 0 && n < len(r.Files[f].Nodes) }
	if !str(r.Root, true) {
		return fmt.Errorf("invalid root string reference")
	}
	paths := map[string]bool{}
	for i, f := range r.Files {
		p := r.String(f.Path)
		if !str(f.Path, false) || !fs.ValidPath(p) || p == "." || strings.ContainsAny(p, "\\\x00") || strings.Contains(p, ":") || paths[p] {
			return fmt.Errorf("file %d: unsafe or duplicate path", i)
		}
		paths[p] = true
		if f.Lang < LangGo || f.Lang > LangTSX {
			return fmt.Errorf("file %d: invalid language", i)
		}
		if !str(f.Unit, true) {
			return fmt.Errorf("file %d: invalid unit", i)
		}
		hash, err := hex.DecodeString(f.Hash)
		if err != nil || (len(hash) != 32 && !(r.Version == "repoctx.ir/v1alpha2" && len(hash) == 12)) {
			return fmt.Errorf("file %d: invalid content hash", i)
		}
		for j, n := range f.Nodes {
			if !str(n.Kind, false) || !str(n.Text, true) {
				return fmt.Errorf("file %d node %d: invalid string ref", i, j)
			}
			sp := n.Span
			// Go synthetic/empty nodes in legacy indexes can have line 0. Materializing
			// an evidence span still requires positive, byte-valid physical coordinates.
			if sp.SL < 0 || sp.SC < 0 || sp.EL < sp.SL || sp.EC < 0 || (sp.EL == sp.SL && sp.EC < sp.SC) {
				return fmt.Errorf("file %d node %d: malformed span", i, j)
			}
			for _, c := range n.Children {
				if c <= j || c >= len(f.Nodes) {
					return fmt.Errorf("file %d node %d: cyclic/invalid AST child", i, j)
				}
			}
		}
		for _, root := range f.Roots {
			if root < 0 || root >= len(f.Nodes) {
				return fmt.Errorf("file %d: invalid AST root", i)
			}
		}
	}
	ids := map[string]bool{}
	for i, s := range r.Symbols {
		id := r.String(s.ID)
		if !str(s.ID, false) || !str(s.Name, false) || !str(s.Receiver, true) || !ref(s.File, s.Node) || s.Kind < SymModule || s.Kind > SymConstant {
			return fmt.Errorf("symbol %d: invalid fields", i)
		}
		// Earlier snapshots may contain colliding semantic IDs. They remain readable
		// for stats, but context serving explicitly requires a freshly compiled v3.
		if r.Version == "repoctx.ir/v1alpha3" && ids[id] {
			return fmt.Errorf("symbol %d: duplicate semantic ID", i)
		}
		ids[id] = true
		if s.Parent < 0 || s.Parent > len(r.Symbols) || s.Parent == i+1 {
			return fmt.Errorf("symbol %d: invalid parent", i)
		}
	}
	colors := make([]uint8, len(r.Symbols))
	for i := range r.Symbols {
		j := i
		var chain []int
		for j >= 0 && colors[j] == 0 {
			colors[j] = 1
			chain = append(chain, j)
			j = r.Symbols[j].Parent - 1
		}
		if j >= 0 && colors[j] == 1 {
			return fmt.Errorf("cyclic symbol parent")
		}
		for _, n := range chain {
			colors[n] = 2
		}
	}
	for i, e := range r.Edges {
		if e.Kind < EdgeDefines || e.Kind > EdgeReferences || !ref(e.From.File, e.From.Node) || !str(e.Text, true) || e.ToSymbol < 0 || e.ToSymbol > len(r.Symbols) || e.OwnerSymbol < 0 || e.OwnerSymbol > len(r.Symbols) {
			return fmt.Errorf("edge %d: invalid reference", i)
		}
		if e.OwnerSymbol > 0 && r.Symbols[e.OwnerSymbol-1].File != e.From.File {
			return fmt.Errorf("edge %d: owner in wrong file", i)
		}
	}
	for i, d := range r.Diagnostics {
		if d.Severity < SeverityInfo || d.Severity > SeverityError || d.File < 0 || d.File > len(r.Files) || !str(d.Message, false) {
			return fmt.Errorf("diagnostic %d: invalid fields", i)
		}
	}
	if r.Graph == nil {
		return nil
	}
	for i, x := range r.Graph.External {
		if !str(x.Name, false) || x.Kind < GraphExternalUnit || x.Kind > GraphExternalSymbol {
			return fmt.Errorf("external node %d: invalid fields", i)
		}
	}
	n := len(r.Symbols) + len(r.Graph.External)
	if err := validateCSR(r.Graph.Out, n); err != nil {
		return fmt.Errorf("out CSR: %w", err)
	}
	if err := validateCSR(r.Graph.In, n); err != nil {
		return fmt.Errorf("in CSR: %w", err)
	}
	type key struct {
		f, t uint32
		k    EdgeKind
	}
	arcs := map[key]uint32{}
	for row := 0; row < n; row++ {
		c := r.Graph.Out
		for i := c.Offsets[row]; i < c.Offsets[row+1]; i++ {
			k := key{uint32(row), c.Targets[i], c.Kinds[i]}
			if arcs[k] != 0 {
				return fmt.Errorf("duplicate collapsed arc")
			}
			arcs[k] = c.Weights[i]
		}
	}
	for row := 0; row < n; row++ {
		c := r.Graph.In
		for i := c.Offsets[row]; i < c.Offsets[row+1]; i++ {
			k := key{c.Targets[i], uint32(row), c.Kinds[i]}
			if arcs[k] != c.Weights[i] {
				return fmt.Errorf("forward/reverse CSR mismatch")
			}
			delete(arcs, k)
		}
	}
	if len(arcs) != 0 {
		return fmt.Errorf("missing reverse arcs")
	}
	return nil
}

func validateCSR(c CSR, n int) error {
	if len(c.Offsets) != n+1 || c.Offsets[0] != 0 || uint64(c.Offsets[n]) != uint64(len(c.Targets)) || len(c.Targets) != len(c.Kinds) || len(c.Targets) != len(c.Weights) {
		return fmt.Errorf("invalid table lengths")
	}
	for i := 1; i < len(c.Offsets); i++ {
		if c.Offsets[i] < c.Offsets[i-1] || uint64(c.Offsets[i]) > uint64(len(c.Targets)) {
			return fmt.Errorf("nonmonotonic offsets")
		}
	}
	for i, t := range c.Targets {
		if uint64(t) >= uint64(n) || c.Kinds[i] < EdgeDefines || c.Kinds[i] > EdgeReferences || c.Weights[i] == 0 {
			return fmt.Errorf("invalid arc %d", i)
		}
	}
	return nil
}
