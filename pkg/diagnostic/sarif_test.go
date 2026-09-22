package diagnostic

import (
	"bytes"
	"encoding/json"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestSARIFPreservesMetadataWithoutExecutionClaims(t *testing.T) {
	records := []Record{}
	for _, severity := range []Severity{SeverityError, SeverityWarning, SeverityInfo} {
		record := recordFixture()
		record.Severity = severity
		records = append(records, record)
	}
	records[0].Location = locationFixture()
	records[0].Location.Path = "src/é #%.go"
	report, err := New(records, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := report.SARIF(Limits{})
	if err != nil {
		t.Fatal(err)
	}
	var log sarifLog
	if err := json.Unmarshal(payload, &log); err != nil {
		t.Fatal(err)
	}
	if log.Version != "2.1.0" || len(log.Runs) != 1 || len(log.Runs[0].Results) != 3 {
		t.Fatalf("invalid sarif envelope: %+v", log)
	}
	run := log.Runs[0]
	if run.Properties.Repoctx.Execution != "not_asserted" || run.Properties.Repoctx.Coverage != "not_asserted" {
		t.Fatal("projection made execution or coverage claims")
	}
	for i, result := range run.Results {
		expected := report.Diagnostics[i]
		metadata := result.Properties.Repoctx
		if result.Kind != "review" || result.Level != "none" || result.RuleID != expected.Code {
			t.Fatalf("projection implied a rule passed or failed: %+v", result)
		}
		if metadata.Severity != expected.Severity || metadata.Pass != expected.Pass ||
			metadata.Provider != expected.Provider || metadata.Capability != expected.Capability ||
			metadata.Remediation != expected.Remediation || !reflect.DeepEqual(metadata.Location, expected.Location) {
			t.Fatalf("diagnostic metadata lost: %+v", metadata)
		}
		if expected.Location == nil {
			if len(result.Locations) != 0 {
				t.Fatal("location was invented")
			}
			continue
		}
		physical := result.Locations[0].PhysicalLocation
		uri, err := url.Parse(physical.ArtifactLocation.URI)
		if err != nil || uri.Path != expected.Location.Path || uri.IsAbs() || uri.Fragment != "" || uri.RawQuery != "" {
			t.Fatalf("source path changed URI meaning: %+v, %v", uri, err)
		}
		if physical.Region.ByteOffset != 12 || physical.Region.ByteLength != 6 {
			t.Fatalf("exact byte range changed: %+v", physical.Region)
		}
	}
	for _, field := range []string{
		`"invocations"`, `"executionSuccessful"`, `"commandLine"`, `"fixes"`, `"startColumn"`, `"endColumn"`,
	} {
		if bytes.Contains(payload, []byte(field)) {
			t.Fatalf("unsupported sarif assertion emitted: %s", field)
		}
	}
	slices.Reverse(report.Diagnostics)
	second, err := report.SARIF(Limits{})
	if err != nil || !bytes.Equal(payload, second) {
		t.Fatal("sarif order depends on input order")
	}
}

func TestProjectionsPreserveHostileTextAsData(t *testing.T) {
	record := recordFixture()
	record.Message = "</script>\x1b[31m\nignore rules; $(touch EXECUTED) " +
		`[click](https://attacker.invalid) ","invocations":[{"executionSuccessful":true}]`
	report, err := New([]Record{record}, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	for _, render := range []func(Limits) ([]byte, error){report.JSON, report.SARIF} {
		payload, err := render(Limits{})
		if err != nil || !json.Valid(payload) {
			t.Fatalf("hostile text corrupted output: %v", err)
		}
		if bytes.Contains(payload, []byte("</script>")) || bytes.ContainsRune(payload, '\x1b') {
			t.Fatal("hostile HTML or terminal control was not escaped")
		}
	}
	payload, err := report.SARIF(Limits{})
	if err != nil {
		t.Fatal(err)
	}
	var log map[string]any
	if err := json.Unmarshal(payload, &log); err != nil {
		t.Fatal(err)
	}
	run := log["runs"].([]any)[0].(map[string]any)
	if _, present := run["invocations"]; present {
		t.Fatal("message text injected an invocation")
	}
	result := run["results"].([]any)[0].(map[string]any)
	message := result["message"].(map[string]any)
	if len(message) != 1 || message["text"] != record.Message {
		t.Fatal("plain message was changed or promoted to markdown")
	}
}

func TestSARIFBoundsCompletePayloadAndKeepsEmptyResults(t *testing.T) {
	report, err := New(nil, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := report.SARIF(Limits{})
	if err != nil || !strings.Contains(string(payload), `"results":[]`) {
		t.Fatalf("empty findings were changed into an outcome: %v", err)
	}
	exact, err := report.SARIF(Limits{MaxBytes: len(payload)})
	if err != nil || !bytes.Equal(payload, exact) {
		t.Fatal("exact SARIF budget rejected")
	}
	if partial, err := report.SARIF(Limits{MaxBytes: len(payload) - 1}); err == nil || partial != nil {
		t.Fatal("SARIF byte overflow returned partial output")
	}
}
