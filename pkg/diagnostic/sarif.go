package diagnostic

import "net/url"

// SARIF projects existing findings into SARIF 2.1.0. It never invokes a check,
// reads source files, emits fixes, or asserts execution success or coverage.
// Results use kind=review and level=none; the original severity is retained in
// properties.repoctx because SARIF error/warning levels imply rule evaluation.
// Byte ranges are exact. Original line/byte-column coordinates remain in
// properties, since SARIF character columns cannot be inferred without source.
// See https://docs.oasis-open.org/sarif/sarif/v2.1.0/os/sarif-v2.1.0-os.html.
func (r *Report) SARIF(limits Limits) ([]byte, error) {
	report, normalized, err := normalizeReport(r, limits)
	if err != nil {
		return nil, err
	}
	results := make([]sarifResult, 0, len(report.Diagnostics))
	for _, record := range report.Diagnostics {
		result := sarifResult{
			RuleID: record.Code, Kind: "review", Level: "none",
			Message: sarifMessage{Text: record.Message},
		}
		result.Properties.Repoctx = sarifRecordProperties{
			Severity: record.Severity, Pass: record.Pass, Provider: record.Provider,
			Capability: record.Capability, Remediation: record.Remediation, Location: record.Location,
		}
		if record.Location != nil {
			location := record.Location
			path := url.URL{Path: location.Path}
			result.Locations = []sarifLocation{{PhysicalLocation: sarifPhysicalLocation{
				ArtifactLocation: sarifArtifactLocation{URI: path.EscapedPath()},
				Region:           sarifRegion{ByteOffset: location.StartByte, ByteLength: location.EndByte - location.StartByte},
			}}}
		}
		results = append(results, result)
	}
	run := sarifRun{Results: results}
	run.Tool.Driver.Name = "repoctx diagnostic projection"
	run.Properties.Repoctx.Contract = Version
	run.Properties.Repoctx.Execution = "not_asserted"
	run.Properties.Repoctx.Coverage = "not_asserted"
	run.Properties.Repoctx.Trust = "untrusted_diagnostic_data"
	return marshalBounded(sarifLog{
		Schema:  "https://docs.oasis-open.org/sarif/sarif/v2.1.0/errata01/os/schemas/sarif-schema-2.1.0.json",
		Version: "2.1.0", Runs: []sarifRun{run},
	}, normalized.MaxBytes)
}

type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool struct {
		Driver struct {
			Name string `json:"name"`
		} `json:"driver"`
	} `json:"tool"`
	Results    []sarifResult `json:"results"`
	Properties struct {
		Repoctx struct {
			Contract  string `json:"contract"`
			Execution string `json:"execution"`
			Coverage  string `json:"coverage"`
			Trust     string `json:"trust"`
		} `json:"repoctx"`
	} `json:"properties"`
}

type sarifResult struct {
	RuleID     Code            `json:"ruleId"`
	Kind       string          `json:"kind"`
	Level      string          `json:"level"`
	Message    sarifMessage    `json:"message"`
	Locations  []sarifLocation `json:"locations,omitempty"`
	Properties struct {
		Repoctx sarifRecordProperties `json:"repoctx"`
	} `json:"properties"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifRecordProperties struct {
	Severity    Severity    `json:"severity"`
	Pass        string      `json:"pass"`
	Provider    string      `json:"provider,omitempty"`
	Capability  string      `json:"capability"`
	Remediation Remediation `json:"remediation"`
	Location    *Location   `json:"location,omitempty"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           sarifRegion           `json:"region"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	ByteOffset int64 `json:"byteOffset"`
	ByteLength int64 `json:"byteLength"`
}
