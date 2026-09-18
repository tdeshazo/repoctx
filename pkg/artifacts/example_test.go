package artifacts_test

import (
	"fmt"
	"github.com/tdeshazo/repoctx/pkg/artifacts"
)

func ExampleDecode() {
	data := []byte(`{"version":"repoctx.artifacts/v1alpha1","namespace":"demo","artifacts":[],"relationships":[]}`)
	catalog, err := artifacts.Decode(data, artifacts.Limits{})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(catalog.Namespace, len(catalog.Artifacts))
	// Output: demo 0
}
