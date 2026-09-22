package diagnostic

import (
	"bytes"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestNewCopiesAndOrdersRecordsDeterministically(t *testing.T) {
	first := recordFixture()
	first.Location = locationFixture()
	second := first
	second.Message = "another finding"
	third := first
	third.Location = nil
	records := []Record{first, second, third, first}
	report, err := New(records, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := report.JSON(Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Diagnostics) != 4 || report.Diagnostics[0].Location != nil {
		t.Fatalf("unexpected order or lost duplicate: %+v", report.Diagnostics)
	}
	slices.Reverse(records)
	reordered := &Report{Version: Version, Diagnostics: records}
	other, err := reordered.JSON(Limits{})
	if err != nil || !bytes.Equal(payload, other) {
		t.Fatalf("input order changed canonical output: %v", err)
	}
	if report.Diagnostics[1].Location == first.Location {
		t.Fatal("report retained caller-owned location")
	}
	first.Location.Path = "changed.go"
	records[0].Message = "changed"
	unchanged, err := report.JSON(Limits{})
	if err != nil || !bytes.Equal(payload, unchanged) {
		t.Fatal("caller mutation changed a constructed report")
	}
	var roundtrip Report
	if err := json.Unmarshal(payload, &roundtrip); err != nil {
		t.Fatal(err)
	}
	if err := roundtrip.Validate(Limits{}); err != nil {
		t.Fatal(err)
	}
}

func TestNewRejectsInvalidRecordsWithoutEchoingContent(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*Record)
	}{
		{name: "code", mutate: func(r *Record) { r.Code = "secret" }},
		{name: "severity", mutate: func(r *Record) { r.Severity = "secret" }},
		{name: "remediation", mutate: func(r *Record) { r.Remediation = "secret" }},
		{name: "empty message", mutate: func(r *Record) { r.Message = "" }},
		{name: "invalid utf8", mutate: func(r *Record) { r.Message = "secret\xff" }},
		{name: "long message", mutate: func(r *Record) { r.Message = strings.Repeat("secret", 1400) }},
		{name: "pass", mutate: func(r *Record) { r.Pass = "secret/execute" }},
		{name: "provider", mutate: func(r *Record) { r.Provider = "https://secret" }},
		{name: "capability", mutate: func(r *Record) { r.Capability = "secret\n" }},
		{name: "long pass", mutate: func(r *Record) { r.Pass = strings.Repeat("a", 65) }},
		{name: "long provider", mutate: func(r *Record) { r.Provider = strings.Repeat("a", 129) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			record := recordFixture()
			test.mutate(&record)
			report, err := New([]Record{record}, Limits{})
			if err == nil || report != nil {
				t.Fatal("invalid diagnostic accepted")
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatalf("validation echoed untrusted content: %v", err)
			}
		})
	}
}

func TestNewValidatesExactOptionalLocations(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*Location)
	}{
		{name: "parent path", mutate: func(l *Location) { l.Path = "../secret" }},
		{name: "absolute path", mutate: func(l *Location) { l.Path = "/secret" }},
		{name: "dot path", mutate: func(l *Location) { l.Path = "a/../secret" }},
		{name: "backslash", mutate: func(l *Location) { l.Path = `a\secret` }},
		{name: "url", mutate: func(l *Location) { l.Path = "https://secret" }},
		{name: "control", mutate: func(l *Location) { l.Path = "secret\n.go" }},
		{name: "root", mutate: func(l *Location) { l.Path = "." }},
		{name: "long path", mutate: func(l *Location) { l.Path = strings.Repeat("a", 4097) }},
		{name: "digest", mutate: func(l *Location) { l.SHA256 = "secret" }},
		{name: "negative", mutate: func(l *Location) { l.StartByte = -1 }},
		{name: "reversed bytes", mutate: func(l *Location) { l.EndByte = l.StartByte - 1 }},
		{name: "unsafe integer", mutate: func(l *Location) { l.EndByte = 1 << 53 }},
		{name: "zero line", mutate: func(l *Location) { l.StartLine = 0 }},
		{name: "reversed lines", mutate: func(l *Location) { l.EndLine = 1 }},
		{name: "columns", mutate: func(l *Location) { l.EndByteColumn++ }},
		{name: "zero width mismatch", mutate: func(l *Location) { l.EndByte = l.StartByte }},
		{name: "first line", mutate: func(l *Location) { l.StartLine, l.EndLine = 1, 1 }},
		{name: "multiline minimum", mutate: func(l *Location) { l.EndLine = 20 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			record := recordFixture()
			record.Location = locationFixture()
			test.mutate(record.Location)
			if _, err := New([]Record{record}, Limits{}); err == nil {
				t.Fatal("invalid exact location accepted")
			}
		})
	}
	record := recordFixture()
	record.Location = locationFixture()
	record.Location.EndByte = record.Location.StartByte
	record.Location.EndByteColumn = record.Location.StartByteColumn
	if _, err := New([]Record{record}, Limits{}); err != nil {
		t.Fatalf("valid insertion point rejected: %v", err)
	}
	record.Location = nil
	if _, err := New([]Record{record}, Limits{}); err != nil {
		t.Fatalf("locationless diagnostic rejected: %v", err)
	}
}

func TestReportEnforcesLimitsWithoutPartialOutput(t *testing.T) {
	record := recordFixture()
	report, err := New([]Record{record}, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := report.JSON(Limits{})
	if err != nil {
		t.Fatal(err)
	}
	exact := Limits{MaxBytes: len(payload)}
	if _, err := New([]Record{record}, exact); err != nil {
		t.Fatalf("exact byte boundary rejected: %v", err)
	}
	if result, err := report.JSON(Limits{MaxBytes: len(payload) - 1}); err == nil || result != nil {
		t.Fatal("overflow returned a partial report")
	}
	for _, limits := range []Limits{
		{MaxRecords: -1}, {MaxRecords: 4097}, {MaxMessageBytes: 8193},
		{MaxPathBytes: 4097}, {MaxBytes: 8<<20 + 1}, {MaxBytes: -1},
		{MaxMessageBytes: len(record.Message) - 1},
	} {
		if result, err := New([]Record{record}, limits); err == nil || result != nil {
			t.Fatalf("invalid or exceeded limits accepted: %+v", limits)
		}
	}
	if _, err := New([]Record{record, record}, Limits{MaxRecords: 1}); err == nil {
		t.Fatal("record count limit ignored")
	}
	record.Location = locationFixture()
	if _, err := New([]Record{record}, Limits{MaxPathBytes: len(record.Location.Path) - 1}); err == nil {
		t.Fatal("path byte limit ignored")
	}
}

func TestReportRejectsMalformedEnvelope(t *testing.T) {
	for _, report := range []*Report{
		nil, {}, {Version: Version}, {Version: "future", Diagnostics: []Record{}},
	} {
		if err := report.Validate(Limits{}); err == nil {
			t.Fatal("malformed report accepted")
		}
		if payload, err := report.SARIF(Limits{}); err == nil || payload != nil {
			t.Fatal("malformed report projected to sarif")
		}
	}
	report, err := New(nil, Limits{})
	if err != nil || !reflect.DeepEqual(report.Diagnostics, []Record{}) {
		t.Fatalf("empty report is not a non-null array: %v", err)
	}
}

func recordFixture() Record {
	return Record{
		Code: CodeProviderUnavailable, Severity: SeverityWarning,
		Message: "type resolution is unavailable", Pass: "link", Provider: "go-types",
		Capability: "type_resolution", Remediation: RemediationCapability,
	}
}

func locationFixture() *Location {
	return &Location{
		Path: "src/example.go", SHA256: strings.Repeat("a", 64),
		StartByte: 12, EndByte: 18, StartLine: 2, EndLine: 2,
		StartByteColumn: 0, EndByteColumn: 6,
	}
}
