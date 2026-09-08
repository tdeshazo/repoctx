package ir

import (
	"encoding/json"
	"testing"
)

func FuzzValidateDoesNotPanic(f *testing.F) {
	f.Add([]byte(`{"v":"repoctx.ir/v1alpha3","f":[]}`))
	f.Add([]byte(`{"v":"repoctx.ir/v1alpha3","f":[],"g":{"o":{"o":[0]},"i":{"o":[0]}}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			t.Skip()
		}
		var r Repository
		if json.Unmarshal(data, &r) == nil {
			_ = r.Validate()
		}
	})
}
