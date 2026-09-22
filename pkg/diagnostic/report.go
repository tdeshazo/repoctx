package diagnostic

import (
	"cmp"
	"encoding/json"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	namePattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,127}$`)
	hashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// New validates, copies, and orders findings without modifying the input.
// Exact duplicates are retained. Nil input becomes an empty array.
func New(records []Record, limits Limits) (*Report, error) {
	if records == nil {
		records = []Record{}
	}
	report, normalized, err := normalizeReport(&Report{Version: Version, Diagnostics: records}, limits)
	if err != nil {
		return nil, err
	}
	if _, err := marshalBounded(report, normalized.MaxBytes); err != nil {
		return nil, err
	}
	return report, nil
}

// Validate checks the contract and bounded JSON representation. Input order is
// unrestricted; JSON and SARIF emit canonical order without changing the report.
func (r *Report) Validate(limits Limits) error {
	_, err := r.JSON(limits)
	return err
}

// JSON returns canonical newline-terminated JSON, or an error with no payload.
func (r *Report) JSON(limits Limits) ([]byte, error) {
	report, normalized, err := normalizeReport(r, limits)
	if err != nil {
		return nil, err
	}
	return marshalBounded(report, normalized.MaxBytes)
}

func normalizeReport(report *Report, limits Limits) (*Report, Limits, error) {
	if err := normalizeLimits(&limits); err != nil {
		return nil, limits, err
	}
	if report == nil || report.Version != Version || report.Diagnostics == nil {
		return nil, limits, fmt.Errorf("diagnostic: invalid report contract")
	}
	if len(report.Diagnostics) > limits.MaxRecords {
		return nil, limits, fmt.Errorf("diagnostic: record limit exceeded")
	}
	var contentBytes int
	for i, record := range report.Diagnostics {
		if err := validateRecord(record, limits); err != nil {
			// Never echo invalid or potentially denied repository content.
			return nil, limits, fmt.Errorf("diagnostic: record %d: %w", i, err)
		}
		contentBytes += len(record.Message)
		if record.Location != nil {
			contentBytes += len(record.Location.Path)
		}
		if contentBytes > limits.MaxBytes {
			return nil, limits, fmt.Errorf("diagnostic: output byte limit exceeded")
		}
	}
	owned := &Report{Version: Version, Diagnostics: append([]Record{}, report.Diagnostics...)}
	for i := range owned.Diagnostics {
		if owned.Diagnostics[i].Location != nil {
			location := *owned.Diagnostics[i].Location
			owned.Diagnostics[i].Location = &location
		}
	}
	slices.SortFunc(owned.Diagnostics, compareRecords)
	return owned, limits, nil
}

func normalizeLimits(limits *Limits) error {
	for _, setting := range []struct {
		value             *int
		fallback, ceiling int
	}{
		{value: &limits.MaxRecords, fallback: 1024, ceiling: 4096},
		{value: &limits.MaxMessageBytes, fallback: 4096, ceiling: 8192},
		{value: &limits.MaxPathBytes, fallback: 4096, ceiling: 4096},
		{value: &limits.MaxBytes, fallback: 1 << 20, ceiling: 8 << 20},
	} {
		if *setting.value == 0 {
			*setting.value = setting.fallback
		}
		if *setting.value < 1 || *setting.value > setting.ceiling {
			return fmt.Errorf("diagnostic: invalid resource limit")
		}
	}
	return nil
}

func validateRecord(record Record, limits Limits) error {
	switch record.Code {
	case CodeSyntaxError, CodeInvalidInput, CodeInputUnavailable, CodeUnresolvedReference,
		CodeStaleInput, CodePolicyDenied, CodeProviderUnavailable, CodeLimitExceeded, CodeInternalError:
	default:
		return fmt.Errorf("unknown code")
	}
	switch record.Severity {
	case SeverityInfo, SeverityWarning, SeverityError:
	default:
		return fmt.Errorf("invalid severity")
	}
	switch record.Remediation {
	case RemediationSource, RemediationConfiguration, RemediationRefresh, RemediationPolicy,
		RemediationCapability, RemediationScope, RemediationBug:
	default:
		return fmt.Errorf("invalid remediation category")
	}
	validMessage := len(record.Message) > 0 && len(record.Message) <= limits.MaxMessageBytes
	if !validMessage || !utf8.ValidString(record.Message) {
		return fmt.Errorf("invalid message")
	}
	for _, name := range []string{record.Pass, record.Capability} {
		if len(name) > 64 || !namePattern.MatchString(name) {
			return fmt.Errorf("invalid pass or capability")
		}
	}
	if record.Provider != "" && !namePattern.MatchString(record.Provider) {
		return fmt.Errorf("invalid provider")
	}
	if record.Location != nil {
		return validateLocation(*record.Location, limits.MaxPathBytes)
	}
	return nil
}

func validateLocation(location Location, maxPathBytes int) error {
	validPath := len(location.Path) <= maxPathBytes && utf8.ValidString(location.Path) && fs.ValidPath(location.Path)
	if !validPath || location.Path == "." || strings.ContainsAny(location.Path, `\:`) {
		return fmt.Errorf("invalid location path")
	}
	for _, character := range location.Path {
		if unicode.IsControl(character) {
			return fmt.Errorf("invalid location path")
		}
	}
	if !hashPattern.MatchString(location.SHA256) {
		return fmt.Errorf("invalid source digest")
	}
	// Keep integers exact for consumers using IEEE-754 JSON numbers.
	const maxCoordinate = 1<<53 - 1
	for _, coordinate := range []int64{
		location.StartByte, location.EndByte, location.StartLine,
		location.EndLine, location.StartByteColumn, location.EndByteColumn,
	} {
		if coordinate < 0 || coordinate > maxCoordinate {
			return fmt.Errorf("invalid location coordinate")
		}
	}
	validLines := location.StartLine >= 1 && location.EndLine >= location.StartLine
	if !validLines || location.EndByte < location.StartByte {
		return fmt.Errorf("reversed location range")
	}
	if location.StartByte < location.StartLine-1+location.StartByteColumn ||
		location.EndByte < location.EndLine-1+location.EndByteColumn {
		return fmt.Errorf("inconsistent location offsets")
	}
	if location.StartLine == 1 && location.StartByte != location.StartByteColumn ||
		location.EndLine == 1 && location.EndByte != location.EndByteColumn {
		return fmt.Errorf("inconsistent first-line location")
	}
	byteLength := location.EndByte - location.StartByte
	if location.StartLine == location.EndLine {
		if location.EndByteColumn-location.StartByteColumn != byteLength {
			return fmt.Errorf("inconsistent same-line location")
		}
	} else if byteLength < location.EndLine-location.StartLine+location.EndByteColumn {
		return fmt.Errorf("inconsistent multiline location")
	}
	return nil
}

func compareRecords(a, b Record) int {
	for _, pair := range [][2]string{
		{string(a.Code), string(b.Code)}, {a.Pass, b.Pass}, {a.Provider, b.Provider},
		{a.Capability, b.Capability},
	} {
		if order := strings.Compare(pair[0], pair[1]); order != 0 {
			return order
		}
	}
	if a.Location == nil && b.Location != nil {
		return -1
	}
	if a.Location != nil && b.Location == nil {
		return 1
	}
	if a.Location != nil {
		if order := compareLocations(*a.Location, *b.Location); order != 0 {
			return order
		}
	}
	for _, pair := range [][2]string{
		{string(a.Severity), string(b.Severity)},
		{string(a.Remediation), string(b.Remediation)}, {a.Message, b.Message},
	} {
		if order := strings.Compare(pair[0], pair[1]); order != 0 {
			return order
		}
	}
	return 0
}

func compareLocations(a, b Location) int {
	if order := strings.Compare(a.Path, b.Path); order != 0 {
		return order
	}
	for _, pair := range [][2]int64{
		{a.StartByte, b.StartByte}, {a.EndByte, b.EndByte},
		{a.StartLine, b.StartLine}, {a.StartByteColumn, b.StartByteColumn},
		{a.EndLine, b.EndLine}, {a.EndByteColumn, b.EndByteColumn},
	} {
		if order := cmp.Compare(pair[0], pair[1]); order != 0 {
			return order
		}
	}
	return strings.Compare(a.SHA256, b.SHA256)
}

func marshalBounded(value any, maxBytes int) ([]byte, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("diagnostic: encoding: %w", err)
	}
	if len(payload) >= maxBytes {
		return nil, fmt.Errorf("diagnostic: output byte limit exceeded")
	}
	return append(payload, '\n'), nil
}
