package agentctx

import "github.com/tdeshazo/repoctx/pkg/ir"

// taskID binds all selection, rendering and budget inputs. Root locations and
// function addresses are not identities. The caller supplies tokenizer identity.
func taskID(index string, o Options) (string, error) {
	consistency := o.Consistency
	if o.sourceMode != "" {
		consistency += "+" + o.sourceMode
	}
	if o.authorityScope != "" {
		consistency += "+authority:" + o.authorityScope
	}
	// Options includes a function even when nil; encode an explicit mirror so
	// future selection options must be deliberately added to this contract.
	return ir.ContentID(struct {
		Index, Renderer, Query                                                 string
		Symbols, Units, Allow, Deny                                            []string
		Depth                                                                  int
		Direction                                                              string
		Relations                                                              []ir.EdgeKind
		MaxSymbols, MaxUnits, MaxCandidates, MaxRelations, MaxBytes, MaxTokens int
		MaxSourceBytes, MaxReadBytes                                           int64
		Tokenizer, Consistency                                                 string
	}{
		Index: index, Renderer: Version + "/" + o.Format + "/m2.1", Query: o.Query,
		Symbols: o.Symbols, Units: o.Units, Allow: o.AllowPaths, Deny: o.DenyPaths,
		Depth: o.Depth, Direction: o.Direction, Relations: o.Relations,
		MaxSymbols: o.MaxSymbols, MaxUnits: o.MaxUnits, MaxCandidates: o.MaxCandidates,
		MaxRelations: o.MaxRelations, MaxBytes: o.MaxBytes, MaxTokens: o.MaxTokens,
		MaxSourceBytes: o.MaxSourceBytes, MaxReadBytes: o.MaxReadBytes,
		Tokenizer: o.TokenizerID, Consistency: consistency,
	})
}
