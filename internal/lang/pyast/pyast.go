// Package pyast preserves the original Python front-end API while delegating
// parsing to the pinned native Tree-sitter Python grammar. The interpreter
// argument is retained for source compatibility and intentionally ignored.
package pyast

import (
	"bytes"
	"io"

	"github.com/tdeshazo/repoctx/internal/lang/treeast"
	"github.com/tdeshazo/repoctx/pkg/ir"
)

// limitedBuffer remains available to older package-local callers. Tree-sitter
// does not use a subprocess protocol, but retaining the bounded helper type is
// harmless API compatibility for tests and downstream internal tooling.
type limitedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	remaining := b.limit - b.Len()
	if remaining <= 0 {
		return 0, io.ErrShortBuffer
	}
	if len(p) > remaining {
		_, _ = b.Buffer.Write(p[:remaining])
		return remaining, io.ErrShortBuffer
	}
	return b.Buffer.Write(p)
}

type Result = treeast.Result
type LocalSymbol = treeast.LocalSymbol
type LocalEdge = treeast.LocalEdge

// Parse lowers Python source with Tree-sitter. The python executable argument
// is ignored; no repository interpreter or subprocess is started.
func Parse(_ string, src []byte, st *ir.Strings) (Result, error) {
	return treeast.Parse(treeast.Python, src, st)
}
