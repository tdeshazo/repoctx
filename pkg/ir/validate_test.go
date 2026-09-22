package ir

import (
	"encoding/json"
	"fmt"
	"strings"
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

func TestRepositoryIdentityRejectsPresentEmptyObject(t *testing.T) {
	base := `{"v":"repoctx.ir/v1alpha5","f":[],"inputs":{"profile":{"compiler":"test","frontends":["test"],"build":"test","allow":[],"deny":[],"ignore_dirs":[],"max_file_bytes":1,"max_read_bytes":1,"max_entries":1},"repository":%s,"sources":[],"go_mod":{"path":"go.mod","state":"absent"}}}`
	for _, test := range []struct {
		name string
		body string
		want bool
	}{
		{name: "empty", body: `{}`, want: false},
		{name: "dirty only", body: `{"dirty":false}`, want: true},
		{name: "revision only", body: `{"revision":"` + strings.Repeat("a", 40) + `"}`, want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var repository Repository
			if err := json.Unmarshal([]byte(fmt.Sprintf(base, test.body)), &repository); err != nil {
				t.Fatal(err)
			}
			err := repository.Validate()
			if (err == nil) != test.want {
				t.Fatalf("Validate() error = %v, want valid=%v", err, test.want)
			}
		})
	}
}
