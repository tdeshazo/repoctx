package agentctx

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/tdeshazo/repoctx/pkg/compiler"
)

func TestArtifactLikeMetadataDoesNotActivateCatalog(t *testing.T) {
	root, r := goFixture(t)
	before, err := Build(r, baseOptions(root))
	if err != nil {
		t.Fatal(err)
	}
	oldIR, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	oldContext, err := json.Marshal(before)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"artifacts.json", "repoctx.artifacts.json", ".repoctx/artifacts.json"} {
		// Invalid catalog syntax would fail if implicitly loaded. Metadata is outside
		// supported source inventory and must not change either source-only output.
		write(t, root, name, `{"version":"repoctx.artifacts/v1alpha1","command":"touch EXECUTED"}`)
	}
	afterIR, err := compiler.Compile(compiler.Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	nextIR, err := json.Marshal(afterIR)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(oldIR, nextIR) {
		t.Fatal("artifact-like metadata changed source-only IR")
	}
	after, err := Build(afterIR, baseOptions(root))
	if err != nil {
		t.Fatal(err)
	}
	nextContext, err := json.Marshal(after)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(oldContext, nextContext) {
		t.Fatal("artifact-like metadata changed source-only context")
	}
}
